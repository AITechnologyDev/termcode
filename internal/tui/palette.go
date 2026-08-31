package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// buildPaletteItems — список команд с локализованными названиями.
func (m Model) buildPaletteItems() []paletteItem {
	t := m.tr()
	items := []paletteItem{
		{
			key: "Ctrl+P", title: t.PalCmdPalette,
			description: t.PalCmdPaletteDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = statePalette
				m.paletteCursor = 0
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "/models", title: t.PalModels,
			description: t.PalModelsDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateModelSelect
				m.modelsLoading = true
				m.paletteFilter = ""
				return m, fetchOllamaModels(m.cfg)
			},
		},
		{
			key: "/pull", title: t.PalPull,
			description: t.PalPullDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.input.SetValue("/pull ")
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "new", title: t.PalNew,
			description: t.PalNewDesc,
			action: func(m Model) (Model, tea.Cmd) {
				_ = m.sess.Save()
				pc, _ := m.cfg.ActiveProviderConfig()
				m.sess = session.New(m.workDir, string(m.cfg.ActiveProvider), pc.Model)
				m.streaming = ""
				m.fullResponse = ""
				m.toolsExtracted = false
				m.displayedLen = 0
				m.errMsg = ""
				m.currentState = stateChat
				m.paletteFilter = ""
				m.refreshViewport()
				return m, nil
			},
		},
		{
			key: "lang", title: t.PalLang,
			description: t.PalLangDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.paletteFilter = ""
				m.currentState = stateChat
				if m.cfg.Language == "ru" {
					m.cfg.Language = "en"
				} else {
					m.cfg.Language = "ru"
				}
				_ = m.cfg.Save()
				m.paletteItems = m.buildPaletteItems()
				return m, nil
			},
		},
		{
			key: "provider", title: t.PalProvider,
			description: t.PalProviderDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.paletteFilter = ""
				m.currentState = stateProviderSelect
				m.providerCursor = 0
				m.providerEditMode = 0
				metas := config.ProvidersMeta()
				for i, p := range metas {
					if p.ID == m.cfg.ActiveProvider {
						m.providerCursor = i
						break
					}
				}
				return m, nil
			},
		},
		{
			key: "sessions", title: t.PalSessions,
			description: t.PalSessionsDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateSessionLoad
				m.sessionsLoading = true
				m.paletteFilter = ""
				return m, loadSessions()
			},
		},
		{
			key: "Ctrl+S", title: t.PalSave,
			description: t.PalSaveDesc,
			action: func(m Model) (Model, tea.Cmd) {
				if err := m.sess.Save(); err != nil {
					m.errMsg = "Save error: " + err.Error()
				}
				m.currentState = stateChat
				m.paletteFilter = ""
				return m, nil
			},
		},
		{
			key: "ls", title: t.PalLS,
			description: t.PalLSDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				result := m.executor.ListFiles("")
				content := result.Output
				if !result.OK {
					content = "Error: " + result.Error
				}
				m.sess.AddMessage(session.RoleAssistant, "```\n"+content+"\n```")
				m.refreshViewport()
				m.scrollToBottom = true
				return m, nil
			},
		},
		{
			key: "git", title: t.PalGit,
			description: t.PalGitDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				result := m.executor.RunCommand("git status --short 2>&1 || echo '(not a git repo)'")
				m.sess.AddMessage(session.RoleAssistant, "```\n"+result.Output+"\n```")
				m.refreshViewport()
				m.scrollToBottom = true
				return m, nil
			},
		},
		{
			key: "build", title: t.PalBuild,
			description: t.PalBuildDesc,
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
			key: "test", title: t.PalTest,
			description: t.PalTestDesc,
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
			key: "ctx", title: t.PalCtx,
			description: t.PalCtxDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				pct := 0
				if m.contextLimit > 0 {
					pct = m.contextUsed * 100 / m.contextLimit
				}
				info := fmt.Sprintf(
					"**Context:** %s / %s tokens (%d%%)\n**Messages:** %d\n**Model:** %s",
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
			key: "clear", title: t.PalClear,
			description: t.PalClearDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateChat
				m.paletteFilter = ""
				m.viewport.SetContent("")
				return m, nil
			},
		},
		{
			key: "profile", title: t.PalProfile,
			description: t.PalProfileDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateProfileEdit
				m.editMode = 0
				m.paletteFilter = ""
				m.editInput = newEditTextarea(m.cfg.UserProfile, m.width)
				return m, textarea.Blink
			},
		},
		{
			key: "instruct", title: t.PalInstructions,
			description: t.PalInstructionsDesc,
			action: func(m Model) (Model, tea.Cmd) {
				m.currentState = stateInstructEdit
				m.editMode = 1
				m.paletteFilter = ""
				m.editInput = newEditTextarea(m.cfg.AIInstructions, m.width)
				return m, textarea.Blink
			},
		},
	}

	// Plugin palette items. Each plugin-contributed entry becomes a
	// selectable item that runs the plugin's callback.
	if m.pluginInput != nil {
		for _, pi := range m.pluginInput.PaletteItems {
			entry := pi
			items = append(items, paletteItem{
				key:         "plugin",
				title:       entry.Title,
				description: entry.Description,
				action: func(m Model) (Model, tea.Cmd) {
					m.currentState = stateChat
					m.paletteFilter = ""
					m2, errMsg := m.runPluginPalette(entry.Title)
					if errMsg != "" {
						m2.errMsg = errMsg
					}
					return m2, nil
				},
			})
		}
	}

	return items
}

// filterPaletteItems — поиск по title/key/description.
func filterPaletteItems(items []paletteItem, filter string) []paletteItem {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var out []paletteItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.title), f) ||
			strings.Contains(strings.ToLower(item.key), f) ||
			strings.Contains(strings.ToLower(item.description), f) {
			out = append(out, item)
		}
	}
	return out
}

func (m Model) executePaletteItem(item paletteItem) (tea.Model, tea.Cmd) {
	return item.action(m)
}

// renderPalette — оверлей с поиском и списком команд.
func (m Model) renderPalette() string {
	w := m.width - 8
	if w < 30 {
		w = 30
	}

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(m.tr().PaletteTitle) + "\n")

	// Поиск
	searchPrompt := inputPromptStyle.Render("  ")
	searchVal := m.paletteFilter
	if searchVal == "" {
		searchVal = keyHintStyle.Render(m.tr().PaletteSearch)
	}
	sb.WriteString(inputContainerFocusStyle.Width(w).Render(searchPrompt+searchVal) + "\n\n")

	// Список
	filtered := filterPaletteItems(m.paletteItems, m.paletteFilter)
	if len(filtered) == 0 {
		sb.WriteString(keyHintStyle.Render(m.tr().PaletteEmpty) + "\n")
	}
	for i, item := range filtered {
		keyPart := keyStyle.Render(fmt.Sprintf("%-10s", item.key))
		titlePart := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(item.title)
		descPart := keyHintStyle.Render("  " + item.description)
		line := fmt.Sprintf("%s  %s%s", keyPart, titlePart, descPart)
		if i == m.paletteCursor {
			line = paletteSelectedStyle.Width(w).Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(keyHintStyle.Render(m.tr().PaletteHint))
	return sb.String()
}
