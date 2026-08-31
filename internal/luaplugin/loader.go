package luaplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// DefaultDir — ~/.config/termcode/plugins.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "termcode", "plugins"), nil
}

// ExtraDirsFromEnv — any directories in $TERMCODE_PLUGIN_PATH, colon-separated.
func ExtraDirsFromEnv() []string {
	v := os.Getenv("TERMCODE_PLUGIN_PATH")
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Load scans every .lua file in dirs (in order), executes each one in
// its own fresh Lua VM with a `termcode` global exposing the
// registration API, and returns a Snapshot of everything that was
// registered. Errors loading individual files are collected in
// snapshot.Errors and loading continues for the rest.
//
// Built-in Lua modules the plugin can require: only `string` for now —
// we deliberately don't expose `os` or `io` to keep plugins sandboxed.
func Load(dirs ...string) (*Snapshot, error) {
	snap := &Snapshot{SourceDirs: append([]string(nil), dirs...)}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			snap.Errors = append(snap.Errors, fmt.Sprintf("%s: %v", dir, err))
			continue
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".lua") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
		sort.Strings(files)
		for _, f := range files {
			if err := loadOne(f, snap); err != nil {
				snap.Errors = append(snap.Errors, fmt.Sprintf("%s: %v", f, err))
			}
		}
	}
	return snap, nil
}

// loadOne runs a single Lua file in a fresh VM, collecting
// registrations into the shared accumulator.
func loadOne(path string, snap *Snapshot) error {
	L := lua.NewState(lua.Options{
		IncludeGoStackTrace: true,
	})
	snap.vms = append(snap.vms, L)

	registerHostAPI(L, snap)

	if err := L.DoFile(path); err != nil {
		return fmt.Errorf("do file: %w", err)
	}

	// Snapshot the plugin name from the file basename.
	snap.PluginNames = append(snap.PluginNames, strings.TrimSuffix(filepath.Base(path), ".lua"))
	return nil
}

// registerHostAPI installs the `termcode` global on the Lua state with
// the registration methods. Each method pushes into the shared
// accumulator `snap` so the loader can collect results.
func registerHostAPI(L *lua.LState, snap *Snapshot) {
	mod := L.NewTable()

	// termcode.register_tool(name, description, params, fn)
	//   fn is a Lua function: takes a table of params, returns a
	//   string result (or nil, error_message).
	L.SetField(mod, "register_tool", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		desc := L.CheckString(2)
		params := L.CheckString(3)
		fn := L.CheckFunction(4)
		tool := Tool{
			Name:        name,
			Description: desc,
			Params:      params,
			Run: func(p map[string]string) (string, error) {
				out, err := callToolFn(L, fn, p)
				return out, err
			},
		}
		snap.Tools = append(snap.Tools, tool)
		return 0
	}))

	// termcode.register_command(name, description, fn)
	//   name must start with "/". fn(argv) -> string.
	L.SetField(mod, "register_command", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		desc := L.CheckString(2)
		fn := L.CheckFunction(3)
		if !strings.HasPrefix(name, "/") {
			L.RaiseError("slash command name %q must start with '/'", name)
			return 0
		}
		snap.SlashCommands = append(snap.SlashCommands, SlashCommand{
			Name:        name,
			Description: desc,
			Run: func(argv []string) (string, error) {
				return callStringFn(L, fn, lua.LString(strings.Join(argv, " ")))
			},
		})
		return 0
	}))

	// termcode.register_palette(title, description, fn)
	//   fn() -> string.
	L.SetField(mod, "register_palette", L.NewFunction(func(L *lua.LState) int {
		title := L.CheckString(1)
		desc := L.CheckString(2)
		fn := L.CheckFunction(3)
		snap.PaletteItems = append(snap.PaletteItems, PaletteItem{
			Title:       title,
			Description: desc,
			Run: func() (string, error) {
				return callStringFn(L, fn)
			},
		})
		return 0
	}))

	// termcode.set_theme({ primary = "...", accent = "...", ... })
	L.SetField(mod, "set_theme", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		th := &Theme{}
		th.Primary = stringField(L, tbl, "primary")
		th.Secondary = stringField(L, tbl, "secondary")
		th.Accent = stringField(L, tbl, "accent")
		th.Success = stringField(L, tbl, "success")
		th.Warning = stringField(L, tbl, "warning")
		th.Error = stringField(L, tbl, "error")
		th.Muted = stringField(L, tbl, "muted")
		th.Bg = stringField(L, tbl, "bg")
		th.BgLight = stringField(L, tbl, "bg_light")
		th.BgSubtle = stringField(L, tbl, "bg_subtle")
		th.Border = stringField(L, tbl, "border")
		th.Text = stringField(L, tbl, "text")
		th.Link = stringField(L, tbl, "link")
		snap.Theme = th
		return 0
	}))

	// termcode.append_prompt(fragment)
	L.SetField(mod, "append_prompt", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		snap.PromptParts = append(snap.PromptParts, s)
		return 0
	}))

	// termcode.log(msg) — for plugin debugging. Prints to stderr.
	L.SetField(mod, "log", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		fmt.Fprintln(os.Stderr, "[plugin] "+s)
		return 0
	}))

	// Expose as global.
	L.SetGlobal("termcode", mod)
}

// stringField reads a string field from a Lua table, returning "" if
// the field is missing or not a string.
func stringField(L *lua.LState, tbl *lua.LTable, key string) string {
	v := tbl.RawGetString(key)
	if s, ok := v.(lua.LString); ok {
		return string(s)
	}
	return ""
}

// callToolFn invokes a Lua function that takes a table of params and
// returns (string, nil) or (string, err_string).
func callToolFn(L *lua.LState, fn *lua.LFunction, params map[string]string) (string, error) {
	tbl := L.NewTable()
	for k, v := range params {
		tbl.RawSetString(k, lua.LString(v))
	}
	if err := L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    2,
		Protect: true,
	}, tbl); err != nil {
		return "", fmt.Errorf("plugin tool: %w", err)
	}
	out, errStr := twoStringReturns(L)
	if errStr != "" {
		return out, fmt.Errorf("%s", errStr)
	}
	return out, nil
}

// callStringFn invokes a Lua function that takes a single string
// argument and returns (string, nil) or (string, err_string).
func callStringFn(L *lua.LState, fn *lua.LFunction, extraArgs ...lua.LValue) (string, error) {
	args := make([]lua.LValue, 0, 1+len(extraArgs))
	args = append(args, extraArgs...)
	if err := L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    2,
		Protect: true,
	}, args...); err != nil {
		return "", fmt.Errorf("plugin callback: %w", err)
	}
	out, errStr := twoStringReturns(L)
	if errStr != "" {
		return out, fmt.Errorf("%s", errStr)
	}
	return out, nil
}

// twoStringReturns pops the top two values from the Lua stack and
// returns them as Go strings. Non-string values become empty.
func twoStringReturns(L *lua.LState) (string, string) {
	errVal := L.Get(-1)
	L.Pop(1)
	outVal := L.Get(-1)
	L.Pop(1)
	out, _ := outVal.(lua.LString)
	errStr, _ := errVal.(lua.LString)
	return string(out), string(errStr)
}
