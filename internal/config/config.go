package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Provider идентифицирует AI провайдера
type Provider string

const (
	ProviderOllama     Provider = "ollama"
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenRouter Provider = "openrouter"
)

// ProviderKind — тип провайдера для UI-логики
type ProviderKind string

const (
	KindLocal ProviderKind = "local" // запускается локально (ollama)
	KindCloud ProviderKind = "cloud" // требует интернет + ключ
	KindFree  ProviderKind = "free"  // free-tier, ключ опционален
)

// ProviderMeta — метаданные провайдера для UI.
// Эти данные отображаются в селекторе провайдеров и палитре.
type ProviderMeta struct {
	ID           Provider
	Label        string       // короткое имя: "Ollama"
	Description  string       // описание в одну строку
	Icon         string       // эмодзи/символ для UI
	Kind         ProviderKind // local / cloud / free
	RequiresKey  bool         // нужен ли API-ключ
	KeyURL       string       // где взять ключ
	DefaultModel string       // модель по умолчанию
	DefaultURL   string       // base URL по умолчанию
	Order        int          // порядок в списке (меньше = выше)
}

// providersMeta — единственный источник правды о провайдерах.
// Порядок и метаданные для селектора провайдера берутся ТОЛЬКО отсюда.
var providersMeta = []ProviderMeta{
	{
		ID:           ProviderOllama,
		Label:        "Ollama",
		Description:  "Local server + free cloud models",
		Icon:         "◎",
		Kind:         KindLocal,
		RequiresKey:  false, // для локального Ollama; для cloud нужен ключ, но не обязателен
		KeyURL:       "https://ollama.com/settings/api-keys",
		DefaultModel: "qwen3-coder-next",
		DefaultURL:   "https://ollama.com/api",
		Order:        1,
	},
	{
		ID:           ProviderOpenRouter,
		Label:        "OpenRouter",
		Description:  "Many free + paid models, one key",
		Icon:         "⊕",
		Kind:         KindFree,
		RequiresKey:  true,
		KeyURL:       "https://openrouter.ai/keys",
		DefaultModel: "nvidia/nemotron-3-super-120b-a12b:free",
		DefaultURL:   "https://openrouter.ai/api/v1",
		Order:        2,
	},
	{
		ID:           ProviderOpenAI,
		Label:        "OpenAI",
		Description:  "GPT-4o, GPT-5, o1, o3",
		Icon:         "◯",
		Kind:         KindCloud,
		RequiresKey:  true,
		KeyURL:       "https://platform.openai.com/api-keys",
		DefaultModel: "gpt-4o-mini",
		DefaultURL:   "https://api.openai.com/v1",
		Order:        3,
	},
	{
		ID:           ProviderAnthropic,
		Label:        "Anthropic",
		Description:  "Claude Opus, Sonnet, Haiku",
		Icon:         "△",
		Kind:         KindCloud,
		RequiresKey:  true,
		KeyURL:       "https://console.anthropic.com/settings/keys",
		DefaultModel: "claude-sonnet-4-20250514",
		DefaultURL:   "https://api.anthropic.com",
		Order:        4,
	},
}

