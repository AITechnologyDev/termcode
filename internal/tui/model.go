package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AITechnologyDev/termcode/internal/ai"
	"github.com/AITechnologyDev/termcode/internal/config"
	"github.com/AITechnologyDev/termcode/internal/session"
	"github.com/AITechnologyDev/termcode/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Состояния TUI ─────────────────────────────────────────────────────────────

type state int

const (
	stateModelSelect state = iota
	stateChat
	stateThinking
	statePulling
	stateQuestion
	statePalette // Ctrl+P — палитра команд
)

// ── Сообщения BubbleTea ───────────────────────────────────────────────────────

// aiChunkMsg — кусок стримингового ответа AI
type aiChunkMsg struct {
	content string
	done    bool
	err     error
}

// toolDoneMsg — результат выполнения инструмента
type toolDoneMsg struct {
	call   ai.ToolCallRequest
	result tools.Result
}

// saveSessionMsg — сигнал сохранить сессию
type saveSessionMsg struct{}

// ollamaModelsMsg — список моделей от Ollama
type ollamaModelsMsg struct {
	models []string
	err    error
}

// pullProgressMsg — прогресс ollama pull
type pullProgressMsg struct {
	status    string
	completed int64
	total     int64
	done      bool
	err       error
}

// paletteItem — одна команда в палитре
type paletteItem struct {
	key         string // горячая клавиша или команда
	title       string // название
	description string // пояснение
	action      func(m Model) (Model, tea.Cmd)
}

// ── Модель TUI ────────────────────────────────────────────────────────────────

// Model — главная модель BubbleTea
type Model struct {
	// Конфиг и провайдер
	cfg      *config.Config
	provider ai.Provider

	// Сессия
	sess    *session.Session
	workDir string

	// Инструменты
	executor *tools.Executor

	// UI компоненты
	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	// Состояние
	currentState state
	streaming    string // буфер текущего стримингового ответа
	errMsg       string

	// Размеры терминала
	width  int
	height int

	// Признак что viewport нужно прокрутить вниз
	scrollToBottom bool

	// ── Выбор модели при старте ───────────────────────────────────────────
	ollamaModels   []string // список моделей от Ollama
	modelCursor    int      // текущий курсор в списке
	modelsLoading  bool     // идёт загрузка списка

	// ── Ollama pull ───────────────────────────────────────────────────────
	pullModelName  string // имя модели которую тянем
	pullStatus     string // статус из API
	pullCompleted  int64  // байт скачано
	pullTotal      int64  // байт всего

	// ── Статистика генерации ──────────────────────────────────────────────
	genStartTime   time.Time
	genTokens      int
	genSpeed       float64

	// ── Использование контекста ───────────────────────────────────────────
	contextUsed  int // токенов в текущем контексте
	contextLimit int // лимит контекста модели

	// ── Интерактивный вопрос от AI ────────────────────────────────────────
	question        string
	questionOptions []string
	questionCursor  int

	// ── Палитра команд (Ctrl+P) ───────────────────────────────────────────
	paletteCursor  int
	paletteFilter  string
	paletteItems   []paletteItem
}

