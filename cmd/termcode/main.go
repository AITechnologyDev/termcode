package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/NekoFemDev/termcode/internal/plugins"
	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/NekoFemDev/termcode/internal/tui"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	root := buildRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	var (
		flagProvider string
		flagModel    string
		flagWorkDir  string
	)

	root := &cobra.Command{
		Use:   "termcode [workdir]",
		Short: "TermCode — AI coding assistant for the terminal",
		Long: `TermCode is an AI coding assistant that runs right in your terminal.
Works with Ollama (local + cloud), OpenAI, Anthropic, OpenRouter.
Reads, writes, and patches project files on request.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := flagWorkDir
			if len(args) > 0 {
				workDir = args[0]
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			if flagProvider != "" {
				cfg.ActiveProvider = config.Provider(flagProvider)
			}
			if flagModel != "" {
				pc := cfg.Providers[cfg.ActiveProvider]
				pc.Model = flagModel
				cfg.Providers[cfg.ActiveProvider] = pc
			}

			m, err := tui.New(cfg, workDir, loadPlugins())
			if err != nil {
				return fmt.Errorf("init TUI: %w", err)
			}

			return tui.Start(m)
		},
	}

	root.Flags().StringVarP(&flagProvider, "provider", "p", "",
		"AI provider: ollama, openai, anthropic, openrouter")
	root.Flags().StringVarP(&flagModel, "model", "m", "",
		"Model (overrides config)")
	root.Flags().StringVarP(&flagWorkDir, "dir", "d", "",
		"Project working directory")

	// Подкоманды
	root.AddCommand(
		buildConfigCmd(),
		buildSessionsCmd(),
		buildPluginCmd(),
		buildVersionCmd(),
	)

	return root
}

// loadPlugins calls the in-process plugin registry and converts the
// result into a tui.PluginInput. Returns nil if no plugins are loaded.
func loadPlugins() *tui.PluginInput {
	snap, err := plugins.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin load: %v\n", err)
	}
	if snap == nil {
		return nil
	}

	in := &tui.PluginInput{
		Names:       snap.PluginNames,
		PromptParts: snap.PromptParts,
	}
	for _, t := range snap.Tools {
		tool := t
		in.Tools = append(in.Tools, tui.PluginToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			Params:      tool.Params,
			Run:         tool.Run,
		})
	}
	if snap.Theme != nil {
		th := &tui.ThemeOverride{}
		th.Primary = snap.Theme.Primary
		th.Secondary = snap.Theme.Secondary
		th.Accent = snap.Theme.Accent
		th.Success = snap.Theme.Success
		th.Warning = snap.Theme.Warning
		th.Error = snap.Theme.Error
		th.Muted = snap.Theme.Muted
		th.Bg = snap.Theme.Bg
		th.BgLight = snap.Theme.BgLight
		th.BgSubtle = snap.Theme.BgSubtle
		th.Border = snap.Theme.Border
		th.Text = snap.Theme.Text
		th.Link = snap.Theme.Link
		in.Theme = th
	}
	return in
}

// ── termcode plugin ──────────────────────────────────────────────────────────

func buildPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage in-process plugins",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List loaded plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := plugins.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
			}
			if len(snap.PluginNames) == 0 {
				fmt.Println("No plugins loaded.")
				fmt.Println("To add one: create internal/plugins/<name>/<name>.go and")
				fmt.Println("import it from internal/plugins/registry.go, then rebuild.")
				return nil
			}
			fmt.Printf("%d plugin(s) loaded, %d tool(s) registered.\n",
				len(snap.PluginNames), len(snap.Tools))
			for _, name := range snap.PluginNames {
				fmt.Println("● " + name)
			}
			if snap.Theme != nil {
				fmt.Println("Theme override: active")
			}
			if len(snap.PromptParts) > 0 {
				fmt.Printf("System-prompt fragments: %d\n", len(snap.PromptParts))
			}
			return nil
		},
	})
	return cmd
}

// ── termcode config ───────────────────────────────────────────────────────────

func buildConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			dir, _ := config.ConfigDir()
			fmt.Printf("Config: %s/config.json\n\n", dir)
			fmt.Printf("Active provider: %s\n", cfg.ActiveProvider)
			fmt.Printf("Providers:\n")
			for _, pm := range config.ProvidersMeta() {
				pc, ok := cfg.Providers[pm.ID]
				if !ok {
					continue
				}
				key := pc.APIKey
				if key != "" {
					if len(key) > 8 {
						key = key[:4] + "..." + key[len(key)-4:]
					}
				} else {
					key = "(not set)"
				}
				active := ""
				if pm.ID == cfg.ActiveProvider {
					active = " ◀ active"
				}
				fmt.Printf("  %-12s  model: %-32s  url: %s  key: %s%s\n",
					pm.ID, pc.Model, pc.BaseURL, key, active)
			}
			return nil
		},
	})

	var apiKey, model, baseURL string
	setCmd := &cobra.Command{
		Use:   "set-provider <provider>",
		Short: "Set active provider and its parameters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			provider := config.Provider(args[0])
			cfg.ActiveProvider = provider

			pc := cfg.Providers[provider]
			if apiKey != "" {
				pc.APIKey = apiKey
			}
			if model != "" {
				pc.Model = model
			}
			if baseURL != "" {
				pc.BaseURL = baseURL
			}
			cfg.Providers[provider] = pc

			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Saved: active provider = %s, model = %s\n", provider, pc.Model)
			return nil
		},
	}
	setCmd.Flags().StringVar(&apiKey, "key", "", "API key")
	setCmd.Flags().StringVar(&model, "model", "", "Model")
	setCmd.Flags().StringVar(&baseURL, "url", "", "Base URL")
	cmd.AddCommand(setCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()
			if err := cfg.Save(); err != nil {
				return err
			}
			dir, _ := config.ConfigDir()
			fmt.Printf("Config created: %s/config.json\n", dir)
			return nil
		},
	})

	return cmd
}

// ── termcode sessions ─────────────────────────────────────────────────────────

func buildSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage session history",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := session.LoadAll()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions yet.")
				return nil
			}
			fmt.Printf("%-20s  %-8s  %-12s  %s\n", "ID", "Msgs", "Provider", "Title")
			fmt.Println(strings.Repeat("─", 70))
			for _, s := range sessions {
				title := s.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Printf("%-20s  %-8d  %-12s  %s\n",
					s.ID, len(s.Messages), s.Provider, title)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <ID>",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := session.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("Session %s deleted.\n", args[0])
			return nil
		},
	})

	return cmd
}

// ── termcode version ──────────────────────────────────────────────────────────

func buildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print TermCode version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("TermCode v%s (%s)\n", version, commit)
			fmt.Println("github.com/NekoFemDev/termcode")
		},
	}
}
