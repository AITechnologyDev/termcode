package tui

import (
	"github.com/NekoFemDev/termcode/internal/session"
)

// PluginInput is the in-process plugin snapshot the TUI consumes at
// startup. May be nil.
type PluginInput struct {
	Tools         []PluginToolSpec
	SlashCommands []PluginSlashSpec
	PaletteItems  []PluginPaletteSpec
	Theme         *ThemeOverride
	PromptParts   []string
	Names         []string
}

// PluginToolSpec — описание инструмента для регистрации в Executor.
type PluginToolSpec struct {
	Name        string
	Description string
	Params      string
	Run         func(params map[string]string) (string, error)
}

// PluginSlashSpec — описание slash-команды.
type PluginSlashSpec struct {
	Name        string
	Description string
	Run         func(argv []string) (string, error)
}

// PluginPaletteSpec — описание элемента Ctrl+P палитры.
type PluginPaletteSpec struct {
	Title       string
	Description string
	Run         func() (string, error)
}

// runPluginSlash запускает slash-команду плагина и применяет побочный
// эффект к чату. Возвращает обновлённую модель, текст ошибки (если есть)
// и флаг "была ли обработана команда".
func (m Model) runPluginSlash(text string) (Model, bool, string) {
	if m.pluginInput == nil {
		return m, false, ""
	}
	for _, sc := range m.pluginInput.SlashCommands {
		if text == sc.Name || (len(text) > len(sc.Name) && text[:len(sc.Name)+1] == sc.Name+" ") {
			argv := []string{}
			rest := text[len(sc.Name):]
			rest = trimLeadingSpace(rest)
			if rest != "" {
				argv = fields(rest)
			}
			out, err := sc.Run(argv)
			if err != nil {
				return m, true, "plugin error: " + err.Error()
			}
			if out != "" {
				m.sess.AddMessage(session.RoleAssistant, out)
				m.refreshViewport()
				m.scrollToBottom = true
			}
			return m, true, ""
		}
	}
	return m, false, ""
}

// runPluginPalette запускает plugin-элемент палитры.
func (m Model) runPluginPalette(title string) (Model, string) {
	if m.pluginInput == nil {
		return m, "no plugins"
	}
	for _, p := range m.pluginInput.PaletteItems {
		if p.Title == title {
			out, err := p.Run()
			if err != nil {
				return m, "plugin error: " + err.Error()
			}
			if out != "" {
				m.sess.AddMessage(session.RoleAssistant, out)
				m.refreshViewport()
				m.scrollToBottom = true
			}
			return m, ""
		}
	}
	return m, "plugin item not found"
}

// ── tiny string helpers (avoid extra imports in new.go) ────────────────

func trimLeadingSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

func fields(s string) []string {
	out := []string{}
	cur := ""
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if inWord {
				out = append(out, cur)
				cur = ""
				inWord = false
			}
			continue
		}
		cur += string(r)
		inWord = true
	}
	if inWord {
		out = append(out, cur)
	}
	return out
}