// ProvidersMeta возвращает список всех провайдеров в стабильном порядке.
func ProvidersMeta() []ProviderMeta {
	out := make([]ProviderMeta, len(providersMeta))
	copy(out, providersMeta)
	// стабильная сортировка по Order
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Order < out[i].Order {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// GetProviderMeta возвращает метаданные провайдера по ID.
// Если провайдер неизвестен — возвращает "generic" метаданные.
func GetProviderMeta(id Provider) ProviderMeta {
	for _, p := range providersMeta {
		if p.ID == id {
			return p
		}
	}
	return ProviderMeta{
		ID:          id,
		Label:       string(id),
		Description: "",
		Icon:        "·",
	}
}

// ProviderConfig — настройки одного провайдера
type ProviderConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	// MaxTokens — максимум токенов в одном ответе (0 = авто по модели)
	MaxTokens int `json:"max_tokens,omitempty"`
	// ContextLength — размер окна контекста (0 = авто по модели)
	ContextLength int `json:"context_length,omitempty"`
}

// NeedsAPIKey возвращает true, если провайдер обычно требует API-ключ.
// Для Ollama — опционально (нужен только для cloud моделей на ollama.com).
func (pc ProviderConfig) NeedsAPIKey(id Provider) bool {
	if id == ProviderOllama {
		// Для локального Ollama ключ не нужен
		return strings.Contains(pc.BaseURL, "ollama.com")
	}
	return true
}

// MaskAPIKey возвращает замаскированный ключ для отображения.
func (pc ProviderConfig) MaskAPIKey() string {
	if pc.APIKey == "" {
		return ""
	}
	if len(pc.APIKey) <= 8 {
		return strings.Repeat("*", len(pc.APIKey))
	}
	return pc.APIKey[:4] + strings.Repeat("•", len(pc.APIKey)-8) + pc.APIKey[len(pc.APIKey)-4:]
}

// modelLimits — известные лимиты моделей: [max_tokens, context_length]
var modelLimits = map[string][2]int{
	// NVIDIA Nemotron-3-Super — 120B/12B active, 1M context, бесплатно на OpenRouter
	"nvidia/nemotron-3-super-120b-a12b:free": {32768, 1000000},
	"nvidia/nemotron-3-super-120b-a12b":      {32768, 1000000},
	"nemotron-3-super":                       {32768, 1000000},
	// NVIDIA Nemotron 3 Nano
	"nvidia/nemotron-3-nano:free": {16384, 128000},
	"nemotron-3-nano":             {16384, 128000},
	// ── g4f провайдеры (через g4f.space) ─────────────────────────────────
	// Anthropic (через puter, azure и др.)
	"anthropic/claude-opus-4-6":   {32768, 200000},
	"anthropic/claude-sonnet-4-6": {32768, 200000},
	"anthropic/claude-haiku-4-5":  {16384, 200000},
	// OpenAI (через puter, PollinationsAI и др.)
	"openai/gpt-4o":        {16384, 128000},
	"openai/gpt-4o-mini":   {16384, 128000},
	"openai/gpt-4.1":       {32768, 1000000},
	"openai/gpt-5.3-codex": {65536, 1000000},
	"openai/o1":            {65536, 200000},
	"openai/o3-mini":       {65536, 200000},
	// Google (через puter и др.)
	"google/gemini-2.5-pro":   {65536, 1000000},
	"google/gemini-2.5-flash": {65536, 1000000},
	// DeepSeek (через g4f)
	"deepseek/deepseek-r1": {32768, 128000},
	"deepseek/deepseek-v3": {32768, 128000},
	// Meta (через g4f)
	"meta/llama-3.1-70b": {8192, 131072},
	"meta/llama-3.3-70b": {8192, 131072},

	// ── Ollama локальные модели ───────────────────────────────────────────
	"qwen2.5-coder:7b":       {8192, 32768},
	"qwen2.5-coder:14b":      {8192, 32768},
	"qwen2.5-coder:32b":      {8192, 32768},
	"qwen3:8b":               {8192, 32768},
	"qwen3:14b":              {8192, 32768},
	"qwen3:32b":              {8192, 32768},
	"qwen3-coder":            {16384, 131072},
	"qwen3-coder:cloud":      {32768, 262144},
	"qwen3-coder-next:cloud": {32768, 262144},
	"qwen3-coder-next":       {32768, 262144},
	"qwen2.5-coder:cloud":    {16384, 131072},
	"glm-4":                  {4096, 128000},
	"glm-4v":                 {1024, 128000},
	"glm-4-flash":            {4096, 128000},
	"glm-4.7":                {8192, 128000},
	"glm-4.7:cloud":          {8192, 128000},
	"glm-3-turbo":            {4096, 32768},
	"glm-5:cloud":            {16384, 128000},
	"llama3.1:8b":            {4096, 131072},
	"llama3.1:70b":           {4096, 131072},
	"llama3.2:3b":            {4096, 131072},
	"mistral:7b":             {4096, 32768},
	"codestral":              {8192, 32768},
	"deepseek-coder":         {8192, 32768},
	"deepseek-r1":            {16384, 65536},

	// ── Прямые API (OpenAI, Anthropic, OpenRouter) ────────────────────────
	"gpt-4o":                    {16384, 128000},
	"gpt-4o-mini":               {16384, 128000},
	"gpt-4-turbo":               {4096, 128000},
	"o1-mini":                   {65536, 128000},
	"claude-opus-4-6":           {32000, 200000},
	"claude-sonnet-4-6":         {16000, 200000},
	"claude-haiku-4-5-20251001": {16000, 200000},
	"gemini-2.5-flash":          {65536, 1000000},
	"gemini-2.5-pro":            {65536, 1000000},
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
	ActiveProvider Provider                    `json:"active_provider"`
	Providers      map[Provider]ProviderConfig `json:"providers"`
	WorkDir        string                      `json:"work_dir,omitempty"`
	Theme          string                      `json:"theme"`
	Language       string                      `json:"language"`
	SystemPrompt   string                      `json:"system_prompt"`
	// UserProfile — описание пользователя (кто я, чем занимаюсь)
	UserProfile string `json:"user_profile,omitempty"`
	// AIInstructions — инструкции для AI (как отвечать, стиль, уровень)
	AIInstructions string `json:"ai_instructions,omitempty"`
}

// DefaultConfig возвращает конфиг с разумными дефолтами
func DefaultConfig() *Config {
	providers := make(map[Provider]ProviderConfig, len(providersMeta))
	for _, p := range providersMeta {
		providers[p.ID] = ProviderConfig{
			BaseURL: p.DefaultURL,
			APIKey:  "",
			Model:   p.DefaultModel,
		}
	}
	return &Config{
		ActiveProvider: ProviderOllama,
		Providers:      providers,
		Theme:          "dark",
		Language:       "en",
		UserProfile:    "",
		AIInstructions: "",
		SystemPrompt: `You are TermCode — an AI coding assistant running inside a terminal on Android (Termux).

TOOL USAGE — CRITICAL:
- To use a tool, output EXACTLY this format and nothing else before/after the block:
` + "```" + `tool
{"tool": "tool_name", "params": {"key": "value"}}
` + "```" + `
- Never use [tool:name] format, never use {"action": ...} format
- Never write tool calls as plain text or comments
- Call ONE tool per response turn, then wait for the result

ASKING QUESTIONS — MANDATORY RULE:
When you need the user to choose between options, you MUST use the ask_user tool.
NEVER write "Options:", "Choose:", "Which would you like?" as plain text.
NEVER present a numbered or bulleted list of choices in your response text.
ALWAYS use ask_user tool instead. This is not optional.

CODING STYLE:
- Use patch_file for small changes, write_file only for new files or full rewrites
- Always read files before modifying them
- Be concise — prefer code over long explanations
- After tool results are shown, continue with next steps`,
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
