// Package host defines the contract for in-process TermCode plugins.
//
// A plugin is a Go file that:
//
//  1. Defines a type implementing the Plugin interface
//  2. Registers it in internal/plugins/registry.go
//
// At startup, TermCode calls Register on every registered plugin. The
// plugin can return:
//
//   - Tools the AI can call (same wire format as built-in tools)
//   - Slash commands users type into chat (e.g. /hello)
//   - Palette items shown in Ctrl+P
//   - A Theme that overrides the default color palette
//   - SystemPromptFragments appended to the AI's system prompt
//
// No IPC, no subprocess, no Go plugins. Adding a plugin requires a single
// `go build` and a TermCode restart.
package host

// Plugin — every plugin implements this. Register is called once at
// startup. The plugin uses reg to push things into TermCode.
type Plugin interface {
	// Name — short identifier used in logs.
	Name() string
	// Version — free-form string.
	Version() string
	// Description — one-line human-readable description.
	Description() string
	// Register — called at startup.
	Register(reg Registry) error
}

// Registry is what TermCode exposes to plugins. All registration is
// one-way: plugins push things in, the host never calls back.
type Registry interface {
	// RegisterTool adds a tool the AI can call. Name must be unique
	// across all plugins and built-in tools.
	RegisterTool(t Tool) error
	// RegisterSlashCommand adds a `/name` command users can type into
	// the chat input. Name must start with "/" and be unique.
	RegisterSlashCommand(c SlashCommand) error
	// RegisterPaletteItem adds an entry to the Ctrl+P command palette.
	// Title must be unique.
	RegisterPaletteItem(p PaletteItem) error
	// SetTheme overrides the default color palette. Pass only the
	// colors you want to change; unset fields keep their defaults.
	SetTheme(Theme)
	// AppendSystemPrompt adds a fragment to the AI's system prompt.
	AppendSystemPrompt(fragment string)
}

// Tool — a callable the AI can invoke.
type Tool struct {
	Name        string
	Description string
	// Params — JSON-schema-ish description shown to the AI verbatim.
	Params string
	// Run is called when the AI invokes the tool. Return the result
	// string and/or an error. The error is reported to the AI as a
	// tool failure.
	Run func(params map[string]string) (string, error)
}

// SlashCommand — a `/name` command typed into the chat input. The Run
// callback receives everything after the command name, split on
// whitespace. Return the string to show in chat and/or an error.
type SlashCommand struct {
	// Name — must start with "/", e.g. "/hello".
	Name string
	// Description — shown in the help footer and the command palette.
	Description string
	// Run — argv is what came after the command name, already split
	// on whitespace. e.g. "/hello world" → argv=["world"].
	Run func(argv []string) (string, error)
}

// PaletteItem — a Ctrl+P entry. The Run callback returns the text to
// append to the chat and/or an error.
type PaletteItem struct {
	// Title — unique identifier, shown as the item name in the palette.
	Title string
	// Description — secondary text in the palette row.
	Description string
	// Run — invoked when the user selects this item. Return the text
	// to show in the chat and/or an error.
	Run func() (string, error)
}

// Theme — color palette override. Empty fields mean "keep default".
// All values are 6-digit hex strings like "A78BFA" or lipgloss color
// names like "red", "231" (256-color), etc.
type Theme struct {
	// Brand colors
	Primary   string // accent (header bg, links)
	Secondary string // cyan-ish (info pills, model name)
	Accent    string // pink (lang pill, active marker)
	Success   string // green (OK status, "key set" tag)
	Warning   string // yellow (busy status, "free tier" tag)
	Error     string // red (errors, "no key" tag)

	// Surfaces
	Bg       string // deepest background
	BgLight  string // status bar, secondary pills
	BgSubtle string // selection background, code blocks
	Border   string // dividers
	Text     string // primary text color
	Muted    string // secondary text, hints
	Link     string // hyperlinks
}

// SystemPromptFragment is what AppendSystemPrompt takes — it's just a
// string, but we alias the type for documentation.
type SystemPromptFragment = string
