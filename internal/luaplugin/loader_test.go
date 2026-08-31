package luaplugin

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlugin(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_NoDir(t *testing.T) {
	snap, err := Load("/nonexistent/path/that/should/never/exist")
	if err != nil {
		t.Fatalf("Load should not error on missing dir, got: %v", err)
	}
	if len(snap.PluginNames) != 0 {
		t.Errorf("expected no plugins, got %d", len(snap.PluginNames))
	}
}

func TestLoad_RegistersAll(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "hello", `
termcode.register_tool("hello_tool", "desc", "params", function(p) return "ok" end)
termcode.register_command("/hi", "say hi", function(argv) return "hi" end)
termcode.register_palette("Hello", "greeting", function() return "hi!" end)
termcode.set_theme({ primary = "FF00FF" })
termcode.append_prompt("hello fragment")
`)

	snap, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.PluginNames) != 1 || snap.PluginNames[0] != "hello" {
		t.Errorf("expected plugin name 'hello', got %v", snap.PluginNames)
	}
	if len(snap.Tools) != 1 || snap.Tools[0].Name != "hello_tool" {
		t.Errorf("expected one tool 'hello_tool', got %+v", snap.Tools)
	}
	if len(snap.SlashCommands) != 1 || snap.SlashCommands[0].Name != "/hi" {
		t.Errorf("expected one slash command '/hi', got %+v", snap.SlashCommands)
	}
	if len(snap.PaletteItems) != 1 || snap.PaletteItems[0].Title != "Hello" {
		t.Errorf("expected one palette item 'Hello', got %+v", snap.PaletteItems)
	}
	if snap.Theme == nil || snap.Theme.Primary != "FF00FF" {
		t.Errorf("expected theme.Primary=FF00FF, got %+v", snap.Theme)
	}
	if len(snap.PromptParts) != 1 || snap.PromptParts[0] != "hello fragment" {
		t.Errorf("expected one prompt fragment 'hello fragment', got %+v", snap.PromptParts)
	}
}

func TestToolRun(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "echo", `
termcode.register_tool("echo", "echo", "text (string)", function(p)
    return "got:" .. (p.text or "")
end)
`)
	snap, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(snap.Tools))
	}
	out, err := snap.Tools[0].Run(map[string]string{"text": "hello"})
	if err != nil {
		t.Fatalf("tool Run: %v", err)
	}
	if out != "got:hello" {
		t.Errorf("expected 'got:hello', got %q", out)
	}
}

func TestToolRun_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "bad", `
termcode.register_tool("bad", "always fails", "", function(p) return nil, "nope" end)
`)
	snap, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = snap.Tools[0].Run(nil)
	if err == nil || err.Error() != "nope" {
		t.Errorf("expected error 'nope', got %v", err)
	}
}

func TestSlashRun(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "greet", `
termcode.register_command("/greet", "greet", function(argv) return "hi " .. (argv or "") end)
`)
	snap, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := snap.SlashCommands[0].Run([]string{"Neko"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi Neko" {
		t.Errorf("expected 'hi Neko', got %q", out)
	}
}

func TestPaletteRun(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "demo", `
termcode.register_palette("Demo", "demo", function() return "clicked!" end)
`)
	snap, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := snap.PaletteItems[0].Run()
	if err != nil {
		t.Fatal(err)
	}
	if out != "clicked!" {
		t.Errorf("expected 'clicked!', got %q", out)
	}
}

func TestLoad_CollectsErrors(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "broken", `this is not valid lua at all`)
	writePlugin(t, dir, "ok", `termcode.register_tool("ok", "", "", function(p) return "" end)`)

	snap, err := Load(dir)
	if err != nil {
		t.Fatalf("Load should not error on a broken file, got: %v", err)
	}
	if len(snap.Errors) == 0 {
		t.Errorf("expected at least one error from broken plugin")
	}
	if len(snap.Tools) != 1 {
		t.Errorf("expected the OK plugin to still load (1 tool), got %d", len(snap.Tools))
	}
}

func TestExtraDirsFromEnv(t *testing.T) {
	t.Setenv("TERMCODE_PLUGIN_PATH", "/a:/b:/c")
	got := ExtraDirsFromEnv()
	want := []string{"/a", "/b", "/c"}
	if len(got) != 3 {
		t.Fatalf("expected 3 dirs, got %d (%v)", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] expected %q, got %q", i, want[i], got[i])
		}
	}
}
