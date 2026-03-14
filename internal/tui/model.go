package tui

import (
	"bufio"
	"context"
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

// ── i18n ──────────────────────────────────────────────────────────────────────

type i18nStrings struct {
	Placeholder        string
	StatusReady        string
	StatusGenerating   string
	StatusLastTok      string
	HintSend           string
	HintNewline        string
	HintCommands       string
	HintModels         string
	HintSave           string
	HintLang           string
	Thinking           string
	LoadingModels      string
	ModelSelectTitle   string
	ModelSelectHint    string
	ModelSelectCount   string
	PullTitle          string
	PullInterrupt      string
	PullDone           string
	QAHint             string
	QASelected         string
	ContextDropped     string
}

var i18nEN = i18nStrings{
	Placeholder:      "Ask anything... (Enter to send, Shift+Enter for newline)",
	StatusReady:      "✓ Ready — %d messages",
	StatusGenerating: " Generating...",
	StatusLastTok:    "last: %.1f tok/s · %d tok",
	HintSend:         " send",
	HintNewline:      " newline",
	HintCommands:     " commands",
	HintModels:       " models",
	HintSave:         " save",
	HintLang:         " Ctrl+P→lang",
	Thinking:         "🧠 Thinking...",
	LoadingModels:    "Loading...",
	ModelSelectTitle: " TermCode — Select Model ",
	ModelSelectHint:  "  ↑↓ navigate  Enter select  p pull new  q skip\n\n",
	ModelSelectCount: "  Model %d/%d",
	PullTitle:        " TermCode — Downloading Model ",
	PullInterrupt:    " — cancel",
	PullDone:         "Done!",
	QAHint:           "  Space — select  ↑↓ — navigate  Enter — confirm  Esc — cancel",
	QASelected:       "  ✓ Selected: %d",
	ContextDropped:   "context: dropped %d old messages",
}

var i18nRU = i18nStrings{
	Placeholder:      "Введи запрос... (Enter — отправить, Shift+Enter — перенос строки)",
	StatusReady:      "✓ Готов — %d сообщений",
	StatusGenerating: " Генерирую...",
	StatusLastTok:    "последний: %.1f tok/s · %d tok",
	HintSend:         " отправить",
	HintNewline:      " перенос",
	HintCommands:     " команды",
	HintModels:       " модели",
	HintSave:         " сохранить",
	HintLang:         " Ctrl+P→язык",
	Thinking:         "🧠 Думает...",
	LoadingModels:    "Загрузка...",
	ModelSelectTitle: " TermCode — Выбор модели ",
	ModelSelectHint:  "  ↑↓ навигация  Enter выбрать  p скачать  q пропустить\n\n",
	ModelSelectCount: "  Модель %d/%d",
	PullTitle:        " TermCode — Загрузка модели ",
	PullInterrupt:    " — прервать",
	PullDone:         "Готово!",
	QAHint:           "  Space — выбрать  ↑↓ — навигация  Enter — отправить  Esc — отмена",
	QASelected:       "  ✓ Выбрано: %d",
	ContextDropped:   "контекст: удалено %d старых сообщений",
}

func (m *Model) tr() i18nStrings {
	if m.cfg != nil && m.cfg.Language == "ru" {
		return i18nRU
	}
	return i18nEN
}

// ── Состояния TUI ─────────────────────────────────────────────────────────────

type state int

const (
	stateModelSelect   state = iota
	stateChat
	stateThinking
	statePulling
	stateQuestion
	statePalette
	stateSessionLoad // загрузка сохранённой сессии
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

// sessionsLoadedMsg — список сохранённых сессий загружен
type sessionsLoadedMsg struct {
	sessions []*session.Session
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
	currentState  state
	streaming     string      // буфер текущего стримингового ответа
	errMsg        string
	streamCancel  func()      // отмена текущего стрима (предотвращает goroutine leak)

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
	question         string
	questionOptions  []string
	questionCursor   int
	questionSelected map[int]bool // мульти-выбор: индекс → выбран ли

	// ── Палитра команд (Ctrl+P) ───────────────────────────────────────────
	paletteCursor  int
	paletteFilter  string
	paletteItems   []paletteItem

	// ── Загрузка сессий ───────────────────────────────────────────────────
	savedSessions    []*session.Session
	sessionCursor    int
	sessionsLoading  bool

	// ── Think-блоки (reasoning) ───────────────────────────────────────────
	thinkExpanded map[int]bool // msgIndex → раскрыт ли think-блок
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
	ta.Placeholder = i18nEN.Placeholder
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
	m.thinkExpanded = make(map[int]bool)
	m.questionSelected = make(map[int]bool)
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

	// ── Список сессий загружен ────────────────────────────────────────────────
	case sessionsLoadedMsg:
		m.sessionsLoading = false
		m.savedSessions = msg.sessions
		m.sessionCursor = 0
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

		// ── Клавиши в режиме загрузки сессий ─────────────────────────────────
		if m.currentState == stateSessionLoad {
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
				// Backspace на сессии — удалить её
				if m.sessionCursor < len(m.savedSessions) {
					return m.deleteSession(m.savedSessions[m.sessionCursor])
				}
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
				// +1 за поле своего ввода в конце
				maxCursor := len(m.questionOptions)
				if m.questionCursor < maxCursor {
					m.questionCursor++
				}
				return m, nil
			case tea.KeyRunes:
				if string(msg.Runes) == " " && m.questionCursor < len(m.questionOptions) {
					// Space — переключить выбор текущего варианта
					m.questionSelected[m.questionCursor] = !m.questionSelected[m.questionCursor]
					return m, nil
				}
			case tea.KeyEnter:
				return m.submitQuestionAnswer()
			case tea.KeyEsc:
				m.currentState = stateChat
				m.question = ""
				m.questionOptions = nil
				m.questionSelected = make(map[int]bool)
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
			if m.streamCancel != nil {
				m.streamCancel()
			}
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
		case tea.KeyRunes:
			if string(msg.Runes) == "T" {
				return m.toggleLastThink(), nil
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

	// Для Ollama — пробуем получить реальный контекст через /api/show
	// Если кешировано в ProviderConfig.ContextLength — используем кеш
	if m.cfg.ActiveProvider == config.ProviderOllama && pc.ContextLength == 0 {
		if detected, err := ai.FetchOllamaModelContext(pc.BaseURL, pc.Model); err == nil && detected > 0 {
			// Кешируем в конфиге чтобы не делать запрос каждый раз
			providersCopy := m.cfg.Providers
			pc.ContextLength = detected
			providersCopy[m.cfg.ActiveProvider] = pc
			m.cfg.Providers = providersCopy
			_ = m.cfg.Save()
			contextLength = detected
		}
	}

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

	systemPrompt := m.cfg.SystemPrompt + "\n\n" + tools.ToolDefs() +
		"\n\nWorking directory: " + m.workDir + langInstruction

	apiMsgs, dropped := ai.TrimMessages(rawMsgs, systemPrompt, contextLength-maxTokens)
	if dropped > 0 {
		m.errMsg = fmt.Sprintf(m.tr().ContextDropped, dropped)
	}

	m.contextUsed = ai.SumTokens(apiMsgs) + ai.EstimateTokens(systemPrompt)
	m.contextLimit = contextLength

	// Отменяем предыдущий стрим если он ещё идёт
	if m.streamCancel != nil {
		m.streamCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel

	provider := m.provider

	return func() tea.Msg {
		ch, err := provider.Stream(apiMsgs, systemPrompt, maxTokens)
		if err != nil {
			cancel()
			return aiChunkMsg{err: err}
		}

		select {
		case <-ctx.Done():
			// Стрим отменён — горутина провайдера завершится сама когда закроет ch
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
}

// streamReaderMsg — внутреннее сообщение для продолжения чтения стрима
type streamReaderMsg struct {
	content string
	done    bool
	ch      <-chan ai.StreamChunk
	ctx     context.Context
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
	ctx := msg.ctx
	return m, func() tea.Msg {
		// Уважаем отмену — если контекст закрыт, завершаем стрим
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
			}
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

// countTokens приближённо считает токены по словам (1 слово ≈ 1.3 токена)
func countTokens(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3 + 0.5)
}

// filterThinkTags убирает <think>...</think> блоки и одиночные теги из текста
func filterThinkTags(text string) string {
	result := text
	// Убираем полные блоки <think>...</think>
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
	// Убираем одиночные сиротские теги (GLM шлёт </think> без открывающего)
	result = strings.ReplaceAll(result, "</think>", "")
	result = strings.ReplaceAll(result, "<think>", "")
	return strings.TrimSpace(result)
}

// finalizeAIResponse вызывается когда стрим завершён
func (m Model) finalizeAIResponse() (tea.Model, tea.Cmd) {
	fullText := m.streaming
	m.streaming = ""

	// Сохраняем финальную статистику
	if elapsed := time.Since(m.genStartTime).Seconds(); elapsed > 0 {
		m.genSpeed = float64(m.genTokens) / elapsed
	}

	// В сессию сохраняем ОРИГИНАЛЬНЫЙ текст с <think> тегами
	// Это позволяет потом показать/скрыть reasoning
	// Для tool calls парсим только видимую часть
	visibleText := filterThinkTags(fullText)
	calls, cleanText := ai.ParseToolCalls(visibleText)

	// Сохраняем в сессию — но только если есть что сохранять
	rawForSession := replaceThinkForSession(fullText, cleanText)
	if strings.TrimSpace(cleanText) != "" || extractThinkContent(fullText) != "" {
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

	// Проверяем вопрос от AI
	if question, options := parseQuestionBlock(cleanText); question != "" {
		m.question = question
		m.questionOptions = options
		m.questionCursor = 0
		m.currentState = stateQuestion
		m.refreshViewport()
		m.scrollToBottom = true
		return m, nil
	}

	m.currentState = stateChat
	m.refreshViewport()
	m.scrollToBottom = true
	return m, nil
}

// replaceThinkForSession формирует текст для сохранения в сессию:
// оборачивает think-блок в специальный маркер, остальное — чистый текст
func replaceThinkForSession(rawText, cleanText string) string {
	// Если нет think-блоков — просто чистый текст
	if !strings.Contains(rawText, "<think>") {
		return cleanText
	}
	// Извлекаем think-контент
	think := extractThinkContent(rawText)
	if think == "" {
		return cleanText
	}
	// Формат: <!--think:CONTENT-->\ncleanText
	return "<!--think:" + think + "-->\n" + cleanText
}

// extractThinkContent извлекает содержимое первого <think>...</think> блока
func extractThinkContent(text string) string {
	start := strings.Index(text, "<think>")
	if start == -1 {
		return ""
	}
	inner := text[start+len("<think>"):]
	end := strings.Index(inner, "</think>")
	if end == -1 {
		return strings.TrimSpace(inner)
	}
	content := strings.TrimSpace(inner[:end])
	// Убираем вложенные теги если есть
	content = strings.ReplaceAll(content, "</think>", "")
	content = strings.ReplaceAll(content, "<think>", "")
	return content
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
		return m.tr().LoadingModels
	}

	// Экран выбора модели
	if m.currentState == stateModelSelect {
		return m.renderModelSelect()
	}

	// Экран загрузки модели (ollama pull)
	if m.currentState == statePulling {
		return m.renderPullScreen()
	}

	// Экран загрузки сессий
	if m.currentState == stateSessionLoad {
		return m.renderSessionLoad()
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

	title := headerStyle.Render(m.tr().ModelSelectTitle)
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

	sb.WriteString(keyHintStyle.Render(m.tr().ModelSelectHint))

	for i, model := range m.ollamaModels {
		if i == m.modelCursor {
			sb.WriteString(userBubbleStyle.Render(fmt.Sprintf("  ▶ %s", model)) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", model))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(keyHintStyle.Render(fmt.Sprintf(m.tr().ModelSelectCount, m.modelCursor+1, len(m.ollamaModels))))
	return sb.String()
}

// renderPullScreen — экран прогресса ollama pull
func (m Model) renderPullScreen() string {
	var sb strings.Builder

	title := headerStyle.Render(m.tr().PullTitle)
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
	sb.WriteString(keyStyle.Render("  Ctrl+C") + keyHintStyle.Render(m.tr().PullInterrupt))
	return sb.String()
}

// renderHeader отрисовывает верхнюю панель
func (m Model) renderHeader() string {
	title := headerStyle.Render(" TermCode ")

	// Язык интерфейса
	lang := m.cfg.Language
	if lang == "" {
		lang = "en"
	}
	langLabel := headerInfoStyle.Render(" " + strings.ToUpper(lang) + " ")

	providerInfo := fmt.Sprintf(" %s / %s ", m.provider.Name(), m.provider.Model())
	info := headerInfoStyle.Render(providerInfo)

	workDirShort := m.workDir
	if home, err := os.UserHomeDir(); err == nil {
		workDirShort = strings.Replace(workDirShort, home, "~", 1)
	}
	dirInfo := headerInfoStyle.Render(" 📁 " + workDirShort + " ")

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(langLabel) -
		lipgloss.Width(info) - lipgloss.Width(dirInfo)
	if gap < 0 {
		gap = 0
	}
	spacer := strings.Repeat(" ", gap)

	return title + langLabel + spacer + info + dirInfo
}

// renderInput отрисовывает область ввода
func (m Model) renderInput() string {
	// Обновляем placeholder по текущему языку
	m.input.Placeholder = m.tr().Placeholder

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
		left = statusBusyStyle.Render(m.spinner.View()+m.tr().StatusGenerating) +
			keyHintStyle.Render(speed)
	case stateChat:
		if m.errMsg != "" {
			left = statusErrStyle.Render("✗ " + m.errMsg)
		} else {
			left = statusOKStyle.Render(fmt.Sprintf(m.tr().StatusReady, len(m.sess.Messages)))
		}
		if m.genSpeed > 0 {
			right = keyHintStyle.Render(fmt.Sprintf(m.tr().StatusLastTok, m.genSpeed, m.genTokens))
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
	t := m.tr()
	lang := m.cfg.Language
	if lang == "" {
		lang = "en"
	}
	hints := []string{
		keyStyle.Render("Enter") + keyHintStyle.Render(t.HintSend),
		keyStyle.Render("Shift+Enter") + keyHintStyle.Render(t.HintNewline),
		keyStyle.Render("Ctrl+P") + keyHintStyle.Render(t.HintCommands),
		keyStyle.Render("/models") + keyHintStyle.Render(t.HintModels),
		keyStyle.Render("Ctrl+S") + keyHintStyle.Render(t.HintSave),
		keyStyle.Render("[" + strings.ToUpper(lang) + "]") + keyHintStyle.Render(t.HintLang),
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

	for msgIdx, msg := range m.sess.Messages {
		switch msg.Role {
		case session.RoleUser:
			sb.WriteString(userLabelStyle.Render("▶ Ты") + "\n")
			sb.WriteString(userBubbleStyle.Width(contentWidth).Render(msg.Content))
			sb.WriteString("\n\n")

		case session.RoleAssistant:
			sb.WriteString(assistantLabelStyle.Render("◆ TermCode") + "\n")

			// Рендерим с поддержкой think-блоков
			rendered := m.renderAssistantContent(msgIdx, msg.Content, contentWidth)
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

		// Определяем — идёт ли сейчас think-фаза
		streamText := m.streaming
		inThink := isInsideThink(streamText)
		visible := filterThinkTags(streamText)

		if inThink {
			// Показываем индикатор что модель "думает"
			thinkIndicator := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5C6370")).Italic(true).
				Render(m.tr().Thinking)
			if visible != "" {
				rendered := renderMarkdown(visible, contentWidth)
				sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(thinkIndicator + "\n\n" + rendered))
			} else {
				sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(thinkIndicator))
			}
		} else {
			rendered := renderMarkdown(visible, contentWidth)
			sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(rendered))
		}
		sb.WriteString(" ▋\n")
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
					label = lipgloss.NewStyle().Foreground(colorMuted).Render(" "+codeLang+" ") + "\n"
				}
				// Применяем syntax highlighting
				highlighted := HighlightCode(code, codeLang)
				sb.WriteString(codeBlockStyle.Width(width-4).Render(label + highlighted))
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
		highlighted := HighlightCode(strings.Join(codeLines, "\n"), codeLang)
		sb.WriteString(codeBlockStyle.Width(width-4).Render(highlighted))
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
	pc := m.cfg.Providers[m.cfg.ActiveProvider]
	pc.Model = name
	// Сбрасываем кеш контекста — новая модель может иметь другой лимит
	pc.ContextLength = 0
	m.cfg.Providers[m.cfg.ActiveProvider] = pc
	_ = m.cfg.Save()

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

	return m, tea.Batch(m.spinner.Tick, streamOllamaPull(baseURL, modelName, m.tr().PullDone))
}

// streamOllamaPull стримит прогресс ollama pull через /api/pull
func streamOllamaPull(baseURL, modelName, pullDoneStr string) tea.Cmd {
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
				return pullProgressMsg{status: pullDoneStr, done: true}
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

// parseQuestionBlock ищет вопрос от AI в тексте ответа.
// Поддерживает несколько форматов которые реально используют модели:
//
// Формат 1 — явный блок (наш формат):
//
//	```question
//	Текст вопроса?
//	- Вариант A
//	- Вариант B
//	```
//
// Формат 2 — вопрос + нумерованный список:
//
//	Какой подход выбрать?
//	1. Вариант A
//	2. Вариант B
//
// Формат 3 — вопрос + маркированный список:
//
//	Что добавить?
//	- Вариант A
//	- Вариант B
//
// Формат 4 — вопрос в конце текста с вариантами:
//
//	...объяснение...
//	Что бы вы хотели улучшить в первую очередь?
//	- Вариант A
//	- Вариант B
func parseQuestionBlock(text string) (question string, options []string) {
	// Формат 1: явный ```question блок — самый надёжный
	const openTag = "```question"
	if start := strings.Index(text, openTag); start != -1 {
		inner := text[start+len(openTag):]
		if end := strings.Index(inner, "```"); end != -1 {
			inner = strings.TrimSpace(inner[:end])
		} else {
			inner = strings.TrimSpace(inner)
		}
		lines := strings.Split(inner, "\n")
		if len(lines) > 0 {
			question = strings.TrimSpace(lines[0])
			for _, line := range lines[1:] {
				line = strings.TrimSpace(line)
				if opt := extractListItem(line); opt != "" {
					options = append(options, opt)
				}
			}
			if question != "" {
				return question, options
			}
		}
	}

	// Форматы 2-4: эвристический поиск — вопрос + список вариантов
	// Условия срабатывания:
	// - строка оканчивается на ? (вопрос)
	// - сразу после неё идут 2+ строки с маркерами (-, *, •, 1., 2., ...)
	// - список не слишком длинный (не TOC и не обычный текст)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !looksLikeQuestion(line) {
			continue
		}

		// Собираем варианты начиная со следующей строки
		var opts []string
		for j := i + 1; j < len(lines) && j < i+16; j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" {
				// Пустая строка — продолжаем если вариантов ещё нет
				if len(opts) > 0 {
					break
				}
				continue
			}
			if opt := extractListItem(l); opt != "" {
				opts = append(opts, opt)
			} else if len(opts) == 0 {
				// Ещё не нашли ни одного варианта — пропускаем вводные строки
				// типа "Я могу помочь с:" или "Варианты:"
				// но только если строка короткая и не похожа на абзац
				if len(l) < 80 && (strings.HasSuffix(l, ":") || strings.HasSuffix(l, ".")) {
					continue
				}
				// Длинный абзац — это не список, прерываем
				break
			} else {
				// Уже есть варианты — любая не-список строка = конец
				break
			}
		}

		// Требуем минимум 2 варианта чтобы избежать ложных срабатываний
		if len(opts) >= 2 {
			return line, opts
		}
	}

	return "", nil
}

// looksLikeQuestion возвращает true если строка похожа на вопрос
func looksLikeQuestion(s string) bool {
	if s == "" {
		return false
	}
	// Самый надёжный критерий — знак вопроса
	if strings.HasSuffix(s, "?") {
		return true
	}
	// Без ? — только если строка содержит явные вопросительные слова
	// И при этом не слишком короткая (чтобы не цеплять "что:" и т.п.)
	if len(s) < 15 {
		return false
	}
	lower := strings.ToLower(s)
	// Только фразы которые однозначно указывают на вопрос без знака ?
	questionPhrases := []string{
		"what would you", "which would you", "what do you",
		"что бы вы хотели", "что бы ты хотел", "что вы хотите",
		"что ты хочешь", "что предпочитаете", "что предпочитаешь",
	}
	for _, phrase := range questionPhrases {
		if strings.Contains(lower, phrase) && len(s) < 200 {
			return true
		}
	}
	return false
}

// extractListItem извлекает текст из строки-элемента списка
// Поддерживает: "- текст", "* текст", "• текст", "1. текст", "1) текст"
func extractListItem(s string) string {
	if s == "" {
		return ""
	}
	// Маркированный список: - * •
	for _, prefix := range []string{"- ", "* ", "• ", "– ", "— "} {
		if strings.HasPrefix(s, prefix) {
			opt := strings.TrimSpace(strings.TrimPrefix(s, prefix))
			if opt != "" && len(opt) < 200 {
				return opt
			}
		}
	}
	// Нумерованный список: "1. " "2) " "1: "
	if len(s) >= 3 {
		// Цифра в начале
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i > 0 && i < len(s) {
			sep := s[i]
			if sep == '.' || sep == ')' || sep == ':' {
				rest := strings.TrimSpace(s[i+1:])
				// Убираем вложенные маркеры типа "1. **Вариант**"
				rest = strings.TrimPrefix(rest, "**")
				rest = strings.TrimSuffix(rest, "**")
				if rest != "" && len(rest) < 200 {
					return rest
				}
			}
		}
	}
	return ""
}

// submitQuestionAnswer отправляет выбранный ответ на вопрос AI
func (m Model) submitQuestionAnswer() (tea.Model, tea.Cmd) {
	var answer string
	customText := strings.TrimSpace(m.input.Value())

	if customText != "" {
		// Пользователь написал свой вариант
		answer = customText
	} else if len(m.questionSelected) > 0 {
		// Есть выбранные чекбоксом варианты — собираем все
		var selected []string
		for i, opt := range m.questionOptions {
			if m.questionSelected[i] {
				selected = append(selected, opt)
			}
		}
		if len(selected) == 1 {
			answer = selected[0]
		} else if len(selected) > 1 {
			answer = strings.Join(selected, ", ")
		}
	} else if m.questionCursor < len(m.questionOptions) {
		// Курсор стоит на варианте — одиночный выбор Enter
		answer = m.questionOptions[m.questionCursor]
	}

	if answer == "" {
		return m, nil
	}

	m.question = ""
	m.questionOptions = nil
	m.questionCursor = 0
	m.questionSelected = make(map[int]bool)
	m.input.Reset()
	m.input.Placeholder = "Ask anything... (Enter to send, Shift+Enter for newline)"
	m.currentState = stateThinking
	m.streaming = ""
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.genSpeed = 0

	m.sess.AddMessage(session.RoleUser, answer)
	m.refreshViewport()
	m.scrollToBottom = true

	return m, tea.Batch(m.streamAI(), m.spinner.Tick)
}

// renderQuestionPanel отрисовывает панель выбора ответа на вопрос AI
func (m Model) renderQuestionPanel() string {
	var sb strings.Builder
	w := m.width - 2

	// ── Заголовок вопроса ─────────────────────────────────────────────────
	questionText := lipgloss.NewStyle().
		Foreground(colorPrimary).Bold(true).
		Render("❓ " + m.question)
	hint := lipgloss.NewStyle().
		Foreground(colorMuted).Italic(true).
		Render(m.tr().QAHint)
	sb.WriteString(questionText + "\n")
	sb.WriteString(hint + "\n\n")

	// ── Кнопки-чекбоксы ──────────────────────────────────────────────────
	for i, opt := range m.questionOptions {
		isSelected := m.questionSelected[i]
		isCursor := m.questionCursor == i

		// Иконка чекбокса
		var checkbox string
		if isSelected {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")).Bold(true).Render("✓")
		} else {
			checkbox = lipgloss.NewStyle().Foreground(colorMuted).Render("○")
		}

		// Текст кнопки
		label := fmt.Sprintf(" %s  %s", checkbox, opt)

		var btn string
		if isCursor && isSelected {
			// Курсор + выбран: яркая зелёная кнопка
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#2D4A2D")).
				Foreground(lipgloss.Color("#98C379")).Bold(true).
				Padding(0, 1).Width(w - 2).
				Render("▶" + label)
		} else if isCursor {
			// Только курсор: подсвеченная кнопка
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#2C313A")).
				Foreground(colorText).Bold(true).
				Padding(0, 1).Width(w - 2).
				Render("▶" + label)
		} else if isSelected {
			// Только выбран: зелёная кнопка без курсора
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E3A1E")).
				Foreground(lipgloss.Color("#98C379")).
				Padding(0, 1).Width(w - 2).
				Render(" " + label)
		} else {
			// Обычная кнопка
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#21252B")).
				Foreground(colorText).
				Padding(0, 1).Width(w - 2).
				Render(" " + label)
		}

		sb.WriteString(btn + "\n")
	}

	// ── Поле своего ввода ────────────────────────────────────────────────
	sb.WriteString("\n")
	isInputFocused := m.questionCursor == len(m.questionOptions)
	prompt := inputPromptStyle.Render("✏ ")

	var inputBox string
	if isInputFocused {
		inputBox = inputContainerFocusStyle.Width(w).Render(prompt + m.input.View())
	} else {
		inputBox = inputContainerStyle.Width(w).Render(prompt + m.input.View())
	}
	sb.WriteString(inputBox)

	// ── Статус выбора ─────────────────────────────────────────────────────
	if len(m.questionSelected) > 0 {
		count := 0
		for _, v := range m.questionSelected {
			if v {
				count++
			}
		}
		if count > 0 {
			sb.WriteString("\n" + lipgloss.NewStyle().
				Foreground(lipgloss.Color("#98C379")).
				Render(fmt.Sprintf(m.tr().QASelected, count)))
		}
	}

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
			key: "lang", title: "Сменить язык / Switch Language",
			description: "EN ↔ RU — язык интерфейса и ответов AI",
			action: func(m Model) (Model, tea.Cmd) {
				m.paletteFilter = ""
				m.currentState = stateChat
				if m.cfg.Language == "ru" {
					m.cfg.Language = "en"
				} else {
					m.cfg.Language = "ru"
				}
				_ = m.cfg.Save()
				return m, nil
			},
		},
		{
			key: "sessions", title: "Загрузить сессию",
			description: "Открыть список сохранённых диалогов",
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateSessionLoad
				m.sessionsLoading = true
				m.paletteFilter = ""
				return m, loadSessions()
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

// ── Загрузка и управление сессиями ───────────────────────────────────────────

// loadSessions асинхронно читает список сессий с диска
func loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, _ := session.LoadAll()
		return sessionsLoadedMsg{sessions: sessions}
	}
}

// loadSession загружает выбранную сессию и переходит в чат
func (m Model) loadSession(s *session.Session) (tea.Model, tea.Cmd) {
	// Сохраняем текущую сессию перед переключением
	_ = m.sess.Save()

	m.sess = s
	m.currentState = stateChat
	m.streaming = ""
	m.errMsg = ""
	m.contextUsed = 0
	m.thinkExpanded = make(map[int]bool)

	m.refreshViewport()
	m.scrollToBottom = true
	return m, nil
}

// deleteSession удаляет сессию и перезагружает список
func (m Model) deleteSession(s *session.Session) (tea.Model, tea.Cmd) {
	_ = session.Delete(s.ID)
	m.sessionsLoading = true
	return m, loadSessions()
}

// renderSessionLoad — экран выбора сессии для загрузки
func (m Model) renderSessionLoad() string {
	var sb strings.Builder

	title := headerStyle.Render(" TermCode — Сессии ")
	sb.WriteString(title + "\n\n")

	if m.sessionsLoading {
		sb.WriteString(fmt.Sprintf("  %s Загружаем сессии...\n", m.spinner.View()))
		return sb.String()
	}

	if len(m.savedSessions) == 0 {
		sb.WriteString(keyHintStyle.Render("  Нет сохранённых сессий.\n\n"))
		sb.WriteString(keyStyle.Render("  Esc") + keyHintStyle.Render(" — назад\n"))
		return sb.String()
	}

	sb.WriteString(keyHintStyle.Render(
		"  ↑↓ навигация  Enter загрузить  Backspace удалить  Esc назад\n\n"))

	for i, s := range m.savedSessions {
		// Форматируем дату
		age := formatAge(s.UpdatedAt)
		msgs := fmt.Sprintf("%d сообщ.", len(s.Messages))
		model := s.Model
		if len(model) > 20 {
			model = model[:18] + ".."
		}

		line := fmt.Sprintf("  %-40s  %-8s  %-22s  %s",
			truncate(s.Title, 40),
			msgs,
			model,
			age,
		)

		if i == m.sessionCursor {
			sb.WriteString(userBubbleStyle.Width(m.width-2).Render("▶"+line) + "\n")
		} else {
			sb.WriteString(keyHintStyle.Render(" "+line) + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\n  %d сессий сохранено", len(m.savedSessions)))
	return sb.String()
}

// formatAge возвращает человекочитаемое время ("2ч назад", "3д назад")
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "только что"
	case d < time.Hour:
		return fmt.Sprintf("%dм назад", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dч назад", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dд назад", int(d.Hours()/24))
	default:
		return t.Format("02.01.06")
	}
}

// truncate обрезает строку до maxLen символов
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-2]) + ".."
}

// ── Think-блоки ───────────────────────────────────────────────────────────────

// isInsideThink возвращает true если текст заканчивается внутри <think> блока
func isInsideThink(text string) bool {
	openCount := strings.Count(text, "<think>")
	closeCount := strings.Count(text, "</think>")
	return openCount > closeCount
}

// renderAssistantContent рендерит сообщение ассистента с поддержкой think-блоков
func (m Model) renderAssistantContent(msgIdx int, content string, width int) string {
	// Проверяем наш формат <!--think:CONTENT-->\nVISIBLE
	const thinkPrefix = "<!--think:"
	const thinkSuffix = "-->"

	if !strings.HasPrefix(content, thinkPrefix) {
		// Нет think-блока — обычный рендер
		return renderMarkdown(content, width)
	}

	// Парсим think и visible части
	rest := content[len(thinkPrefix):]
	endIdx := strings.Index(rest, thinkSuffix)
	if endIdx < 0 {
		return renderMarkdown(content, width)
	}
	thinkContent := rest[:endIdx]
	visible := strings.TrimPrefix(rest[endIdx+len(thinkSuffix):], "\n")

	// Стиль заголовка think-блока
	expanded := m.thinkExpanded[msgIdx]
	var thinkHeader string
	if expanded {
		thinkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).Italic(true).
			Render("🧠 Мысли [T — скрыть]")
	} else {
		thinkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).Italic(true).
			Render("🧠 Мысли [T — показать]")
	}

	var sb strings.Builder
	sb.WriteString(thinkHeader + "\n")

	if expanded && thinkContent != "" {
		// Показываем think-контент в затемнённом стиле
		thinkStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#636D83")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#3E4451")).
			PaddingLeft(1)
		// Обрезаем если очень длинный
		display := thinkContent
		if len(display) > 2000 {
			display = display[:1997] + "..."
		}
		sb.WriteString(thinkStyle.Width(width-4).Render(display))
		sb.WriteString("\n")
	}

	if visible != "" {
		sb.WriteString("\n")
		sb.WriteString(renderMarkdown(visible, width))
	}

	return sb.String()
}

// toggleLastThink переключает видимость think-блока последнего сообщения ассистента
func (m Model) toggleLastThink() Model {
	// Ищем последнее сообщение ассистента с think-блоком
	for i := len(m.sess.Messages) - 1; i >= 0; i-- {
		msg := m.sess.Messages[i]
		if msg.Role == session.RoleAssistant && strings.HasPrefix(msg.Content, "<!--think:") {
			m.thinkExpanded[i] = !m.thinkExpanded[i]
			m.refreshViewport()
			return m
		}
	}
	return m
}
