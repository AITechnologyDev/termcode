package tui

import "github.com/charmbracelet/lipgloss"

// Цветовая палитра (тёмная тема)
var (
	colorPrimary   = lipgloss.Color("#A78BFA") // светло-фиолетовый — акцент
	colorSecondary = lipgloss.Color("#22D3EE") // циан
	colorAccent    = lipgloss.Color("#F472B6") // розовый — для активного
	colorSuccess   = lipgloss.Color("#34D399") // зелёный
	colorWarning   = lipgloss.Color("#FBBF24") // жёлтый
	colorError     = lipgloss.Color("#F87171") // красный
	colorMuted     = lipgloss.Color("#6B7280") // серый
	colorBg        = lipgloss.Color("#111827") // фон
	colorBgLight   = lipgloss.Color("#1F2937") // подложка плашек
	colorBgSubtle  = lipgloss.Color("#374151") // граница выделения
	colorText      = lipgloss.Color("#F9FAFB") // основной текст
	colorBorder    = lipgloss.Color("#4B5563") // граница
	colorLink      = lipgloss.Color("#60A5FA") // ссылки
)

// Стили компонентов
var (
	// ── Заголовок / хедер ───────────────────────────────────────────────────
	headerStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(lipgloss.Color("#1F2937")).
			Bold(true).
			Padding(0, 2)

	headerPillStyle = lipgloss.NewStyle().
			Background(colorBgSubtle).
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)

	headerInfoStyle = lipgloss.NewStyle().
			Background(colorBgLight).
			Foreground(colorSecondary).
			Padding(0, 1)

	langPillStyle = lipgloss.NewStyle().
			Background(colorAccent).
			Foreground(lipgloss.Color("#1F2937")).
			Bold(true).
			Padding(0, 1)

	// ── Сообщения чата ──────────────────────────────────────────────────────
	userBubbleStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1).
			MarginTop(1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorPrimary)

	assistantBubbleStyle = lipgloss.NewStyle().
				Foreground(colorText).
				MarginTop(1)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	// ── Tool calls ──────────────────────────────────────────────────────────
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

	// ── Строка ввода ────────────────────────────────────────────────────────
	inputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	inputContainerFocusStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(colorSecondary).
					Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	// ── Статусная строка ────────────────────────────────────────────────────
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

	// ── Подсказки клавиш ────────────────────────────────────────────────────
	keyHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	linkStyle = lipgloss.NewStyle().
			Foreground(colorLink).
			Underline(true)

	// ── Разделитель ─────────────────────────────────────────────────────────
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorBorder)

	// ── Код-блоки ───────────────────────────────────────────────────────────
	codeBlockStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorText).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorSecondary)

	// ── Spinner ─────────────────────────────────────────────────────────────
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// ── Теги провайдеров/моделей ───────────────────────────────────────────
	tagLocalStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#064E3B")).
			Foreground(colorSuccess).
			Bold(true).
			Padding(0, 1)

	tagCloudStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3A8A")).
			Foreground(lipgloss.Color("#93C5FD")).
			Bold(true).
			Padding(0, 1)

	tagFreeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7C2D12")).
			Foreground(lipgloss.Color("#FED7AA")).
			Bold(true).
			Padding(0, 1)

	// ── Provider select ────────────────────────────────────────────────────
	providerActiveStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	providerSelectedStyle = lipgloss.NewStyle().
				Background(colorBgSubtle).
				Bold(true)

	// ── API key status ─────────────────────────────────────────────────────
	keyStatusOKStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#064E3B")).
				Foreground(colorSuccess).
				Bold(true).
				Padding(0, 1)

	keyStatusBadStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7F1D1D")).
				Foreground(colorError).
				Bold(true).
				Padding(0, 1)

	keyStatusNAStyle = lipgloss.NewStyle().
				Background(colorBgLight).
				Foreground(colorMuted).
				Padding(0, 1)

	// ── Palette ─────────────────────────────────────────────────────────────
	paletteSelectedStyle = lipgloss.NewStyle().
				Background(colorBgSubtle).
				Foreground(colorText)
)
