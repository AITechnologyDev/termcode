package tui

import (
	"fmt"
	"os"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/NekoFemDev/termcode/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// PluginInput is the in-process plugin snapshot the TUI consumes at
// startup. May be nil.
type PluginInput struct {
	Tools       []PluginToolSpec
	Theme       *ThemeOverride
	PromptParts []string
	Names       []string
}

// PluginToolSpec — минимальное описание инструмента для регистрации в
// Executor. Совпадает по форме с tools.PluginTool, но живёт в tui,
// чтобы не тянуть plugin/host в эту зависимость.
type PluginToolSpec struct {
	Name        string
	Description string
	Params      string
	Run         func(params map[string]string) (string, error)
}

// New создаёт новую TUI модель.
// plugins may be nil — the TUI then runs without plugins.
func New(cfg *config.Config, workDir string, plugins *PluginInput) (*Model, error) {
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("os.Getwd: %w", err)
		}
		workDir = wd
	}

	pc, ok := cfg.ActiveProviderConfig()
	if !ok {
		return nil, fmt.Errorf("provider config %q not found", cfg.ActiveProvider)
	}
	provider, err := ai.New(pc, cfg.ActiveProvider)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	sess := session.New(workDir, string(cfg.ActiveProvider), pc.Model)

	ta := textarea.New()
	ta.Placeholder = i18nEN.Placeholder
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")

	vp := viewport.New(80, 20)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	editTa := textarea.New()
	editTa.ShowLineNumbers = false
	editTa.CharLimit = 0
	editTa.KeyMap.InsertNewline.SetKeys("shift+enter")

	widthPx, heightPx := 0, 0
	if ws, err := getTermSize(); err == nil {
		widthPx = int(ws.Xpixel)
		heightPx = int(ws.Ypixel)
	}

	exec := tools.New(workDir)

	m := &Model{
		cfg:            cfg,
		provider:       provider,
		sess:           sess,
		workDir:        workDir,
		executor:       exec,
		viewport:       vp,
		input:          ta,
		editInput:      editTa,
		spinner:        sp,
		currentState:   stateModelSelect,
		modelsLoading:  true,
		screenWidthPx:  widthPx,
		screenHeightPx: heightPx,
		maxRetries:     3,
		retryCount:     0,
		fullResponse:   "",
		toolsExtracted: false,
		displayedLen:   0,
	}

	// Apply plugin theme + register plugin tools.
	if plugins != nil {
		if plugins.Theme != nil {
			applyTheme(plugins.Theme)
		}
		for _, t := range plugins.Tools {
			tool := t
			if err := exec.RegisterPluginTool(tool.Name, tool.Run); err != nil {
				fmt.Fprintf(os.Stderr, "plugin tool %q: %v\n", tool.Name, err)
			}
		}
		m.pluginInput = plugins
	}

	m.paletteItems = m.buildPaletteItems()
	m.thinkExpanded = make(map[int]bool)
	m.questionSelected = make(map[int]bool)
	return m, nil
}

// Init — начальные команды при старте TUI.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textarea.Blink,
		m.spinner.Tick,
		fetchOllamaModels(m.cfg),
	}
	if m.cfg.ActiveProvider == config.ProviderOllama {
		pc, ok := m.cfg.ActiveProviderConfig()
		if ok && pc.ContextLength == 0 {
			cmds = append(cmds, fetchContextLength(pc.BaseURL, pc.Model))
		}
	}
	return tea.Batch(cmds...)
}

// pluginTools returns the slice of plugin tools for inclusion in the
// system prompt. Empty if no plugins.
func (m Model) pluginTools() []tools.PluginTool {
	if m.pluginInput == nil {
		return nil
	}
	out := []tools.PluginTool{}
	for _, t := range m.pluginInput.Tools {
		out = append(out, tools.PluginTool{
			Name:        t.Name,
			Description: t.Description,
			Params:      t.Params,
		})
	}
	return out
}

// pluginSystemPromptParts returns plugin-contributed system-prompt
// fragments in registration order. Empty if no plugins.
func (m Model) pluginSystemPromptParts() []string {
	if m.pluginInput == nil {
		return nil
	}
	return m.pluginInput.PromptParts
}
