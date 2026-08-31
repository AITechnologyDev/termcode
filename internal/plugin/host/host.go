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
//   - a list of Tools the AI can call (same wire format as built-in tools)
//   - a Theme that overrides the default color palette
//   - a SystemPromptFragment appended to the AI's system prompt
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
