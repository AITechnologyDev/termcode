package tui

import "github.com/charmbracelet/lipgloss"

// ThemeOverride — оверрайды палитры. Поля соответствуют
// plugin/host.Theme; конвертация делается в main.go, чтобы tui не
// зависел от plugin/host напрямую.
type ThemeOverride struct {
	Primary   string
	Secondary string
	Accent    string
	Success   string
	Warning   string
	Error     string
	Muted     string
	Bg        string
	BgLight   string
	BgSubtle  string
	Border    string
	Text      string
	Link      string
}

// Цветовая палитра (тёмная тема). Поля — *lipgloss.Color, потому что
// плагины могут менять их на старте через applyTheme.
var (
	colorPrimary   = colorPtr("#A78BFA") // светло-фиолетовый — акцент
	colorSecondary = colorPtr("#22D3EE") // циан
	colorAccent    = colorPtr("#F472B6") // розовый — для активного
	colorSuccess   = colorPtr("#34D399") // зелёный
	colorWarning   = colorPtr("#FBBF24") // жёлтый
	colorError     = colorPtr("#F87171") // красный
	colorMuted     = colorPtr("#6B7280") // серый
	colorBg        = colorPtr("#111827") // фон
	colorBgLight   = colorPtr("#1F2937") // подложка плашек
	colorBgSubtle  = colorPtr("#374151") // граница выделения
	colorText      = colorPtr("#F9FAFB") // основной текст
	colorBorder    = colorPtr("#4B5563") // граница
	colorLink      = colorPtr("#60A5FA") // ссылки
)

func colorPtr(hex string) *lipgloss.Color {
	c := lipgloss.Color(hex)
	return &c
}

func setColor(dst *lipgloss.Color, hex string) {
	if hex == "" {
		return
	}
	*dst = lipgloss.Color(hex)
}

// applyTheme переписывает палитру из ThemeOverride (пустые поля
// игнорируются) и пересоздаёт зависимые стили. Вызывается один раз
// при старте.
func applyTheme(t *ThemeOverride) {
	if t == nil {
		return
	}
	setColor(colorPrimary, t.Primary)
	setColor(colorSecondary, t.Secondary)
	setColor(colorAccent, t.Accent)
	setColor(colorSuccess, t.Success)
	setColor(colorWarning, t.Warning)
	setColor(colorError, t.Error)
	setColor(colorMuted, t.Muted)
	setColor(colorBg, t.Bg)
	setColor(colorBgLight, t.BgLight)
	setColor(colorBgSubtle, t.BgSubtle)
	setColor(colorBorder, t.Border)
	setColor(colorText, t.Text)
	setColor(colorLink, t.Link)
	// После изменения палитры — пересоздаём стили, зависящие от цветов.
	rebuildStyles()
}

// ── Стили компонентов ─────────────────────────────────────────────────
//
// Стили пересоздаются в rebuildStyles() при applyTheme. Сами var-объявления
// ниже — это просто инициализация по умолчанию.

