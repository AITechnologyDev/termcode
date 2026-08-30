package tui

import (
	"context"
	"time"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/NekoFemDev/termcode/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// state — режим TUI
type state int

const (
	stateModelSelect state = iota
	stateChat
	stateThinking
	statePulling
	stateQuestion
	statePalette
	stateSessionLoad
	stateProviderSelect
	stateProfileEdit
	stateInstructEdit
	stateRetrying
)

// Сообщения BubbleTea

type aiChunkMsg struct {
	content    string
	done       bool
	err        error
	retryCount int
}

type toolDoneMsg struct {
	call   ai.ToolCallRequest
	result tools.Result
}

type saveSessionMsg struct{}

type contextDetectedMsg struct {
	contextLength   int
	maxOutputTokens int
	err             error
}

type ollamaModelsMsg struct {
	models []string
	err    error
}

type pullProgressMsg struct {
	status    string
	completed int64
	total     int64
	done      bool
	err       error
}

type sessionsLoadedMsg struct {
	sessions []*session.Session
}

// streamReaderMsg — рекурсивное чтение следующего чанка стрима
type streamReaderMsg struct {
	content string
	done    bool
	ch      <-chan ai.StreamChunk
	ctx     context.Context
}

type streamTickMsg struct{}

type retryMsg struct{}

// Model — главная модель BubbleTea
type Model struct {
	cfg      *config.Config
	provider ai.Provider

	sess    *session.Session
	workDir string

	executor *tools.Executor

	currentState state
	streaming    string
	errMsg       string

	// streamCancel — функция отмены активного стрима.
	// Устанавливается синхронно в streamAI() и затем сразу вызывается из cancelStream() синхронно.
	// Доступ только из главного цикла BubbleTea (без конкурентности), поэтому
	// обычный указатель достаточен.
	streamCancel context.CancelFunc

	// UI components
	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	width  int
	height int

	// Пиксельные размеры (для Termux)
	screenWidthPx  int
	screenHeightPx int

	scrollToBottom bool

	// ── Выбор модели при старте ───────────────────────────────────────
	ollamaModels  []string
	modelCursor   int
	modelsLoading bool

	// ── Ollama pull ───────────────────────────────────────────────────
	pullModelName string
	pullStatus    string
	pullCompleted int64
	pullTotal     int64

	// ── Статистика генерации ──────────────────────────────────────────
	genStartTime time.Time
	genTokens    int
	genSpeed     float64

	// ── Использование контекста ───────────────────────────────────────
	contextUsed  int
	contextLimit int

	// ── Интерактивный вопрос от AI ────────────────────────────────────
	question         string
	questionOptions  []string
	questionCursor   int
	questionSelected map[int]bool
	questionMulti    bool
	questionToolCall bool

	// ── Палитра команд (Ctrl+P) ───────────────────────────────────────
	paletteCursor int
	paletteFilter string
	paletteItems  []paletteItem

	// ── Загрузка сессий ───────────────────────────────────────────────
	savedSessions   []*session.Session
	sessionCursor   int
	sessionsLoading bool

	// ── Выбор провайдера ──────────────────────────────────────────────
	providerCursor   int
	providerEditMode int // 0=none 1=edit-key 2=edit-url 3=edit-model

	// ── Редактор профиля/инструкций ───────────────────────────────────
	editInput textarea.Model
	editMode  int

	// ── Think-блоки (reasoning) ───────────────────────────────────────
	thinkExpanded map[int]bool

	// ── Retry логика ──────────────────────────────────────────────────
	retryCount     int
	maxRetries     int
	pendingContent string

	// ── Буферизация стриминга ─────────────────────────────────────────
	fullResponse   string
	toolsExtracted bool
	displayedLen   int
}

// paletteItem — одна команда в палитре
type paletteItem struct {
	key         string
	title       string
	description string
	action      func(m Model) (Model, tea.Cmd)
}
