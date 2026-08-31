package tui

import (
	"strings"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Update — главный диспетчер сообщений BubbleTea.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if ws, err := getTermSize(); err == nil {
			m.screenWidthPx = int(ws.Xpixel)
			m.screenHeightPx = int(ws.Ypixel)
		}
		m = m.resize()
		return m, nil

	case ollamaModelsMsg:
		m.modelsLoading = false
		if msg.err != nil {
			m.errMsg = "Ollama unavailable: " + msg.err.Error()
			m.currentState = stateChat
		} else if len(msg.models) == 0 {
			m.currentState = stateChat
		} else {
			m.ollamaModels = msg.models
			m.modelCursor = 0
			pc, _ := m.cfg.ActiveProviderConfig()
			for i, name := range msg.models {
				if name == pc.Model {
					m.modelCursor = i
					break
				}
			}
		}
		return m, nil

	case pullProgressMsg:
		if msg.err != nil {
			m.currentState = stateChat
			m.errMsg = "pull error: " + msg.err.Error()
			return m, nil
		}
		m.pullStatus = msg.status
		m.pullCompleted = msg.completed
		m.pullTotal = msg.total
		if msg.done {
			m.currentState = stateModelSelect
			m.modelsLoading = true
			m.pullModelName = ""
			return m, fetchOllamaModels(m.cfg)
		}
		return m, nil

	case sessionsLoadedMsg:
		m.sessionsLoading = false
		m.savedSessions = msg.sessions
		m.sessionCursor = 0
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case aiChunkMsg:
		return m.handleAIChunk(msg)

	case streamReaderMsg:
		return m.updateStream(msg)

	case retryMsg:
		m.streaming = m.pendingContent
		m.refreshViewport()
		return m, tea.Batch(m.streamAI(), m.spinner.Tick)

	case toolDoneMsg:
		return m.handleToolDone(msg)

	case saveSessionMsg:
		_ = m.sess.Save()
		return m, nil

	case contextDetectedMsg:
		if msg.err == nil && msg.contextLength > 0 {
			pc, _ := m.cfg.ActiveProviderConfig()
			changed := false
			if pc.ContextLength != msg.contextLength {
				pc.ContextLength = msg.contextLength
				changed = true
			}
			if msg.maxOutputTokens > 0 && pc.MaxTokens != msg.maxOutputTokens {
				pc.MaxTokens = msg.maxOutputTokens
				changed = true
			}
			if changed {
				providers := m.cfg.Providers
				providers[m.cfg.ActiveProvider] = pc
				m.cfg.Providers = providers
				_ = m.cfg.Save()
			}
			m.contextLimit = msg.contextLength
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.currentState == stateChat || m.currentState == stateQuestion {
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	if m.scrollToBottom {
		m.viewport.GotoBottom()
		m.scrollToBottom = false
	}

	return m, tea.Batch(cmds...)
}

// handleKey — маршрутизация нажатий по режимам.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentState {
	case stateModelSelect:
		return m.handleKeyModelSelect(msg)
	case statePalette:
		return m.handleKeyPalette(msg)
	case statePulling:
		return m.handleKeyPulling(msg)
	case stateSessionLoad:
		return m.handleKeySessionLoad(msg)
	case stateProviderSelect:
		return m.handleKeyProviderSelect(msg)
	case stateProfileEdit:
		return m.handleKeyEdit(msg, profileEditSave)
	case stateInstructEdit:
		return m.handleKeyEdit(msg, instructEditSave)
	case stateQuestion:
		return m.handleKeyQuestion(msg)
	case stateChat, stateThinking, stateRetrying:
		return m.handleKeyChat(msg)
	}
	return m, nil
}

// ── Chat ───────────────────────────────────────────────────────────────

func (m Model) handleKeyChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.cancelStream()
		_ = m.sess.Save()
		return m, tea.Quit
	case tea.KeyCtrlS:
		if err := m.sess.Save(); err != nil {
			m.errMsg = "Save error: " + err.Error()
		}
		return m, nil
	case tea.KeyCtrlP:
		m.paletteItems = m.buildPaletteItems()
		m.currentState = statePalette
		m.paletteCursor = 0
		m.paletteFilter = ""
		return m, nil
	case tea.KeyEsc:
		m.errMsg = ""
	case tea.KeyRunes:
		if string(msg.Runes) == "T" {
			out := m.toggleLastThink()
			return out, nil
		}
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
		// Plugin slash commands
		if strings.HasPrefix(text, "/") {
			if newM, handled, errMsg := m.runPluginSlash(text); handled {
				m.input.Reset()
				m.currentState = stateChat
				if errMsg != "" {
					m.errMsg = errMsg
				}
				return newM, nil
			}
		}
		return m.sendMessage()
	}
	return m, nil
}

// ── Model select ───────────────────────────────────────────────────────

func (m Model) handleKeyModelSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modelsLoading {
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m, nil
	}
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
		switch string(msg.Runes) {
		case "p":
			m.currentState = stateChat
			m.input.SetValue("/pull ")
			return m, nil
		case "q":
			m.currentState = stateChat
			return m, nil
		}
	}
	return m, nil
}