var (
	// ── Заголовок / хедер ───────────────────────────────────────────────
	headerStyle = lipgloss.NewStyle().
			Background(*colorPrimary).
			Foreground(lipgloss.Color("#1F2937")).
			Bold(true).
			Padding(0, 2)

	headerPillStyle = lipgloss.NewStyle().
			Background(*colorBgSubtle).
			Foreground(*colorPrimary).
			Bold(true).
			Padding(0, 1)

	headerInfoStyle = lipgloss.NewStyle().
			Background(*colorBgLight).
			Foreground(*colorSecondary).
			Padding(0, 1)

	langPillStyle = lipgloss.NewStyle().
			Background(*colorAccent).
			Foreground(lipgloss.Color("#1F2937")).
			Bold(true).
			Padding(0, 1)

	// ── Сообщения чата ──────────────────────────────────────────────────
	userBubbleStyle = lipgloss.NewStyle().
			Foreground(*colorText).
			Padding(0, 1).
			MarginTop(1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(*colorPrimary)

	assistantBubbleStyle = lipgloss.NewStyle().
				Foreground(*colorText).
				MarginTop(1)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(*colorPrimary).
			Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
				Foreground(*colorSecondary).
				Bold(true)

	// ── Tool calls ──────────────────────────────────────────────────────
	toolCallStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(*colorWarning).
			Foreground(*colorWarning).
			Padding(0, 1).
			MarginTop(1)

	toolResultStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(*colorSuccess).
			Foreground(*colorSuccess).
			Padding(0, 1)

	toolErrorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(*colorError).
			Foreground(*colorError).
			Padding(0, 1)

	// ── Строка ввода ────────────────────────────────────────────────────
	inputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(*colorBorder).
				Padding(0, 1)

	inputContainerFocusStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(*colorSecondary).
					Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(*colorPrimary).
				Bold(true)

	// ── Статусная строка ────────────────────────────────────────────────
	statusBarStyle = lipgloss.NewStyle().
			Background(*colorBgLight).
			Foreground(*colorMuted).
			Padding(0, 1)

	statusOKStyle = lipgloss.NewStyle().
			Foreground(*colorSuccess).
			Bold(true)

	statusBusyStyle = lipgloss.NewStyle().
			Foreground(*colorWarning).
			Bold(true)

	statusErrStyle = lipgloss.NewStyle().
			Foreground(*colorError).
			Bold(true)

	// ── Подсказки клавиш ────────────────────────────────────────────────
	keyHintStyle = lipgloss.NewStyle().
			Foreground(*colorMuted)

	keyStyle = lipgloss.NewStyle().
			Foreground(*colorSecondary).
			Bold(true)

	linkStyle = lipgloss.NewStyle().
			Foreground(*colorLink).
			Underline(true)

	// ── Разделитель ─────────────────────────────────────────────────────
	dividerStyle = lipgloss.NewStyle().
			Foreground(*colorBorder)

	// ── Код-блоки ───────────────────────────────────────────────────────
	codeBlockStyle = lipgloss.NewStyle().
			Background(*colorBg).
			Foreground(*colorText).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(*colorSecondary)

	// ── Spinner ─────────────────────────────────────────────────────────
	spinnerStyle = lipgloss.NewStyle().
			Foreground(*colorSecondary)

	// ── Теги провайдеров/моделей ───────────────────────────────────────
	tagLocalStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#064E3B")).
			Foreground(*colorSuccess).
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

	// ── Provider select ────────────────────────────────────────────────
	providerActiveStyle = lipgloss.NewStyle().
				Foreground(*colorAccent).
				Bold(true)

	providerSelectedStyle = lipgloss.NewStyle().
				Background(*colorBgSubtle).
				Bold(true)

	// ── API key status ─────────────────────────────────────────────────
	keyStatusOKStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#064E3B")).
				Foreground(*colorSuccess).
				Bold(true).
				Padding(0, 1)

	keyStatusBadStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7F1D1D")).
				Foreground(*colorError).
				Bold(true).
				Padding(0, 1)

	keyStatusNAStyle = lipgloss.NewStyle().
				Background(*colorBgLight).
				Foreground(*colorMuted).
				Padding(0, 1)

	// ── Palette ─────────────────────────────────────────────────────────
	paletteSelectedStyle = lipgloss.NewStyle().
				Background(*colorBgSubtle).
				Foreground(*colorText)
)

// rebuildStyles пересоздаёт все стили, зависящие от палитры. Вызывается
// после applyTheme.
func rebuildStyles() {
	headerStyle = headerStyle.Background(*colorPrimary)
	headerPillStyle = headerPillStyle.Background(*colorBgSubtle).Foreground(*colorPrimary)
	headerInfoStyle = headerInfoStyle.Background(*colorBgLight).Foreground(*colorSecondary)
	langPillStyle = langPillStyle.Background(*colorAccent)

	userBubbleStyle = userBubbleStyle.Foreground(*colorText).BorderForeground(*colorPrimary)
	assistantBubbleStyle = assistantBubbleStyle.Foreground(*colorText)
	userLabelStyle = userLabelStyle.Foreground(*colorPrimary)
	assistantLabelStyle = assistantLabelStyle.Foreground(*colorSecondary)

	toolCallStyle = toolCallStyle.BorderForeground(*colorWarning).Foreground(*colorWarning)
	toolResultStyle = toolResultStyle.BorderForeground(*colorSuccess).Foreground(*colorSuccess)
	toolErrorStyle = toolErrorStyle.BorderForeground(*colorError).Foreground(*colorError)

	inputContainerStyle = inputContainerStyle.BorderForeground(*colorBorder)
	inputContainerFocusStyle = inputContainerFocusStyle.BorderForeground(*colorSecondary)
	inputPromptStyle = inputPromptStyle.Foreground(*colorPrimary)

	statusBarStyle = statusBarStyle.Background(*colorBgLight).Foreground(*colorMuted)
	statusOKStyle = statusOKStyle.Foreground(*colorSuccess)
	statusBusyStyle = statusBusyStyle.Foreground(*colorWarning)
	statusErrStyle = statusErrStyle.Foreground(*colorError)

	keyHintStyle = keyHintStyle.Foreground(*colorMuted)
	keyStyle = keyStyle.Foreground(*colorSecondary)
	linkStyle = linkStyle.Foreground(*colorLink)
	dividerStyle = dividerStyle.Foreground(*colorBorder)

	codeBlockStyle = codeBlockStyle.Background(*colorBg).Foreground(*colorText).BorderForeground(*colorSecondary)
	spinnerStyle = spinnerStyle.Foreground(*colorSecondary)

	tagLocalStyle = tagLocalStyle.Foreground(*colorSuccess)
	tagFreeStyle = tagFreeStyle.Foreground(lipgloss.Color("#FED7AA"))
	// tagCloudStyle uses a hard-coded light blue foreground, no theme dep.

	providerActiveStyle = providerActiveStyle.Foreground(*colorAccent)
	providerSelectedStyle = providerSelectedStyle.Background(*colorBgSubtle)
	paletteSelectedStyle = paletteSelectedStyle.Background(*colorBgSubtle).Foreground(*colorText)

	keyStatusOKStyle = keyStatusOKStyle.Foreground(*colorSuccess)
	keyStatusBadStyle = keyStatusBadStyle.Foreground(*colorError)
	keyStatusNAStyle = keyStatusNAStyle.Background(*colorBgLight).Foreground(*colorMuted)
}