// New создаёт новую TUI модель
func New(cfg *config.Config, workDir string) (*Model, error) {
	// Определяем рабочую директорию
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("os.Getwd: %w", err)
		}
	}

	// Создаём провайдера
	pc, ok := cfg.ActiveProviderConfig()
	if !ok {
		return nil, fmt.Errorf("конфиг провайдера %q не найден", cfg.ActiveProvider)
	}
	provider, err := ai.New(pc, cfg.ActiveProvider)
	if err != nil {
		return nil, fmt.Errorf("создание провайдера: %w", err)
	}

	// Создаём сессию
	sess := session.New(workDir, string(cfg.ActiveProvider), pc.Model)

	// Textarea для ввода
	ta := textarea.New()
	ta.Placeholder = "Введи запрос... (Enter — отправить, Shift+Enter — перенос строки)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")

	// Viewport для вывода чата
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	m := &Model{
		cfg:          cfg,
		provider:     provider,
		sess:         sess,
		workDir:      workDir,
		executor:     tools.New(workDir),
		viewport:     vp,
		input:        ta,
		spinner:      sp,
		currentState: stateModelSelect,
		modelsLoading: true,
	}
	m.paletteItems = buildPaletteItems()
	return m, nil
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		fetchOllamaModels(m.cfg),
	)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.resize()

	// ── Список моделей получен ────────────────────────────────────────────────
	case ollamaModelsMsg:
		m.modelsLoading = false
		if msg.err != nil {
			// Ollama недоступна — сразу идём в чат
			m.errMsg = "Ollama недоступна: " + msg.err.Error()
			m.currentState = stateChat
		} else if len(msg.models) == 0 {
			m.currentState = stateChat
		} else {
			m.ollamaModels = msg.models
			m.modelCursor = 0
			// Предвыбираем текущую модель если она есть в списке
			pc, _ := m.cfg.ActiveProviderConfig()
			for i, name := range msg.models {
				if name == pc.Model {
					m.modelCursor = i
					break
				}
			}
		}
		return m, nil

	// ── Прогресс ollama pull ──────────────────────────────────────────────────
	case pullProgressMsg:
		if msg.err != nil {
			m.currentState = stateChat
			m.errMsg = "pull ошибка: " + msg.err.Error()
			return m, nil
		}
		m.pullStatus = msg.status
		m.pullCompleted = msg.completed
		m.pullTotal = msg.total
		if msg.done {
			// Pull завершён — перезагружаем список моделей
			m.currentState = stateModelSelect
			m.modelsLoading = true
			m.pullModelName = ""
			return m, fetchOllamaModels(m.cfg)
		}
		return m, nil

	case tea.KeyMsg:
		// ── Клавиши в режиме выбора модели ───────────────────────────────────
		if m.currentState == stateModelSelect && !m.modelsLoading {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyUp:
				if m.modelCursor > 0 {
					m.modelCursor--
				}
				return m, nil
			case tea.KeyDown:
				if m.modelCursor < len(m.ollamaModels)-1 {
					m.modelCursor++
				}
				return m, nil
			case tea.KeyEnter:
				return m.selectModel(m.ollamaModels[m.modelCursor])
			case tea.KeyRunes:
				// 'p' — pull новой модели: ввод имени
				if string(msg.Runes) == "p" {
					m.currentState = stateChat
					m.input.SetValue("Введи имя модели для pull (например: qwen3:8b): ")
					return m, nil
				}
				// 'q' — пропустить выбор
				if string(msg.Runes) == "q" {
					m.currentState = stateChat
					return m, nil
				}
			}
			return m, nil
		}

		// ── Клавиши в режиме палитры (Ctrl+P) ────────────────────────────────
		if m.currentState == statePalette {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.currentState = stateChat
				m.paletteFilter = ""
				return m, nil
			case tea.KeyUp:
				if m.paletteCursor > 0 {
					m.paletteCursor--
				}
				return m, nil
			case tea.KeyDown:
				filtered := filterPaletteItems(m.paletteItems, m.paletteFilter)
				if m.paletteCursor < len(filtered)-1 {
					m.paletteCursor++
				}
				return m, nil
			case tea.KeyEnter:
				filtered := filterPaletteItems(m.paletteItems, m.paletteFilter)
				if m.paletteCursor < len(filtered) {
					return m.executePaletteItem(filtered[m.paletteCursor])
				}
				return m, nil
			case tea.KeyBackspace:
				if len(m.paletteFilter) > 0 {
					m.paletteFilter = m.paletteFilter[:len(m.paletteFilter)-1]
					m.paletteCursor = 0
				}
				return m, nil
			case tea.KeyRunes:
				m.paletteFilter += string(msg.Runes)
				m.paletteCursor = 0
				return m, nil
			}
			return m, nil
		}

		// ── Клавиши в режиме pull ─────────────────────────────────────────────
		if m.currentState == statePulling {
			if msg.Type == tea.KeyCtrlC {
				m.currentState = stateChat
				m.pullModelName = ""
				return m, nil
			}
			return m, nil
		}

		// ── Клавиши в режиме вопроса от AI ───────────────────────────────────
		if m.currentState == stateQuestion {
			switch msg.Type {
			case tea.KeyCtrlC:
				_ = m.sess.Save()
				return m, tea.Quit
			case tea.KeyUp:
				if m.questionCursor > 0 {
					m.questionCursor--
				}
				return m, nil
			case tea.KeyDown:
				if m.questionCursor < len(m.questionOptions) {
					m.questionCursor++
				}
				return m, nil
			case tea.KeyEnter:
				return m.submitQuestionAnswer()
			case tea.KeyEsc:
				// Esc — отменить вопрос, вернуться в чат
				m.currentState = stateChat
				m.question = ""
				m.questionOptions = nil
				return m, nil
			}
			// Текстовый ввод — обновляем textarea (свой ответ)
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)
			return m, inputCmd
		}

		// ── Клавиши в чате (только если stateChat) ───────────────────────────
		if m.currentState != stateChat {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			_ = m.sess.Save()
			return m, tea.Quit
		case tea.KeyCtrlS:
			if err := m.sess.Save(); err != nil {
				m.errMsg = "Ошибка сохранения: " + err.Error()
			}
			return m, nil
		case tea.KeyCtrlP:
			// Ctrl+P — открыть палитру команд
			m.currentState = statePalette
			m.paletteCursor = 0
			m.paletteFilter = ""
			return m, nil
		case tea.KeyEsc:
			m.errMsg = ""
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "/models" {
				m.input.Reset()
				m.currentState = stateModelSelect
				m.modelsLoading = true
				return m, fetchOllamaModels(m.cfg)
			}
			if strings.HasPrefix(text, "/pull ") {
				modelName := strings.TrimSpace(strings.TrimPrefix(text, "/pull "))
				m.input.Reset()
				return m.startPull(modelName)
			}
			return m.sendMessage()
		}

	case aiChunkMsg:
		return m.handleAIChunk(msg)

	case streamReaderMsg:
		return m.updateStream(msg)

	case toolDoneMsg:
		return m.handleToolDone(msg)

	case saveSessionMsg:
		_ = m.sess.Save()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Обновляем input только в режиме чата
	if m.currentState == stateChat {
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	// Обновляем viewport
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	if m.scrollToBottom {
		m.viewport.GotoBottom()
		m.scrollToBottom = false
	}

	return m, tea.Batch(cmds...)
}

// sendMessage отправляет сообщение пользователя в AI
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}

	m.input.Reset()
	m.errMsg = ""
	m.currentState = stateThinking
	m.streaming = ""
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.genSpeed = 0

	m.sess.AddMessage(session.RoleUser, text)
	m.refreshViewport()
	m.scrollToBottom = true

	cmd := m.streamAI()
	return m, tea.Batch(cmd, m.spinner.Tick)
}

