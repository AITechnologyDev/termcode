// Package register is the in-process plugin registration API used by
// plugin authors. Plugins import this package, define a type that
// implements host.Plugin, and call register.Plug(&MyPlugin{}) from
// their init() function. The registry collects those Plug() calls and
// the plugins package loads them at startup.
//
// This is a separate package from `plugins` to avoid an import cycle:
// plugin code lives under internal/plugins/<name>/, and it imports
// this package; the `plugins` package imports each plugin via a blank
// import to trigger their init() functions.
package register

import (
	"fmt"
	"sort"

	"github.com/NekoFemDev/termcode/internal/plugin/host"
)

// entry is one registered plugin.
type entry struct {
	name string
	p    host.Plugin
}

var registry = map[string]*entry{}

// Plug makes a plugin visible to Load(). Called from each plugin
// package's init() function.
func Plug(p host.Plugin) {
	if p == nil {
		panic("register.Plug: nil plugin")
	}
	name := p.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("register.Plug: duplicate plugin name %q", name))
	}
	registry[name] = &entry{name: name, p: p}
}

// Snapshot is a stable view of all registered plugins.
type Snapshot struct {
	Tools       []host.Tool
	Theme       *host.Theme
	PromptParts []string
	PluginNames []string
}

// Load runs every registered plugin's Register hook against an internal
// registry and returns a snapshot. Plugins that fail Register are
// skipped; a non-nil error reports the first failure.
func Load() (*Snapshot, error) {
	reg := newRegistry()

	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)

	var firstErr error
	for _, n := range names {
		plug := registry[n].p
		reg.names = append(reg.names, plug.Name())
		if err := plug.Register(reg); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("plugin %q: %w", plug.Name(), err)
			}
		}
	}
	return reg.snapshot(), firstErr
}

// Names returns the registered plugin names in deterministic order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ── registry implementation of host.Registry ────────────────────────────

type registryImpl struct {
	tools       []host.Tool
	toolsByName map[string]int
	theme       *host.Theme
	prompts     []string
	names       []string
}

func newRegistry() *registryImpl {
	return &registryImpl{toolsByName: map[string]int{}}
}

func (r *registryImpl) RegisterTool(t host.Tool) error {
	if _, dup := r.toolsByName[t.Name]; dup {
		return fmt.Errorf("duplicate tool name %q", t.Name)
	}
	r.toolsByName[t.Name] = len(r.tools)
	r.tools = append(r.tools, t)
	return nil
}

func (r *registryImpl) SetTheme(th host.Theme) {
	r.theme = &th
}

func (r *registryImpl) AppendSystemPrompt(fragment string) {
	if fragment == "" {
		return
	}
	r.prompts = append(r.prompts, fragment)
}

func (r *registryImpl) snapshot() *Snapshot {
	return &Snapshot{
		Tools:       r.tools,
		Theme:       r.theme,
		PromptParts: r.prompts,
		PluginNames: append([]string(nil), r.names...),
	}
}
