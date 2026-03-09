package tui

import "github.com/charmbracelet/lipgloss"

// Цветовая палитра (dark theme)
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // фиолетовый — акцент
	colorSecondary = lipgloss.Color("#06B6D4") // циан
	colorSuccess   = lipgloss.Color("#10B981") // зелёный
	colorWarning   = lipgloss.Color("#F59E0B") // жёлтый
	colorError     = lipgloss.Color("#EF4444") // красный
	colorMuted     = lipgloss.Color("#6B7280") // серый
	colorBg        = lipgloss.Color("#1F2937") // тёмный фон
	colorBgLight   = lipgloss.Color("#374151") // немного светлее
	colorText      = lipgloss.Color("#F9FAFB") // белый текст
	colorBorder    = lipgloss.Color("#4B5563") // граница
)

// Стили компонентов
var (
	// ── Общий контейнер ──────────────────────────────────────────────────────
	appStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// ── Заголовок / хедер ────────────────────────────────────────────────────
	headerStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorText).
			Bold(true).
			Padding(0, 2)

	headerInfoStyle = lipgloss.NewStyle().
			Background(colorBgLight).
			Foreground(colorSecondary).
			Padding(0, 1)

	// ── Сообщения чата ───────────────────────────────────────────────────────
	userBubbleStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorText).
			Padding(0, 1).
			MarginTop(1)

	assistantBubbleStyle = lipgloss.NewStyle().
				Foreground(colorText).
				MarginTop(1)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	// ── Tool calls ───────────────────────────────────────────────────────────
	toolCallStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorWarning).
			Foreground(colorWarning).
			Padding(0, 1).
			MarginTop(1)

	toolResultStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSuccess).
			Foreground(colorSuccess).
			Padding(0, 1)

	toolErrorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorError).
			Foreground(colorError).
			Padding(0, 1)

	// ── Строка ввода ─────────────────────────────────────────────────────────
	inputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	inputContainerFocusStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(colorSecondary).
					Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	// ── Статусная строка ─────────────────────────────────────────────────────
	statusBarStyle = lipgloss.NewStyle().
			Background(colorBgLight).
			Foreground(colorMuted).
			Padding(0, 1)

	statusOKStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	statusBusyStyle = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	statusErrStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	// ── Подсказки клавиш ─────────────────────────────────────────────────────
	keyHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// ── Разделитель ──────────────────────────────────────────────────────────
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorBorder)

	// ── Код-блоки ────────────────────────────────────────────────────────────
	codeBlockStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorText).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorSecondary)

	// ── Заголовок сессии ─────────────────────────────────────────────────────
	sessionTitleStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	// ── Spinner ───────────────────────────────────────────────────────────────
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)
)
