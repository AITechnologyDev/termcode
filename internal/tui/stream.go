package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/NekoFemDev/termcode/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

// sendMessage — пользователь нажал Enter, начинаем генерацию.
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}

	m.input.Reset()
	m.errMsg = ""
	m.currentState = stateThinking
	m.streaming = ""
	m.fullResponse = ""
	m.toolsExtracted = false
	m.displayedLen = 0
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.genSpeed = 0
	m.retryCount = 0
	m.pendingContent = ""

	m.sess.AddMessage(session.RoleUser, text)
	m.refreshViewport()
	m.scrollToBottom = true

	return m, tea.Batch(m.streamAI(), m.spinner.Tick)
}

// streamAI — запускает запрос к провайдеру и возвращает первый чанк.
func (m *Model) streamAI() tea.Cmd {
	m.cancelStream()

	pc, _ := m.cfg.ActiveProviderConfig()
	maxTokens := pc.GetMaxTokens()
	contextLength := pc.GetContextLength()

	var detectCmd tea.Cmd
	if m.cfg.ActiveProvider == config.ProviderOllama && pc.ContextLength == 0 &&
		!strings.Contains(pc.BaseURL, "ollama.com") {
		detectCmd = fetchContextLength(pc.BaseURL, pc.Model)
	}

	rawMsgs := make([]ai.Message, 0, len(m.sess.Messages))
	for _, msg := range m.sess.Messages {
		if msg.Role == session.RoleSystem {
			continue
		}
		role := string(msg.Role)
		if msg.Role == session.RoleTool {
			role = "user"
		}
		if role == "user" && len(rawMsgs) > 0 && rawMsgs[len(rawMsgs)-1].Role == "user" {
			rawMsgs[len(rawMsgs)-1].Content += "\n" + msg.Content
			continue
		}
		rawMsgs = append(rawMsgs, ai.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	lang := m.cfg.Language
	if lang == "" {
		lang = "en"
	}
	var langInstruction string
	if lang == "ru" {
		langInstruction = "\n\nIMPORTANT: Always respond in Russian language."
	} else {
		langInstruction = "\n\nIMPORTANT: Always respond in English language."
	}

	var extraContext string
	if m.cfg.UserProfile != "" {
		extraContext += "\n\n## About the user\n" + m.cfg.UserProfile
	}
	if m.cfg.AIInstructions != "" {
		extraContext += "\n\n## Response instructions\n" + m.cfg.AIInstructions
	}

	systemPrompt := m.cfg.SystemPrompt + "\n\n" + tools.ToolDefsWithExtras(m.pluginTools()) +
		"\n\nWorking directory: " + m.workDir + extraContext + langInstruction

	// Append system-prompt fragments contributed by in-process plugins.
	for _, frag := range m.pluginSystemPromptParts() {
		if frag != "" {
			systemPrompt += "\n\n" + frag
		}
	}

	apiMsgs, dropped := ai.TrimMessages(rawMsgs, systemPrompt, contextLength-maxTokens)
	if dropped > 0 {
		m.errMsg = fmt.Sprintf(m.tr().ContextDropped, dropped)
	}

	m.contextUsed = ai.SumTokens(apiMsgs) + ai.EstimateTokens(systemPrompt)
	m.contextLimit = contextLength

	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel

	provider := m.provider
	ctxLen := contextLength

	streamCmd := func() tea.Msg {
		ch, err := provider.Stream(apiMsgs, systemPrompt, maxTokens, ctxLen)
		if err != nil {
			cancel()
			return aiChunkMsg{err: err}
		}
		select {
		case <-ctx.Done():
			return aiChunkMsg{done: true}
		case chunk, ok := <-ch:
			if !ok {
				return aiChunkMsg{done: true}
			}
			if chunk.Err != nil {
				return aiChunkMsg{err: chunk.Err}
			}
			return streamReaderMsg{content: chunk.Content, done: chunk.Done, ch: ch, ctx: ctx}
		}
	}

	if detectCmd != nil {
		return tea.Batch(streamCmd, detectCmd)
	}
	return streamCmd
}

// updateStream — читает следующий чанк из канала стрима.
func (m Model) updateStream(msg streamReaderMsg) (tea.Model, tea.Cmd) {
	clean := stripANSI(msg.content)
	m.streaming += clean
	m.genTokens += countTokens(clean)
	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}
	m.refreshViewport()
	m.scrollToBottom = true

	if msg.done {
		return m.finalizeAIResponse()
	}

	ch := msg.ch
	ctx := msg.ctx
	return m, func() tea.Msg {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return aiChunkMsg{done: true}
			case chunk, ok := <-ch:
				if !ok {
					return aiChunkMsg{done: true}
				}
				if chunk.Err != nil {
					return aiChunkMsg{err: chunk.Err}
				}
				return streamReaderMsg{content: chunk.Content, done: chunk.Done, ch: ch, ctx: ctx}
			case <-time.After(120 * time.Second):
				return aiChunkMsg{err: fmt.Errorf("stream timeout: no response for 2 minutes")}
			}
		}
		select {
		case chunk, ok := <-ch:
			if !ok {
				return aiChunkMsg{done: true}
			}
			if chunk.Err != nil {
				return aiChunkMsg{err: chunk.Err}
			}
			return streamReaderMsg{content: chunk.Content, done: chunk.Done, ch: ch}
		case <-time.After(120 * time.Second):
			return aiChunkMsg{err: fmt.Errorf("stream timeout: no response for 2 minutes")}
		}
	}
}