// streamAI запускает запрос к AI провайдеру
func (m *Model) streamAI() tea.Cmd {
	pc, _ := m.cfg.ActiveProviderConfig()
	maxTokens := pc.GetMaxTokens()
	contextLength := pc.GetContextLength()

	// Строим сообщения для API
	rawMsgs := make([]ai.Message, 0, len(m.sess.Messages))
	for _, msg := range m.sess.Messages {
		if msg.Role == session.RoleSystem {
			continue
		}
		rawMsgs = append(rawMsgs, ai.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	systemPrompt := m.cfg.SystemPrompt + "\n\n" + tools.ToolDefs() +
		"\n\nWorking directory: " + m.workDir

	// Обрезаем историю если не влезает в контекст
	apiMsgs, dropped := ai.TrimMessages(rawMsgs, systemPrompt, contextLength-maxTokens)
	if dropped > 0 {
		m.errMsg = fmt.Sprintf("контекст: удалено %d старых сообщений", dropped)
	}

	// Обновляем счётчик использования контекста
	m.contextUsed = ai.SumTokens(apiMsgs) + ai.EstimateTokens(systemPrompt)
	m.contextLimit = contextLength

	provider := m.provider

	return func() tea.Msg {
		ch, err := provider.Stream(apiMsgs, systemPrompt, maxTokens)
		if err != nil {
			return aiChunkMsg{err: err}
		}
		chunk, ok := <-ch
		if !ok {
			return aiChunkMsg{done: true}
		}
		if chunk.Err != nil {
			return aiChunkMsg{err: chunk.Err}
		}
		return streamReaderMsg{content: chunk.Content, done: chunk.Done, ch: ch}
	}
}

// streamReaderMsg — внутреннее сообщение для продолжения чтения стрима
type streamReaderMsg struct {
	content string
	done    bool
	ch      <-chan ai.StreamChunk
}

// handleAIChunk обрабатывает кусок ответа AI
func (m Model) handleAIChunk(msg aiChunkMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.currentState = stateChat
		m.errMsg = "Ошибка AI: " + msg.err.Error()
		m.streaming = ""
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

// updateStream читает следующий чанк из канала
func (m Model) updateStream(msg streamReaderMsg) (tea.Model, tea.Cmd) {
	m.streaming += msg.content
	m.genTokens += countTokens(msg.content)
	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}
	m.refreshViewport()
	m.scrollToBottom = true

	if msg.done {
		return m.finalizeAIResponse()
	}

	ch := msg.ch
	return m, func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return aiChunkMsg{done: true}
		}
		if chunk.Err != nil {
			return aiChunkMsg{err: chunk.Err}
		}
		return streamReaderMsg{content: chunk.Content, done: chunk.Done, ch: ch}
	}
}

// countTokens приближённо считает токены по словам (1 слово ≈ 1.3 токена)
func countTokens(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3 + 0.5)
}

// filterThinkTags убирает <think>...</think> блоки из текста (для reasoning моделей)
func filterThinkTags(text string) string {
	result := text
	for {
		start := strings.Index(result, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(result, "</think>")
		if end == -1 {
			// Незакрытый тег — обрезаем от <think> до конца
			result = strings.TrimSpace(result[:start])
			break
		}
		result = result[:start] + result[end+len("</think>"):]
	}
	return strings.TrimSpace(result)
}

// finalizeAIResponse вызывается когда стрим завершён
func (m Model) finalizeAIResponse() (tea.Model, tea.Cmd) {
	fullText := m.streaming
	m.streaming = ""

	// Фильтруем <think>...</think> теги (Qwen3, GLM, DeepSeek reasoning)
	fullText = filterThinkTags(fullText)

	// Сохраняем финальную статистику
	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}

	// Парсим tool calls из ответа
	calls, cleanText := ai.ParseToolCalls(fullText)

	// Добавляем ответ ассистента в историю
	m.sess.AddMessage(session.RoleAssistant, cleanText)

	if len(calls) > 0 {
		// Есть вызовы инструментов — выполняем первый
		call := calls[0]
		executor := m.executor

		m.refreshViewport()
		m.scrollToBottom = true

		return m, func() tea.Msg {
			result := executor.Dispatch(call.Tool, call.Params)
			return toolDoneMsg{call: call, result: result}
		}
	}

	// Нет tool calls — проверяем не вопрос ли это с вариантами
	m.currentState = stateChat

	// Парсим ```question блок если есть
	if q, opts := parseQuestionBlock(cleanText); q != "" {
		m.question = q
		m.questionOptions = opts
		m.questionCursor = 0
		m.currentState = stateQuestion
		// Очищаем ввод для своего варианта
		m.input.Reset()
		m.input.Placeholder = "Свой вариант ответа..."
	}

	m.refreshViewport()
	m.scrollToBottom = true

	// Сохраняем сессию асинхронно
	return m, func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return saveSessionMsg{}
	}
}

