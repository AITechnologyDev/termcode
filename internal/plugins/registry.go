// Package plugins is the central registry for in-process TermCode plugins.
//
// To add a plugin:
//
//  1. Create internal/plugins/<name>/<name>.go with a type that
//     implements host.Plugin and a package-level init() that calls
//     register.Plug(&MyPlugin{}).
//  2. Add a blank import to the builtins block below.
//  3. Rebuild TermCode.
//
// At startup, Load() runs every plugin's Register hook and returns a
// Snapshot. The TUI consumes the Snapshot once at startup.
package plugins

import (
	"github.com/NekoFemDev/termcode/internal/plugin/register"

	// Built-in plugins. Each is a package that calls register.Plug()
	// from its init() function. To disable a plugin, remove the import
	// below (and rebuild).
	//
	// To add a new plugin:
	//   1. Create internal/plugins/<name>/<name>.go with a type that
	//      implements host.Plugin.
	//   2. Add `import _ "github.com/NekoFemDev/termcode/internal/plugins/<name>"`
	//      below.
	//   3. Rebuild TermCode.
	//
	// No IPC, no subprocess, no .so files. Each plugin compiles into
	// the same binary as TermCode itself.
	_ "github.com/NekoFemDev/termcode/internal/plugins/example"
)

// Snapshot is the result of loading all plugins. It's a thin re-export
// of register.Snapshot so callers can stay on this package's API.
type Snapshot = register.Snapshot

// Load is a thin re-export of register.Load.
func Load() (*Snapshot, error) { return register.Load() }

// Names is a thin re-export of register.Names.
func Names() []string { return register.Names() }