// handleAIChunk — обрабатывает чанк / ошибку / done.
func (m Model) handleAIChunk(msg aiChunkMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.cancelStream()
		if isRetryableError(msg.err) && m.retryCount < m.maxRetries {
			m.retryCount++
			m.pendingContent += m.streaming
			m.streaming = ""
			m.currentState = stateRetrying
			m.errMsg = m.tr().RetryConnecting
			m.refreshViewport()

			return m, tea.Batch(
				tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return retryMsg{} }),
				m.spinner.Tick,
			)
		}

		m.currentState = stateChat
		m.errMsg = "AI error: " + msg.err.Error()
		m.streaming = ""
		m.fullResponse = ""
		m.retryCount = 0
		m.pendingContent = ""
		m.refreshViewport()
		return m, nil
	}

	if msg.done {
		return m.finalizeAIResponse()
	}

	m.streaming += msg.content
	m.genTokens += countTokens(msg.content)
	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}
	m.refreshViewport()
	m.scrollToBottom = true
	return m, nil
}

// finalizeAIResponse — стрим закончился, парсим tool calls и завершаем.
func (m Model) finalizeAIResponse() (tea.Model, tea.Cmd) {
	fullText := m.streaming
	m.streaming = ""
	m.retryCount = 0
	m.pendingContent = ""

	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}

	visibleText := filterThinkTags(fullText)
	calls, cleanText := ai.ParseToolCalls(visibleText)

	hasThink := extractThinkContent(fullText) != ""
	hasVisible := strings.TrimSpace(cleanText) != ""
	hasCalls := len(calls) > 0

	if hasVisible || hasThink || hasCalls {
		rawForSession := replaceThinkForSession(fullText, cleanText)
		m.sess.AddMessage(session.RoleAssistant, rawForSession)
	}

	if len(calls) > 0 {
		call := calls[0]
		executor := m.executor

		m.refreshViewport()
		m.scrollToBottom = true

		return m, func() tea.Msg {
			result := executor.Dispatch(call.Tool, call.Params)
			return toolDoneMsg{call: call, result: result}
		}
	}

	m.currentState = stateChat
	m.refreshViewport()
	m.scrollToBottom = true
	return m, nil
}

// handleToolDone — результат выполнения инструмента получен.
func (m Model) handleToolDone(msg toolDoneMsg) (tea.Model, tea.Cmd) {
	// Спецслучай: ask_user — показываем Q&A панель
	if msg.call.Tool == "ask_user" && msg.result.Output == "__ask_user__" {
		extra := msg.result.Extra
		question, _ := extra["question"].(string)
		optionsRaw, _ := extra["options"].([]string)
		multi, _ := extra["multi"].(bool)

		if question != "" {
			m.question = question
			m.questionOptions = optionsRaw
			m.questionCursor = 0
			m.questionSelected = make(map[int]bool)
			m.questionMulti = multi
			m.questionToolCall = true
			m.currentState = stateQuestion
			m.input.Reset()
			m = m.resize()
			m.refreshViewport()
			m.scrollToBottom = true
			return m, nil
		}
	}

	tc := session.ToolCall{
		Name:   msg.call.Tool,
		Params: msg.call.Params,
	}
	if msg.result.OK {
		tc.Result = msg.result.Output
	} else {
		tc.Error = msg.result.Error
	}
	m.sess.AddToolCall(tc)

	var toolResultContent string
	if msg.result.OK {
		toolResultContent = fmt.Sprintf("[tool:%s]\n%s", msg.call.Tool, msg.result.Output)
	} else {
		toolResultContent = fmt.Sprintf("[tool:%s ERROR]\n%s", msg.call.Tool, msg.result.Error)
	}
	m.sess.AddMessage(session.RoleTool, toolResultContent)

	m.streaming = ""
	m.fullResponse = ""
	m.displayedLen = 0
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.currentState = stateThinking
	m.refreshViewport()
	m.scrollToBottom = true

	return m, tea.Batch(m.streamAI(), m.spinner.Tick)
}

// isRetryableError — стоит ли повторять запрос при этой ошибке.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	for _, s := range []string{
		"connection refused", "connection reset", "broken pipe",
		"timeout", "no such host", "eof", "unexpected eof",
		"stream", "parse",
	} {
		if strings.Contains(errStr, s) {
			return true
		}
	}
	return false
}

// countTokens — приближённо: 1 слово ≈ 1.3 токена.
func countTokens(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3 + 0.5)
}