// handleToolDone обрабатывает результат выполнения инструмента
func (m Model) handleToolDone(msg toolDoneMsg) (tea.Model, tea.Cmd) {
	// Записываем tool call + результат в сессию
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

	// Добавляем результат как tool-сообщение для AI
	var toolResultContent string
	if msg.result.OK {
		toolResultContent = fmt.Sprintf("[tool:%s]\n%s", msg.call.Tool, msg.result.Output)
	} else {
		toolResultContent = fmt.Sprintf("[tool:%s ERROR]\n%s", msg.call.Tool, msg.result.Error)
	}
	m.sess.AddMessage(session.RoleTool, toolResultContent)

	m.refreshViewport()
	m.scrollToBottom = true

	// Отправляем результат обратно в AI для продолжения
	return m, m.streamAI()
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Загрузка..."
	}

	// Экран выбора модели
	if m.currentState == stateModelSelect {
		return m.renderModelSelect()
	}

	// Экран загрузки модели (ollama pull)
	if m.currentState == statePulling {
		return m.renderPullScreen()
	}

	header := m.renderHeader()
	chatView := m.viewport.View()
	statusBar := m.renderStatusBar()

	// В режиме вопроса — показываем панель вопроса вместо обычного ввода
	var inputArea string
	if m.currentState == stateQuestion {
		inputArea = m.renderQuestionPanel()
	} else {
		inputArea = m.renderInput()
	}

	hints := m.renderHints()

	base := lipgloss.JoinVertical(lipgloss.Left,
		header,
		chatView,
		dividerStyle.Render(strings.Repeat("─", m.width)),
		inputArea,
		statusBar,
		hints,
	)

	// Палитра команд — оверлей поверх основного экрана
	if m.currentState == statePalette {
		return renderOverlay(base, m.renderPalette(), m.width, m.height)
	}

	return base
}

