package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Provider идентифицирует AI провайдера
type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenRouter Provider = "openrouter"
)

// ProviderConfig — настройки одного провайдера
type ProviderConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// Config — главный конфиг TermCode
type Config struct {
	// Активный провайдер
	ActiveProvider Provider `json:"active_provider"`

	// Провайдеры
	Providers map[Provider]ProviderConfig `json:"providers"`

	// Рабочая директория (если пуста — текущая)
	WorkDir string `json:"work_dir,omitempty"`

	// Тема (dark / light)
	Theme string `json:"theme"`

	// Максимум токенов в ответе
	MaxTokens int `json:"max_tokens"`

	// Системный промпт
	SystemPrompt string `json:"system_prompt"`
}

// DefaultConfig возвращает конфиг с разумными дефолтами
func DefaultConfig() *Config {
	return &Config{
		ActiveProvider: ProviderOllama,
		Providers: map[Provider]ProviderConfig{
			ProviderOllama: {
				BaseURL: "http://127.0.0.1:11434",
				Model:   "qwen2.5-coder:7b",
			},
			ProviderOpenAI: {
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
			ProviderAnthropic: {
				BaseURL: "https://api.anthropic.com",
				Model:   "claude-sonnet-4-20250514",
			},
			ProviderOpenRouter: {
				BaseURL: "https://openrouter.ai/api/v1",
				Model:   "qwen/qwen3-8b:free",
			},
		},
		Theme:     "dark",
		MaxTokens: 8192,
		SystemPrompt: `You are TermCode — an AI coding assistant running in a terminal.
You have access to tools: read_file, write_file, patch_file, list_files, run_command.
When modifying code: prefer patch_file over full rewrites for small changes.
Always explain what you're doing before calling a tool.
Be concise. Prefer code over long explanations.

When a user request is ambiguous or you need clarification, ask an interactive question using this format:
` + "```" + `question
Your question here?
- Option A
- Option B
- Option C
` + "```" + `
The user will see your options as clickable choices and can also type a custom answer.
Only use question blocks when genuinely needed — don't overuse them.`,
	}
}

// ConfigDir возвращает путь к директории конфигов
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось найти home dir: %w", err)
	}
	return filepath.Join(home, ".config", "termcode"), nil
}

// Load загружает конфиг из файла или возвращает дефолтный
func Load() (*Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	path := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("чтение конфига: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("парсинг конфига: %w", err)
	}
	return cfg, nil
}

// Save сохраняет конфиг на диск
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("создание config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализация конфига: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, data, 0600)
}

// ActiveProviderConfig возвращает конфиг активного провайдера
func (c *Config) ActiveProviderConfig() (ProviderConfig, bool) {
	pc, ok := c.Providers[c.ActiveProvider]
	return pc, ok
}