// ── Palette ────────────────────────────────────────────────────────────

func (m Model) handleKeyPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// ── Pull ───────────────────────────────────────────────────────────────

func (m Model) handleKeyPulling(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.currentState = stateChat
		m.pullModelName = ""
	}
	return m, nil
}

// ── Session load ───────────────────────────────────────────────────────

func (m Model) handleKeySessionLoad(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.currentState = stateChat
		return m, nil
	case tea.KeyUp:
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.sessionCursor < len(m.savedSessions)-1 {
			m.sessionCursor++
		}
		return m, nil
	case tea.KeyEnter:
		if m.sessionCursor < len(m.savedSessions) {
			return m.loadSession(m.savedSessions[m.sessionCursor])
		}
		return m, nil
	case tea.KeyDelete, tea.KeyBackspace:
		if m.sessionCursor < len(m.savedSessions) {
			return m.deleteSession(m.savedSessions[m.sessionCursor])
		}
		return m, nil
	}
	return m, nil
}

// ── Provider select ────────────────────────────────────────────────────

func (m Model) handleKeyProviderSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	metas := config.ProvidersMeta()

	// Edit mode: всё идёт в textarea
	if m.providerEditMode != 0 {
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.providerEditMode = 0
			return m, nil
		case tea.KeyCtrlS:
			value := strings.TrimSpace(m.editInput.Value())
			cur := metas[m.providerCursor]
			pc := m.cfg.Providers[cur.ID]
			switch m.providerEditMode {
			case 1:
				pc.APIKey = value
			case 2:
				pc.BaseURL = value
			case 3:
				pc.Model = value
			}
			m.cfg.Providers[cur.ID] = pc
			_ = m.cfg.Save()
			if cur.ID == m.cfg.ActiveProvider {
				if newProvider, err := ai.New(pc, cur.ID); err == nil {
					m.provider = newProvider
				}
			}
			m.providerEditMode = 0
			return m, nil
		}
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.currentState = stateChat
		return m, nil
	case tea.KeyUp:
		if m.providerCursor > 0 {
			m.providerCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.providerCursor < len(metas)-1 {
			m.providerCursor++
		}
		return m, nil
	case tea.KeyEnter:
		if m.providerCursor < len(metas) {
			chosen := metas[m.providerCursor]
			newM, cmd := m.switchProvider(chosen.ID)
			newM.currentState = stateChat
			return newM, cmd
		}
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			m.providerEditMode = 1
			cur := metas[m.providerCursor]
			m.editInput = newEditTextarea(m.cfg.Providers[cur.ID].APIKey, m.width)
			return m, textarea.Blink
		case "u":
			m.providerEditMode = 2
			cur := metas[m.providerCursor]
			m.editInput = newEditTextarea(m.cfg.Providers[cur.ID].BaseURL, m.width)
			return m, textarea.Blink
		case "m":
			m.providerEditMode = 3
			cur := metas[m.providerCursor]
			m.editInput = newEditTextarea(m.cfg.Providers[cur.ID].Model, m.width)
			return m, textarea.Blink
		}
	}
	return m, nil
}

// ── Edit screens (profile / instructions) ──────────────────────────────

type editSaveAction int

const (
	profileEditSave editSaveAction = iota
	instructEditSave
)

func (m Model) handleKeyEdit(msg tea.KeyMsg, which editSaveAction) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.currentState = stateChat
		return m, nil
	case tea.KeyCtrlS:
		val := strings.TrimSpace(m.editInput.Value())
		switch which {
		case profileEditSave:
			m.cfg.UserProfile = val
		case instructEditSave:
			m.cfg.AIInstructions = val
		}
		_ = m.cfg.Save()
		m.currentState = stateChat
		return m, nil
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

// ── Question (Q&A от AI) ───────────────────────────────────────────────

func (m Model) handleKeyQuestion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		maxCursor := len(m.questionOptions)
		if m.questionCursor < maxCursor {
			m.questionCursor++
		}
		return m, nil
	case tea.KeySpace:
		if m.questionCursor < len(m.questionOptions) {
			m.questionSelected[m.questionCursor] = !m.questionSelected[m.questionCursor]
			return m, nil
		}
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		return m, inputCmd
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == ' ' && m.questionCursor < len(m.questionOptions) {
			m.questionSelected[m.questionCursor] = !m.questionSelected[m.questionCursor]
			return m, nil
		}
		if m.questionCursor == len(m.questionOptions) {
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)
			return m, inputCmd
		}
		return m, nil
	case tea.KeyEnter:
		return m.submitQuestionAnswer()
	case tea.KeyEsc:
		m.currentState = stateChat
		m.question = ""
		m.questionOptions = nil
		m.questionSelected = make(map[int]bool)
		m = m.resize()
		return m, nil
	default:
		if m.questionCursor == len(m.questionOptions) {
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)
			return m, inputCmd
		}
	}
	return m, nil
}
