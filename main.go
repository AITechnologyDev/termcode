package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/AITechnologyDev/termcode/internal/config"
	"github.com/AITechnologyDev/termcode/internal/session"
	"github.com/AITechnologyDev/termcode/internal/tui"
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
		Use:   "termcode [рабочая-директория]",
		Short: "TermCode — AI coding assistant для терминала",
		Long: `TermCode — AI-ассистент для кодинга прямо в терминале.
Работает с Ollama (локально), OpenAI, Anthropic, OpenRouter.
Читает, пишет и патчит файлы проекта по запросу.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Рабочая директория
			workDir := flagWorkDir
			if len(args) > 0 {
				workDir = args[0]
			}

			// Загружаем конфиг
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("конфиг: %w", err)
			}

			// Переопределяем провайдер и модель из флагов
			if flagProvider != "" {
				cfg.ActiveProvider = config.Provider(flagProvider)
			}
			if flagModel != "" {
				pc := cfg.Providers[cfg.ActiveProvider]
				pc.Model = flagModel
				cfg.Providers[cfg.ActiveProvider] = pc
			}

			// Запускаем TUI
			m, err := tui.New(cfg, workDir)
			if err != nil {
				return fmt.Errorf("инициализация TUI: %w", err)
			}

			return tui.Start(m)
		},
	}

	root.Flags().StringVarP(&flagProvider, "provider", "p", "",
		"AI провайдер: ollama, openai, anthropic, openrouter")
	root.Flags().StringVarP(&flagModel, "model", "m", "",
		"Модель (переопределяет конфиг)")
	root.Flags().StringVarP(&flagWorkDir, "dir", "d", "",
		"Рабочая директория проекта")

	// Подкоманды
	root.AddCommand(
		buildConfigCmd(),
		buildSessionsCmd(),
		buildVersionCmd(),
	)

	return root
}

// ── termcode config ───────────────────────────────────────────────────────────

func buildConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Управление конфигурацией",
	}

	// config show
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Показать текущий конфиг",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			dir, _ := config.ConfigDir()
			fmt.Printf("Конфиг: %s/config.json\n\n", dir)
			fmt.Printf("Активный провайдер: %s\n", cfg.ActiveProvider)
			fmt.Printf("Провайдеры:\n")
			for name, pc := range cfg.Providers {
				key := pc.APIKey
				if key != "" {
					if len(key) > 8 {
						key = key[:4] + "..." + key[len(key)-4:]
					}
				} else {
					key = "(не задан)"
				}
				active := ""
				if config.Provider(name) == cfg.ActiveProvider {
					active = " ◄ активен"
				}
				fmt.Printf("  %-12s  модель: %-30s  url: %s  key: %s%s\n",
					name, pc.Model, pc.BaseURL, key, active)
			}
			return nil
		},
	})

	// config set-provider
	var apiKey, model, baseURL string
	setCmd := &cobra.Command{
		Use:   "set-provider <провайдер>",
		Short: "Установить активный провайдер и его параметры",
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
			fmt.Printf("Сохранено: активный провайдер = %s, модель = %s\n", provider, pc.Model)
			return nil
		},
	}
	setCmd.Flags().StringVar(&apiKey, "key", "", "API ключ")
	setCmd.Flags().StringVar(&model, "model", "", "Модель")
	setCmd.Flags().StringVar(&baseURL, "url", "", "Base URL")
	cmd.AddCommand(setCmd)

	// config init — создаёт дефолтный конфиг
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Создать дефолтный конфиг",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()
			if err := cfg.Save(); err != nil {
				return err
			}
			dir, _ := config.ConfigDir()
			fmt.Printf("Конфиг создан: %s/config.json\n", dir)
			return nil
		},
	})

	return cmd
}

// ── termcode sessions ─────────────────────────────────────────────────────────

func buildSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Управление историей сессий",
	}

	// sessions list
	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Показать список сессий",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := session.LoadAll()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Println("Сессий пока нет.")
				return nil
			}
			fmt.Printf("%-20s  %-8s  %-10s  %s\n", "ID", "Сообщ.", "Провайдер", "Заголовок")
			fmt.Println(strings.Repeat("─", 70))
			for _, s := range sessions {
				title := s.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Printf("%-20s  %-8d  %-10s  %s\n",
					s.ID, len(s.Messages), s.Provider, title)
			}
			return nil
		},
	})

	// sessions delete
	cmd.AddCommand(&cobra.Command{
		Use:   "delete <ID>",
		Short: "Удалить сессию",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := session.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("Сессия %s удалена.\n", args[0])
			return nil
		},
	})

	return cmd
}

// ── termcode version ──────────────────────────────────────────────────────────

func buildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Версия TermCode",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("TermCode v%s (%s)\n", version, commit)
			fmt.Println("github.com/AITechnologyDev/termcode")
		},
	}
}
