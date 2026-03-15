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
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	// MaxTokens — максимум токенов в одном ответе (0 = авто по модели)
	MaxTokens     int    `json:"max_tokens,omitempty"`
	// ContextLength — размер окна контекста (0 = авто по модели)
	ContextLength int    `json:"context_length,omitempty"`
}

// modelLimits — известные лимиты моделей: [max_tokens, context_length]
// Если модели нет в таблице — используем консервативные дефолты
var modelLimits = map[string][2]int{
	// GLM
	"glm-4":          {4096, 128000},
	"glm-4v":         {1024, 128000},
	"glm-4-flash":    {4096, 128000},
	"glm-4.7":        {8192, 128000},
	"glm-4.7:cloud":  {8192, 128000},
	"glm-3-turbo":    {4096, 32768},
	// Qwen
	"qwen2.5-coder:7b":        {8192, 32768},
	"qwen2.5-coder:14b":       {8192, 32768},
	"qwen2.5-coder:32b":       {8192, 32768},
	"qwen3:8b":                {8192, 32768},
	"qwen3:14b":               {8192, 32768},
	"qwen3:32b":               {8192, 32768},
	"qwen3-coder":             {16384, 131072},
	"qwen3-coder:cloud":       {32768, 262144},
	"qwen3-coder-next:cloud":  {32768, 262144},
	"qwen3-coder-next":        {32768, 262144},
	"qwen2.5-coder:cloud":     {16384, 131072},
	// OpenAI
	"gpt-4o":             {16384, 128000},
	"gpt-4o-mini":        {16384, 128000},
	"gpt-4-turbo":        {4096, 128000},
	"o1-mini":            {65536, 128000},
	// Anthropic
	"claude-opus-4-6":         {32000, 200000},
	"claude-sonnet-4-6":  {16000, 200000},
	"claude-haiku-4-5-20251001":  {16000, 200000},
	// DeepSeek
	"deepseek-coder":     {8192, 32768},
	"deepseek-r1":        {16384, 65536},
	// Llama
	"llama3.1:8b":        {4096, 131072},
	"llama3.1:70b":       {4096, 131072},
	"llama3.2:3b":        {4096, 131072},
	// Mistral
	"mistral:7b":         {4096, 32768},
	"codestral":          {8192, 32768},
}

// GetMaxTokens возвращает лимит токенов ответа для данного конфига провайдера
func (pc ProviderConfig) GetMaxTokens() int {
	if pc.MaxTokens > 0 {
		return pc.MaxTokens
	}
	if lim, ok := modelLimits[pc.Model]; ok {
		return lim[0]
	}
	// Консервативный дефолт для неизвестных моделей
	return 4096
}

// GetContextLength возвращает размер контекстного окна
func (pc ProviderConfig) GetContextLength() int {
	if pc.ContextLength > 0 {
		return pc.ContextLength
	}
	if lim, ok := modelLimits[pc.Model]; ok {
		return lim[1]
	}
	// Консервативный дефолт
	return 8192
}

// Config — главный конфиг TermCode
type Config struct {
	ActiveProvider Provider `json:"active_provider"`
	Providers      map[Provider]ProviderConfig `json:"providers"`
	WorkDir        string   `json:"work_dir,omitempty"`
	Theme          string   `json:"theme"`
	// Language — язык интерфейса и системного промпта ("en" / "ru")
	Language       string   `json:"language"`
	SystemPrompt   string   `json:"system_prompt"`
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
		Theme:        "dark",
		Language:     "en",
		SystemPrompt: `You are TermCode — an AI coding assistant running inside a terminal on Android (Termux).

TOOL USAGE — CRITICAL:
- To use a tool, output EXACTLY this format and nothing else before/after the block:
` + "```" + `tool
{"tool": "tool_name", "params": {"key": "value"}}
` + "```" + `
- Never use [tool:name] format, never use {"action": ...} format
- Never write tool calls as plain text or comments
- Call ONE tool per response turn, then wait for the result

CODING STYLE:
- Use patch_file for small changes, write_file only for new files or full rewrites
- Always read files before modifying them
- Be concise — prefer code over long explanations
- After tool results are shown, continue with next steps

ASKING QUESTIONS:
When a request is ambiguous, ask using this format:
` + "```" + `question
Your question here?
- Option A
- Option B
- Option C
` + "```" + ``,
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
