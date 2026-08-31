// Package luaplugin loads user plugins written in Lua from
// ~/.config/termcode/plugins/*.lua (and any directory in
// $TERMCODE_PLUGIN_PATH).
//
// Each plugin is a regular Lua file that calls functions on the
// `termcode` global to register tools, slash commands, palette items,
// a theme override, and system-prompt fragments:
//
//	-- ~/.config/termcode/plugins/hello.lua
//	termcode.register_tool("hello_greet", "Greet by name",
//	    "name (string, required)", function(params)
//	        return "Hello, " .. (params.name or "world") .. "!"
//	    end)
//
//	termcode.register_command("/hello", "Say hi", function(argv)
//	    return "Hi from Lua!"
//	end)
//
//	termcode.set_theme({ primary = "EC4899" })
//	termcode.append_prompt("## hello active\nKeep it short.")
//
// Plugins run inside an embedded Lua VM (gopher-lua), so they execute
// in-process. No IPC, no .so files, no Termux workaround, and no
// rebuild of TermCode to add a plugin.
package luaplugin

import lua "github.com/yuin/gopher-lua"

// Snapshot is the result of loading every plugin file.
type Snapshot struct {
	Tools         []Tool
	SlashCommands []SlashCommand
	PaletteItems  []PaletteItem
	Theme         *Theme
	PromptParts   []string
	PluginNames   []string
	SourceDirs    []string
	Errors        []string // one entry per file that failed to load

	// vms keeps every Lua VM alive for the lifetime of the snapshot.
	// Plugin callbacks (tool/command/palette Run funcs) reference
	// their VM by pointer, so the VM must not be closed while the
	// snapshot is in use. Call Close() to release.
	vms []*lua.LState
}

// Close releases every Lua VM kept alive by the snapshot. After
// Close, calling any Run func on a registered tool/command/palette
// will panic. The TUI should call Close on shutdown.
func (s *Snapshot) Close() {
	for _, L := range s.vms {
		L.Close()
	}
	s.vms = nil
}

// Tool — a callable the AI can invoke.
type Tool struct {
	Name        string
	Description string
	Params      string
	Run         func(params map[string]string) (string, error)
}

// SlashCommand — a `/name` command.
type SlashCommand struct {
	Name        string
	Description string
	Run         func(argv []string) (string, error)
}

// PaletteItem — a Ctrl+P entry.
type PaletteItem struct {
	Title       string
	Description string
	Run         func() (string, error)
}

// Theme — color palette override. Empty fields mean "keep default".
type Theme struct {
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