// renderModelSelect — экран выбора модели
func (m Model) renderModelSelect() string {
	var sb strings.Builder

	title := headerStyle.Render(" TermCode — Выбор модели ")
	sb.WriteString(title + "\n\n")

	if m.modelsLoading {
		sb.WriteString(fmt.Sprintf("  %s Загружаем список моделей Ollama...\n", m.spinner.View()))
		return sb.String()
	}

	if len(m.ollamaModels) == 0 {
		sb.WriteString(statusErrStyle.Render("  Ollama недоступна или нет моделей.") + "\n\n")
		sb.WriteString(keyHintStyle.Render("  Запусти: ollama serve\n"))
		sb.WriteString(keyHintStyle.Render("  Скачай модель: /pull qwen2.5-coder:7b\n\n"))
		sb.WriteString(keyStyle.Render("  q") + keyHintStyle.Render(" — продолжить без выбора\n"))
		return sb.String()
	}

	sb.WriteString(keyHintStyle.Render("  Выбери модель (↑↓ — навигация, Enter — выбрать, p — скачать новую, q — пропустить)\n\n"))

	for i, model := range m.ollamaModels {
		if i == m.modelCursor {
			sb.WriteString(userBubbleStyle.Render(fmt.Sprintf("  ▶ %s", model)) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", model))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(keyHintStyle.Render(fmt.Sprintf("  Модель %d/%d", m.modelCursor+1, len(m.ollamaModels))))
	return sb.String()
}

// renderPullScreen — экран прогресса ollama pull
func (m Model) renderPullScreen() string {
	var sb strings.Builder

	title := headerStyle.Render(" TermCode — Загрузка модели ")
	sb.WriteString(title + "\n\n")

	model := assistantLabelStyle.Render(m.pullModelName)
	sb.WriteString(fmt.Sprintf("  Скачиваем: %s\n\n", model))

	sb.WriteString(fmt.Sprintf("  Статус: %s\n\n", m.pullStatus))

	if m.pullTotal > 0 {
		pct := float64(m.pullCompleted) / float64(m.pullTotal)
		barWidth := m.width - 10
		if barWidth < 10 {
			barWidth = 10
		}
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		completedMB := float64(m.pullCompleted) / 1024 / 1024
		totalMB := float64(m.pullTotal) / 1024 / 1024

		sb.WriteString(fmt.Sprintf("  [%s] %.0f%%\n", bar, pct*100))
		sb.WriteString(fmt.Sprintf("  %.1f MB / %.1f MB\n", completedMB, totalMB))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Идёт загрузка...\n", m.spinner.View()))
	}

	sb.WriteString("\n")
	sb.WriteString(keyStyle.Render("  Ctrl+C") + keyHintStyle.Render(" — прервать"))
	return sb.String()
}

// renderHeader отрисовывает верхнюю панель
func (m Model) renderHeader() string {
	title := headerStyle.Render(" TermCode ")

	providerInfo := fmt.Sprintf(" %s / %s ", m.provider.Name(), m.provider.Model())
	info := headerInfoStyle.Render(providerInfo)

	workDirShort := m.workDir
	if home, err := os.UserHomeDir(); err == nil {
		workDirShort = strings.Replace(workDirShort, home, "~", 1)
	}
	dirInfo := headerInfoStyle.Render(" 📁 " + workDirShort + " ")

	// Заполняем пространство между элементами
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(info) - lipgloss.Width(dirInfo)
	if gap < 0 {
		gap = 0
	}
	spacer := strings.Repeat(" ", gap)

	return title + spacer + info + dirInfo
}

// renderInput отрисовывает область ввода
func (m Model) renderInput() string {
	var style lipgloss.Style
	if m.currentState == stateThinking {
		style = inputContainerStyle
	} else {
		style = inputContainerFocusStyle
	}

	prompt := inputPromptStyle.Render("❯ ")
	inputView := m.input.View()

	return style.Width(m.width - 2).Render(prompt + inputView)
}

// renderStatusBar отрисовывает статусную строку
func (m Model) renderStatusBar() string {
	var left, right string

	switch m.currentState {
	case stateThinking:
		speed := ""
		if m.genSpeed > 0 {
			speed = fmt.Sprintf(" %.1f tok/s · ~%d tok", m.genSpeed, m.genTokens)
		}
		left = statusBusyStyle.Render(m.spinner.View()+" Генерирую...") +
			keyHintStyle.Render(speed)
	case stateChat:
		if m.errMsg != "" {
			left = statusErrStyle.Render("✗ " + m.errMsg)
		} else {
			left = statusOKStyle.Render(fmt.Sprintf("✓ Готов — %d сообщений", len(m.sess.Messages)))
		}
		if m.genSpeed > 0 {
			right = keyHintStyle.Render(fmt.Sprintf("последний: %.1f tok/s · %d tok", m.genSpeed, m.genTokens))
		}
	}

	// Индикатор контекста справа
	if m.contextLimit > 0 {
		pct := 0
		if m.contextLimit > 0 {
			pct = m.contextUsed * 100 / m.contextLimit
		}
		ctxStyle := keyHintStyle
		if pct >= 80 {
			ctxStyle = statusErrStyle
		} else if pct >= 60 {
			ctxStyle = statusBusyStyle
		}
		ctxStr := ctxStyle.Render(fmt.Sprintf("ctx %d%% (%s/%s)",
			pct,
			formatTok(m.contextUsed),
			formatTok(m.contextLimit),
		))
		if right != "" {
			right = right + "  " + ctxStr
		} else {
			right = ctxStr
		}
	}

	if right != "" {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
		if gap < 1 {
			gap = 1
		}
		return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
	}
	return statusBarStyle.Width(m.width).Render(left)
}

// formatTok форматирует токены: 1200 → "1.2k", 500 → "500"
func formatTok(n int) string {
	if n >= 1000 {
		k := float64(n) / 1000.0
		return fmt.Sprintf("%.1fk", k)
	}
	return fmt.Sprintf("%d", n)
}

// renderHints отрисовывает подсказки клавиш
func (m Model) renderHints() string {
	hints := []string{
		keyStyle.Render("Enter") + keyHintStyle.Render(" отправить"),
		keyStyle.Render("Shift+Enter") + keyHintStyle.Render(" перенос"),
		keyStyle.Render("Ctrl+P") + keyHintStyle.Render(" команды"),
		keyStyle.Render("/models") + keyHintStyle.Render(" модели"),
		keyStyle.Render("Ctrl+S") + keyHintStyle.Render(" сохранить"),
		keyStyle.Render("Ctrl+C") + keyHintStyle.Render(" выйти"),
	}
	return keyHintStyle.Render(strings.Join(hints, "  "))
}

// ── Вспомогательные методы ────────────────────────────────────────────────────

// resize пересчитывает размеры компонентов
func (m Model) resize() Model {
	headerH := 1
	inputH := 5
	statusH := 1
	hintsH := 1
	dividerH := 1

	vpHeight := m.height - headerH - inputH - statusH - hintsH - dividerH
	if vpHeight < 3 {
		vpHeight = 3
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	m.input.SetWidth(m.width - 4) // -4 для padding + prompt
	m.refreshViewport()
	return m
}

// refreshViewport перерисовывает содержимое viewport
func (m *Model) refreshViewport() {
	content := m.renderMessages()
	m.viewport.SetContent(content)
}

// renderMessages отрисовывает историю сообщений
func (m Model) renderMessages() string {
	if len(m.sess.Messages) == 0 && m.streaming == "" {
		welcome := lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(2).
			Render("  Добро пожаловать в TermCode 🚀\n  Задай вопрос или попроси изменить файл проекта.")
		return welcome
	}

	var sb strings.Builder
	contentWidth := m.width - 4

	for _, msg := range m.sess.Messages {
		switch msg.Role {
		case session.RoleUser:
			sb.WriteString(userLabelStyle.Render("▶ Ты") + "\n")
			sb.WriteString(userBubbleStyle.Width(contentWidth).Render(msg.Content))
			sb.WriteString("\n\n")

		case session.RoleAssistant:
			sb.WriteString(assistantLabelStyle.Render("◆ TermCode") + "\n")
			rendered := renderMarkdown(msg.Content, contentWidth)
			sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(rendered))

			// Tool calls этого сообщения
			for _, tc := range msg.ToolCalls {
				sb.WriteString(renderToolCall(tc, contentWidth))
			}
			sb.WriteString("\n\n")

		case session.RoleTool:
			// Tool результаты уже показаны через ToolCalls в сообщении ассистента
			continue
		}
	}

	// Текущий стриминг
	if m.streaming != "" {
		sb.WriteString(assistantLabelStyle.Render("◆ TermCode") + "\n")
		rendered := renderMarkdown(m.streaming, contentWidth)
		sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(rendered))
		sb.WriteString(" ▋") // курсор
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderToolCall отрисовывает вызов инструмента
func renderToolCall(tc session.ToolCall, width int) string {
	var sb strings.Builder

	// Заголовок инструмента
	params := make([]string, 0, len(tc.Params))
	for k, v := range tc.Params {
		short := v
		if len(short) > 40 {
			short = short[:37] + "..."
		}
		params = append(params, k+"="+short)
	}
	header := fmt.Sprintf("⚡ %s(%s)", tc.Name, strings.Join(params, ", "))
	sb.WriteString(toolCallStyle.Width(width-2).Render(header))
	sb.WriteString("\n")

	// Результат
	if tc.Error != "" {
		errShort := tc.Error
		if len(errShort) > 200 {
			errShort = errShort[:197] + "..."
		}
		sb.WriteString(toolErrorStyle.Width(width-2).Render("✗ " + errShort))
	} else if tc.Result != "" {
		resultShort := tc.Result
		if len(resultShort) > 500 {
			resultShort = resultShort[:497] + "..."
		}
		sb.WriteString(toolResultStyle.Width(width-2).Render(resultShort))
	}
	sb.WriteString("\n")
	return sb.String()
}

// renderMarkdown минимальный рендер markdown (без внешних зависимостей)
func renderMarkdown(text string, width int) string {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	inCodeBlock := false
	var codeLines []string
	codeLang := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// Конец кода
				code := strings.Join(codeLines, "\n")
				label := ""
				if codeLang != "" {
					label = lipgloss.NewStyle().Foreground(colorMuted).Render(" " + codeLang + " ") + "\n"
				}
				sb.WriteString(codeBlockStyle.Width(width-4).Render(label+code))
				sb.WriteString("\n")
				inCodeBlock = false
				codeLines = nil
				codeLang = ""
			} else {
				// Начало кода
				inCodeBlock = true
				codeLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Заголовки
		if strings.HasPrefix(line, "### ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).Render(line[4:]))
			sb.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(line[3:]))
			sb.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(line[2:]))
			sb.WriteString("\n")
			continue
		}

		// Жирный **text**
		line = renderInlineBold(line)

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Незакрытый код-блок
	if inCodeBlock && len(codeLines) > 0 {
		sb.WriteString(codeBlockStyle.Width(width-4).Render(strings.Join(codeLines, "\n")))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderInlineBold заменяет **text** на жирный
func renderInlineBold(line string) string {
	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	result := line
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end = start + 2 + end
		bold := boldStyle.Render(result[start+2 : end])
		result = result[:start] + bold + result[end+2:]
	}
	return result
}



// ── Ollama: список моделей ────────────────────────────────────────────────────

// fetchOllamaModels загружает список установленных моделей из Ollama
func fetchOllamaModels(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		pc, ok := cfg.ActiveProviderConfig()
		if !ok {
			return ollamaModelsMsg{err: fmt.Errorf("конфиг провайдера не найден")}
		}

		// Ollama API: GET /api/tags
		url := strings.TrimRight(pc.BaseURL, "/") + "/api/tags"
		resp, err := httpGetJSON(url)
		if err != nil {
			return ollamaModelsMsg{err: err}
		}

		// Парсим {"models": [{"name": "..."}]}
		type ollamaModel struct {
			Name string `json:"name"`
		}
		type ollamaTagsResp struct {
			Models []ollamaModel `json:"models"`
		}

		var tagsResp ollamaTagsResp
		if err := jsonUnmarshal(resp, &tagsResp); err != nil {
			return ollamaModelsMsg{err: err}
		}

		names := make([]string, 0, len(tagsResp.Models))
		for _, m := range tagsResp.Models {
			names = append(names, m.Name)
		}
		return ollamaModelsMsg{models: names}
	}
}

// selectModel применяет выбранную модель и переходит в чат
func (m Model) selectModel(name string) (tea.Model, tea.Cmd) {
	// Обновляем конфиг
	pc := m.cfg.Providers[m.cfg.ActiveProvider]
	pc.Model = name
	m.cfg.Providers[m.cfg.ActiveProvider] = pc
	_ = m.cfg.Save()

	// Пересоздаём провайдера с новой моделью
	provider, err := ai.New(pc, m.cfg.ActiveProvider)
	if err != nil {
		m.errMsg = "Ошибка смены модели: " + err.Error()
		m.currentState = stateChat
		return m, nil
	}

	m.provider = provider
	m.sess.Model = name
	m.currentState = stateChat
	m.refreshViewport()
	return m, nil
}

// ── Ollama pull ───────────────────────────────────────────────────────────────

// startPull запускает ollama pull для указанной модели
func (m Model) startPull(modelName string) (tea.Model, tea.Cmd) {
	if modelName == "" {
		m.errMsg = "Укажи имя модели: /pull qwen2.5-coder:7b"
		return m, nil
	}

	m.currentState = statePulling
	m.pullModelName = modelName
	m.pullStatus = "Подключение..."
	m.pullCompleted = 0
	m.pullTotal = 0

	pc, _ := m.cfg.ActiveProviderConfig()
	baseURL := strings.TrimRight(pc.BaseURL, "/")

	return m, tea.Batch(m.spinner.Tick, streamOllamaPull(baseURL, modelName))
}

// streamOllamaPull стримит прогресс ollama pull через /api/pull
func streamOllamaPull(baseURL, modelName string) tea.Cmd {
	return func() tea.Msg {
		url := baseURL + "/api/pull"

		body := fmt.Sprintf(`{"name":%q,"stream":true}`, modelName)
		resp, err := httpPostStream(url, body)
		if err != nil {
			return pullProgressMsg{err: err}
		}
		defer resp.Close()

		scanner := newLineScanner(resp)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			status, completed, total, done, parseErr := parsePullLine(line)
			if parseErr != nil {
				continue
			}

			if done {
				return pullProgressMsg{status: "Готово!", done: true}
			}

			return pullProgressMsg{
				status:    status,
				completed: completed,
				total:     total,
				done:      false,
			}
		}

		return pullProgressMsg{done: true}
	}
}

// ── HTTP хелперы (минимальные, без лишних зависимостей) ──────────────────────

func httpGetJSON(url string) ([]byte, error) {
	resp, err := httpDo("GET", url, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func httpPostStream(url, body string) (io.ReadCloser, error) {
	resp, err := httpDo("POST", url, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func httpDo(method, url, body string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 0} // без таймаута для pull
	return client.Do(req)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 64*1024)
	return s
}

// parsePullLine парсит одну строку JSON из ollama pull stream
func parsePullLine(line string) (status string, completed, total int64, done bool, err error) {
	var obj struct {
		Status    string `json:"status"`
		Completed int64  `json:"completed"`
		Total     int64  `json:"total"`
	}
	if err = json.Unmarshal([]byte(line), &obj); err != nil {
		return
	}
	status = obj.Status
	completed = obj.Completed
	total = obj.Total
	done = obj.Status == "success"
	return
}

// ── Интерактивные вопросы от AI ───────────────────────────────────────────────

// parseQuestionBlock ищет ```question блок в тексте ответа AI.
// Формат:
//
//	```question
//	Текст вопроса?
//	- Вариант A
//	- Вариант B
//	- Вариант C
//	```
//
// Возвращает текст вопроса и срез вариантов (может быть пустым).
func parseQuestionBlock(text string) (question string, options []string) {
	const openTag = "```question"
	const closeTag = "```"

	start := strings.Index(text, openTag)
	if start == -1 {
		return "", nil
	}
	inner := text[start+len(openTag):]
	end := strings.Index(inner, closeTag)
	if end == -1 {
		inner = strings.TrimSpace(inner)
	} else {
		inner = strings.TrimSpace(inner[:end])
	}

	lines := strings.Split(inner, "\n")
	if len(lines) == 0 {
		return "", nil
	}

	// Первая строка — текст вопроса
	question = strings.TrimSpace(lines[0])

	// Остальные строки начинающиеся с "- " — варианты
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			opt := strings.TrimPrefix(line, "- ")
			if opt != "" {
				options = append(options, opt)
			}
		}
	}
	return question, options
}

// submitQuestionAnswer отправляет выбранный ответ на вопрос AI
func (m Model) submitQuestionAnswer() (tea.Model, tea.Cmd) {
	var answer string

	customText := strings.TrimSpace(m.input.Value())

	if customText != "" {
		// Пользователь написал свой вариант
		answer = customText
	} else if len(m.questionOptions) > 0 && m.questionCursor < len(m.questionOptions) {
		// Выбран один из предложенных вариантов
		answer = m.questionOptions[m.questionCursor]
	} else {
		// Ничего не выбрано и не написано
		return m, nil
	}

	// Сбрасываем состояние вопроса
	m.question = ""
	m.questionOptions = nil
	m.questionCursor = 0
	m.input.Reset()
	m.input.Placeholder = "Введи запрос... (Enter — отправить, Shift+Enter — перенос строки)"
	m.currentState = stateThinking
	m.streaming = ""
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.genSpeed = 0

	// Добавляем ответ как сообщение пользователя
	m.sess.AddMessage(session.RoleUser, answer)
	m.refreshViewport()
	m.scrollToBottom = true

	return m, tea.Batch(m.streamAI(), m.spinner.Tick)
}

// renderQuestionPanel отрисовывает панель выбора ответа на вопрос AI
func (m Model) renderQuestionPanel() string {
	var sb strings.Builder
	w := m.width - 2

	// Заголовок вопроса
	questionHeader := toolCallStyle.Width(w).Render("❓ " + m.question)
	sb.WriteString(questionHeader)
	sb.WriteString("\n")

	// Варианты ответа
	if len(m.questionOptions) > 0 {
		for i, opt := range m.questionOptions {
			var line string
			if i == m.questionCursor {
				line = userBubbleStyle.Render(fmt.Sprintf("  ▶ %d. %s", i+1, opt))
			} else {
				line = keyHintStyle.Render(fmt.Sprintf("    %d. %s", i+1, opt))
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Строка своего ввода (последний "вариант" = курсор на конце списка)
	var inputStyle lipgloss.Style
	if m.questionCursor == len(m.questionOptions) || len(m.questionOptions) == 0 {
		inputStyle = inputContainerFocusStyle
	} else {
		inputStyle = inputContainerStyle
	}
	prompt := inputPromptStyle.Render("✏ ")
	sb.WriteString(inputStyle.Width(w).Render(prompt + m.input.View()))

	return sb.String()
}

// ── Палитра команд (Ctrl+P) ───────────────────────────────────────────────────

// buildPaletteItems возвращает полный список команд палитры
func buildPaletteItems() []paletteItem {
	return []paletteItem{
		{
			key: "Ctrl+P", title: "Палитра команд",
			description: "Открыть эту палитру",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = statePalette
				m.paletteCursor = 0
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "/models", title: "Сменить модель",
			description: "Показать список моделей Ollama",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateModelSelect
				m.modelsLoading = true
				m.paletteFilter = ""
				return m, fetchOllamaModels(m.cfg)
			},
		},
		{
			key: "/pull", title: "Скачать модель (ollama pull)",
			description: "Ввести имя модели для загрузки",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.input.SetValue("/pull ")
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "new", title: "Новая сессия",
			description: "Начать новый диалог (текущий сохранится)",
			action: func(m Model) (Model, tea.Cmd) {
				_ = m.sess.Save()
				pc, _ := m.cfg.ActiveProviderConfig()
				m.sess = session.New(m.workDir, string(m.cfg.ActiveProvider), pc.Model)
				m.streaming = ""
				m.errMsg = ""
				m.currentState = stateChat
				m.paletteFilter = ""
				m.refreshViewport()
				return m, nil
			},
		},
		{
			key: "Ctrl+S", title: "Сохранить сессию",
			description: "Сохранить историю диалога на диск",
			action: func(m Model) (Model, tea.Cmd) {
				if err := m.sess.Save(); err != nil {
					m.errMsg = "Ошибка сохранения: " + err.Error()
				}
				m.currentState = stateChat
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "ls", title: "Список файлов проекта",
			description: "Показать дерево файлов в рабочей директории",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				result := m.executor.ListFiles("")
				content := result.Output
				if !result.OK {
					content = "Ошибка: " + result.Error
				}
				m.sess.AddMessage(session.RoleAssistant,
					"```\n"+content+"\n```")
				m.refreshViewport()
				m.scrollToBottom = true
				return m, nil
			},
		},
		{
			key: "git", title: "Git статус",
			description: "Показать git status проекта",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				result := m.executor.RunCommand("git status --short 2>&1 || echo '(не git репозиторий)'")
				m.sess.AddMessage(session.RoleAssistant, "```\n"+result.Output+"\n```")
				m.refreshViewport()
				m.scrollToBottom = true
				return m, nil
			},
		},
		{
			key: "build", title: "Go build",
			description: "Запустить go build ./...",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateThinking
				m.paletteFilter = ""
				m.sess.AddMessage(session.RoleUser, "Run: go build ./...")
				m.genStartTime = time.Now()
				m.genTokens = 0
				m.refreshViewport()
				return m, tea.Batch(m.streamAI(), m.spinner.Tick)
			},
		},
		{
			key: "test", title: "Go test",
			description: "Запустить go test ./...",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateThinking
				m.paletteFilter = ""
				m.sess.AddMessage(session.RoleUser, "Run: go test ./... and show results")
				m.genStartTime = time.Now()
				m.genTokens = 0
				m.refreshViewport()
				return m, tea.Batch(m.streamAI(), m.spinner.Tick)
			},
		},
		{
			key: "ctx", title: "Показать использование контекста",
			description: "Сколько токенов занимает текущая история",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				pct := 0
				if m.contextLimit > 0 {
					pct = m.contextUsed * 100 / m.contextLimit
				}
				info := fmt.Sprintf(
					"**Контекст:** %s / %s токенов (%d%%)\n**Сообщений:** %d\n**Модель:** %s",
					formatTok(m.contextUsed), formatTok(m.contextLimit), pct,
					len(m.sess.Messages), m.provider.Model(),
				)
				m.sess.AddMessage(session.RoleAssistant, info)
				m.refreshViewport()
				m.scrollToBottom = true
				return m, nil
			},
		},
		{
			key: "clear", title: "Очистить экран",
			description: "Очистить viewport (история сохраняется)",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				m.viewport.SetContent("")
				return m, nil
			},
		},
	}
}

