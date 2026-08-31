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
	name        string
	tools       []host.Tool
	theme       *host.Theme
	prompts     []string
	registerErr error
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
