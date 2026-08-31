package register

import (
	"strings"
	"testing"

	"github.com/NekoFemDev/termcode/internal/plugin/host"
)

// reset the registry between tests
func reset() {
	registry = map[string]*entry{}
}

type fakePlugin struct {
	name          string
	tools         []host.Tool
	slashCommands []host.SlashCommand
	paletteItems  []host.PaletteItem
	theme         *host.Theme
	prompts       []string
	registerErr   error
}

func (f *fakePlugin) Name() string        { return f.name }
func (f *fakePlugin) Version() string     { return "test" }
func (f *fakePlugin) Description() string { return "" }
func (f *fakePlugin) Register(r host.Registry) error {
	if f.registerErr != nil {
		return f.registerErr
	}
	for _, t := range f.tools {
		if err := r.RegisterTool(t); err != nil {
			return err
		}
	}
	for _, sc := range f.slashCommands {
		if err := r.RegisterSlashCommand(sc); err != nil {
			return err
		}
	}
	for _, pi := range f.paletteItems {
		if err := r.RegisterPaletteItem(pi); err != nil {
			return err
		}
	}
	if f.theme != nil {
		r.SetTheme(*f.theme)
	}
	for _, p := range f.prompts {
		r.AppendSystemPrompt(p)
	}
	return nil
}

func TestLoad_NoPlugins(t *testing.T) {
	reset()
	snap, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.PluginNames) != 0 {
		t.Errorf("expected zero plugins, got %d", len(snap.PluginNames))
	}
}

func TestLoad_RegistersAndLoads(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name: "alpha",
		tools: []host.Tool{{
			Name: "alpha_echo", Description: "echo back", Run: nil,
		}},
		prompts: []string{"alpha prompt"},
	})
	Plug(&fakePlugin{
		name: "beta",
		tools: []host.Tool{{
			Name: "beta_echo", Description: "echo back", Run: nil,
		}},
		theme: &host.Theme{Primary: "FF00FF"},
	})

	snap, err := Load()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(snap.PluginNames) != 2 {
		t.Errorf("expected 2 plugins, got %d (%v)", len(snap.PluginNames), snap.PluginNames)
	}
	if len(snap.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(snap.Tools))
	}
	if snap.Theme == nil || snap.Theme.Primary != "FF00FF" {
		t.Errorf("expected theme.Primary=FF00FF, got %+v", snap.Theme)
	}
	if len(snap.PromptParts) != 1 || !strings.Contains(snap.PromptParts[0], "alpha") {
		t.Errorf("expected alpha prompt, got %v", snap.PromptParts)
	}
}

func TestLoad_RejectsDuplicateTool(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name:  "first",
		tools: []host.Tool{{Name: "shared", Run: nil}},
	})
	Plug(&fakePlugin{
		name:  "second",
		tools: []host.Tool{{Name: "shared", Run: nil}},
	})
	snap, err := Load()
	if err == nil {
		t.Fatal("expected error from duplicate tool name")
	}
	if !strings.Contains(err.Error(), "shared") {
		t.Errorf("error should mention tool name, got: %v", err)
	}
	// First plugin's tool should still be in the snapshot.
	if len(snap.Tools) != 1 {
		t.Errorf("expected 1 tool from the surviving plugin, got %d", len(snap.Tools))
	}
}

func TestPlug_PanicsOnDuplicate(t *testing.T) {
	reset()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate Plug")
		}
	}()
	Plug(&fakePlugin{name: "x"})
	Plug(&fakePlugin{name: "x"})
}

func TestLoad_SlashAndPalette(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name: "p",
		slashCommands: []host.SlashCommand{
			{Name: "/hi", Description: "say hi"},
		},
		paletteItems: []host.PaletteItem{
			{Title: "Demo", Description: "demo item"},
		},
	})
	snap, err := Load()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(snap.SlashCommands) != 1 || snap.SlashCommands[0].Name != "/hi" {
		t.Errorf("expected one slash command named /hi, got %+v", snap.SlashCommands)
	}
	if len(snap.PaletteItems) != 1 || snap.PaletteItems[0].Title != "Demo" {
		t.Errorf("expected one palette item named Demo, got %+v", snap.PaletteItems)
	}
}

func TestRegister_RejectsSlashWithoutLeadingSlash(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name: "p",
		slashCommands: []host.SlashCommand{
			{Name: "hi"}, // missing /
		},
	})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must start with '/'") {
		t.Errorf("expected error about leading slash, got: %v", err)
	}
}

func TestRegister_RejectsDuplicateSlash(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name:          "p1",
		slashCommands: []host.SlashCommand{{Name: "/dup"}},
	})
	Plug(&fakePlugin{
		name:          "p2",
		slashCommands: []host.SlashCommand{{Name: "/dup"}},
	})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "/dup") {
		t.Errorf("expected duplicate slash error, got: %v", err)
	}
}

func TestRegister_RejectsDuplicatePalette(t *testing.T) {
	reset()
	Plug(&fakePlugin{
		name:         "p1",
		paletteItems: []host.PaletteItem{{Title: "Same"}},
	})
	Plug(&fakePlugin{
		name:         "p2",
		paletteItems: []host.PaletteItem{{Title: "Same"}},
	})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "Same") {
		t.Errorf("expected duplicate palette error, got: %v", err)
	}
}
