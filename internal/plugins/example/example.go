// Package example is a demo plugin. It registers:
//
//   - A tool the AI can call: example_echo
//   - A slash command users can type: /example
//   - A palette item under Ctrl+P: "Example — say hi"
//   - A theme override that turns the brand color hot pink
//   - A system-prompt fragment reminding the AI to be terse
//
// To disable it: remove the import in internal/plugins/registry.go.
package example

import (
	"fmt"
	"strings"
	"time"

	"github.com/NekoFemDev/termcode/internal/plugin/host"
	"github.com/NekoFemDev/termcode/internal/plugin/register"
)

type Example struct{}

func (e *Example) Name() string    { return "example" }
func (e *Example) Version() string { return "0.2.0" }
func (e *Example) Description() string {
	return "Demo plugin: tool, slash command, palette item, theme, prompt."
}

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

	// 2. A slash command. Try it: type "/example Neko" and press Enter.
	if err := reg.RegisterSlashCommand(host.SlashCommand{
		Name:        "/example",
		Description: "Demo slash command — greets the args (or just says hi).",
		Run: func(argv []string) (string, error) {
			if len(argv) == 0 {
				return "👋 Hi from the example plugin! Try `/example <name>`.", nil
			}
			return fmt.Sprintf("👋 Hi, %s! (from /example at %s)",
				strings.Join(argv, " "),
				time.Now().Format("15:04:05"),
			), nil
		},
	}); err != nil {
		return err
	}

	// 3. A palette item. Press Ctrl+P, then type "example" to find it.
	if err := reg.RegisterPaletteItem(host.PaletteItem{
		Title:       "Example — say hi",
		Description: "Inserts a friendly greeting from the example plugin.",
		Run: func() (string, error) {
			return "👋 Hello from the example palette item! (Ctrl+P → type 'example')", nil
		},
	}); err != nil {
		return err
	}

	// 4. A theme override — turn the brand color hot pink.
	reg.SetTheme(host.Theme{
		Primary: "EC4899", // hot pink instead of lavender
	})

	// 5. A prompt fragment.
	reg.AppendSystemPrompt(`## Plugin example active
The example plugin is loaded. Keep replies under 3 sentences unless asked otherwise.
If the user pastes a long file, summarize it before responding.`)

	return nil
}

// init auto-registers the plugin when this package is imported.
func init() {
	register.Plug(&Example{})
}
