package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Role — роль участника диалога
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// ToolCall — вызов инструмента в сообщении
type ToolCall struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params"`
	Result string            `json:"result,omitempty"`
	Error  string            `json:"error,omitempty"`
}

// Message — одно сообщение в диалоге
type Message struct {
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Session — сессия диалога с AI
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	WorkDir   string    `json:"work_dir"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// New создаёт новую сессию
func New(workDir, provider, model string) *Session {
	now := time.Now()
	return &Session{
		ID:        fmt.Sprintf("%d", now.UnixMilli()),
		Title:     "Новая сессия",
		WorkDir:   workDir,
		Provider:  provider,
		Model:     model,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage добавляет сообщение в сессию
func (s *Session) AddMessage(role Role, content string) {
	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	})
	s.UpdatedAt = time.Now()

	// Авто-заголовок из первого сообщения пользователя
	if s.Title == "Новая сессия" && role == RoleUser && len(content) > 0 {
		title := content
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		s.Title = title
	}
}

// AddToolCall добавляет вызов инструмента к последнему сообщению ассистента
func (s *Session) AddToolCall(tc ToolCall) {
	if len(s.Messages) == 0 {
		return
	}
	last := &s.Messages[len(s.Messages)-1]
	if last.Role == RoleAssistant {
		last.ToolCalls = append(last.ToolCalls, tc)
	}
	s.UpdatedAt = time.Now()
}

// APIMessages возвращает сообщения в формате для API (без tool results в отдельных полях)
func (s *Session) APIMessages() []map[string]string {
	result := make([]map[string]string, 0, len(s.Messages))
	for _, m := range s.Messages {
		if m.Role == RoleSystem {
			continue
		}
		result = append(result, map[string]string{
			"role":    string(m.Role),
			"content": m.Content,
		})
	}
	return result
}

// sessionsDir возвращает путь к директории сессий
func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "termcode", "sessions"), nil
}

// Save сохраняет сессию на диск
func (s *Session) Save() error {
	dir, err := sessionsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, s.ID+".json")
	return os.WriteFile(path, data, 0600)
}

// LoadAll загружает все сессии, отсортированные по дате (новые первые)
func LoadAll() ([]*Session, error) {
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	sessions := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, &s)
	}

	// Сортируем: новые первые
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete удаляет сессию с диска
func Delete(id string) error {
	dir, err := sessionsDir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, id+".json"))
}
