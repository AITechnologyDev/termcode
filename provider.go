package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AITechnologyDev/termcode/internal/config"
)

// Message — сообщение для API
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamChunk — кусок стримингового ответа
type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

// Provider — интерфейс AI провайдера
type Provider interface {
	Stream(messages []Message, system string, maxTokens int) (<-chan StreamChunk, error)
	Name() string
	Model() string
}

// New создаёт нужный провайдер по конфигу
func New(cfg config.ProviderConfig, provider config.Provider) (Provider, error) {
	switch provider {
	case config.ProviderOllama:
		return &OllamaProvider{cfg: cfg}, nil
	case config.ProviderOpenAI, config.ProviderOpenRouter:
		return &OpenAIProvider{cfg: cfg, providerName: string(provider)}, nil
	case config.ProviderAnthropic:
		return &AnthropicProvider{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("неизвестный провайдер: %s", provider)
	}
}

// httpClient — общий клиент с таймаутом
var httpClient = &http.Client{Timeout: 120 * time.Second}

// ── Ollama ────────────────────────────────────────────────────────────────────

// OllamaProvider реализует Provider для Ollama
type OllamaProvider struct {
	cfg config.ProviderConfig
}

func (p *OllamaProvider) Name() string  { return "ollama" }
func (p *OllamaProvider) Model() string { return p.cfg.Model }

type ollamaRequest struct {
	Model    string             `json:"model"`
	Messages []ollamaMessage    `json:"messages"`
	Stream   bool               `json:"stream"`
	Options  ollamaOptions      `json:"options"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	NumPredict int     `json:"num_predict"`
	Temperature float64 `json:"temperature"`
}

type ollamaStreamResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func (p *OllamaProvider) Stream(messages []Message, system string, maxTokens int) (<-chan StreamChunk, error) {
	msgs := make([]ollamaMessage, 0, len(messages)+1)
	if system != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		msgs = append(msgs, ollamaMessage{Role: m.Role, Content: m.Content})
	}

	req := ollamaRequest{
		Model:    p.cfg.Model,
		Messages: msgs,
		Stream:   true,
		Options: ollamaOptions{
			NumPredict:  maxTokens,
			Temperature: 0.1,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("ollama: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Err: fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(data))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var chunk ollamaStreamResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}
			ch <- StreamChunk{Content: chunk.Message.Content, Done: chunk.Done}
			if chunk.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Err: err}
		}
	}()

	return ch, nil
}

// ── OpenAI-совместимый провайдер (OpenAI, OpenRouter, LM Studio, и т.д.) ──────

// OpenAIProvider реализует Provider для OpenAI-совместимых API
type OpenAIProvider struct {
	cfg          config.ProviderConfig
	providerName string
}

func (p *OpenAIProvider) Name() string  { return p.providerName }
func (p *OpenAIProvider) Model() string { return p.cfg.Model }

type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) Stream(messages []Message, system string, maxTokens int) (<-chan StreamChunk, error) {
	msgs := make([]openAIMessage, 0, len(messages)+1)
	if system != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		msgs = append(msgs, openAIMessage{Role: m.Role, Content: m.Content})
	}

	req := openAIRequest{
		Model:     p.cfg.Model,
		Messages:  msgs,
		Stream:    true,
		MaxTokens: maxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("%s: %w", p.providerName, err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Err: fmt.Errorf("%s HTTP %d: %s", p.providerName, resp.StatusCode, string(data))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}
			var chunk openAIStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				done := chunk.Choices[0].FinishReason != nil
				ch <- StreamChunk{Content: content, Done: done}
				if done {
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Err: err}
		}
	}()

	return ch, nil
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

// AnthropicProvider реализует Provider для Anthropic Claude API
type AnthropicProvider struct {
	cfg config.ProviderConfig
}

func (p *AnthropicProvider) Name() string  { return "anthropic" }
func (p *AnthropicProvider) Model() string { return p.cfg.Model }

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

func (p *AnthropicProvider) Stream(messages []Message, system string, maxTokens int) (<-chan StreamChunk, error) {
	msgs := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	req := anthropicRequest{
		Model:     p.cfg.Model,
		Messages:  msgs,
		System:    system,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("anthropic: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Err: fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(data))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Type == "text_delta" {
					ch <- StreamChunk{Content: event.Delta.Text}
				}
			case "message_stop":
				ch <- StreamChunk{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Err: err}
		}
	}()

	return ch, nil
}