// filterPaletteItems фильтрует команды по строке поиска
func filterPaletteItems(items []paletteItem, filter string) []paletteItem {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var result []paletteItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.title), f) ||
			strings.Contains(strings.ToLower(item.key), f) ||
			strings.Contains(strings.ToLower(item.description), f) {
			result = append(result, item)
		}
	}
	return result
}

// executePaletteItem выполняет выбранную команду
func (m Model) executePaletteItem(item paletteItem) (tea.Model, tea.Cmd) {
	newM, cmd := item.action(m)
	return newM, cmd
}

// renderPalette отрисовывает палитру команд
func (m Model) renderPalette() string {
	w := m.width - 8
	if w < 30 {
		w = 30
	}

	var sb strings.Builder

	// Заголовок
	title := headerStyle.Render(" ⌘ Палитра команд ")
	sb.WriteString(title + "\n")

	// Строка поиска
	searchPrompt := inputPromptStyle.Render("🔍 ")
	searchVal := m.paletteFilter
	if searchVal == "" {
		searchVal = keyHintStyle.Render("Введи для поиска...")
	}
	sb.WriteString(inputContainerFocusStyle.Width(w).Render(searchPrompt+searchVal) + "\n\n")

	// Список команд
	filtered := filterPaletteItems(m.paletteItems, m.paletteFilter)
	if len(filtered) == 0 {
		sb.WriteString(keyHintStyle.Render("  Ничего не найдено") + "\n")
	}
	for i, item := range filtered {
		keyPart := keyStyle.Render(fmt.Sprintf("%-10s", item.key))
		titlePart := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(item.title)
		descPart := keyHintStyle.Render("  " + item.description)
		line := fmt.Sprintf("  %s  %s%s", keyPart, titlePart, descPart)
		if i == m.paletteCursor {
			line = userBubbleStyle.Width(w).Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(keyHintStyle.Render("  ↑↓ навигация  Enter выбрать  Esc закрыть"))
	return sb.String()
}

// renderOverlay накладывает overlay поверх base по центру
func renderOverlay(base, overlay string, width, height int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if lw := lipgloss.Width(l); lw > overlayW {
			overlayW = lw
		}
	}

	// Центрируем
	startY := (height - overlayH) / 3 // чуть выше центра
	startX := (width - overlayW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	// Рисуем overlay поверх base построчно
	baseLines := strings.Split(base, "\n")
	for i, ol := range overlayLines {
		y := startY + i
		if y >= len(baseLines) {
			baseLines = append(baseLines, "")
		}
		bl := baseLines[y]
		// Вставляем overlay в строку
		blRunes := []rune(bl)
		olRunes := []rune(ol)
		// Дополняем пробелами если нужно
		for len(blRunes) < startX+len(olRunes) {
			blRunes = append(blRunes, ' ')
		}
		copy(blRunes[startX:], olRunes)
		baseLines[y] = string(blRunes)
	}
	return strings.Join(baseLines, "\n")
}
