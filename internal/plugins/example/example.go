// Package example is a demo plugin. It registers:
//
//   - A tool the AI can call: example_echo
//   - A theme override that turns the brand color hot pink
//   - A system-prompt fragment reminding the AI to be terse
//
// To enable it: add an import of this package to internal/plugins/registry.go.
package example

import (
	"fmt"
	"strings"

	"github.com/NekoFemDev/termcode/internal/plugin/host"
	"github.com/NekoFemDev/termcode/internal/plugin/register"
)

type Example struct{}

func (e *Example) Name() string        { return "example" }
func (e *Example) Version() string     { return "0.1.0" }
func (e *Example) Description() string { return "Demo plugin: tool, theme override, prompt fragment." }

func (e *Example) Register(reg host.Registry) error {
	// 1. A tool the AI can call.
	if err := reg.RegisterTool(host.Tool{
		Name:        "example_echo",
		Description: "Echo the input back to the AI. Useful for testing plugin wiring.",
		Params:      `text (string, required) — the text to echo back`,
		Run: func(params map[string]string) (string, error) {
			text := strings.TrimSpace(params["text"])
			if text == "" {
				return "", fmt.Errorf("missing 'text' parameter")
			}
			return text, nil
		},
	}); err != nil {
		return err
	}

	// 2. A theme override — turn the brand color hot pink.
	reg.SetTheme(host.Theme{
		Primary: "EC4899", // hot pink instead of lavender
	})

	// 3. A prompt fragment.
	reg.AppendSystemPrompt(`## Plugin example active
The example plugin is loaded. Keep replies under 3 sentences unless asked otherwise.
If the user pastes a long file, summarize it before responding.`)

	return nil
}

// init auto-registers the plugin when this package is imported.
func init() {
	register.Plug(&Example{})
}
