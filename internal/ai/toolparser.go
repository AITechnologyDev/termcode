package ai

import (
	"encoding/json"
	"strings"
)

// ToolCallRequest — распарсенный вызов инструмента из ответа AI
type ToolCallRequest struct {
	Tool   string            `json:"tool"`
	Params map[string]string `json:"params"`
}

// ParseToolCalls ищет ```tool ... ``` блоки в тексте ответа AI
// Возвращает список вызовов и текст без tool-блоков
func ParseToolCalls(text string) (calls []ToolCallRequest, cleanText string) {
	const openTag = "```tool"
	const closeTag = "```"

	var sb strings.Builder
	remaining := text

	for {
		start := strings.Index(remaining, openTag)
		if start == -1 {
			sb.WriteString(remaining)
			break
		}

		// Текст до блока
		sb.WriteString(remaining[:start])
		remaining = remaining[start+len(openTag):]

		// Ищем закрывающий тег
		end := strings.Index(remaining, closeTag)
		if end == -1 {
			// Незакрытый блок — добавляем как есть
			sb.WriteString(openTag)
			sb.WriteString(remaining)
			break
		}

		jsonStr := strings.TrimSpace(remaining[:end])
		remaining = remaining[end+len(closeTag):]

		var call ToolCallRequest
		if err := json.Unmarshal([]byte(jsonStr), &call); err == nil && call.Tool != "" {
			calls = append(calls, call)
		}
	}

	cleanText = strings.TrimSpace(sb.String())
	return calls, cleanText
}

// ContainsToolCall проверяет есть ли в тексте незавершённый tool-блок
// (нужно чтобы знать — стримить ли ещё или буфферизовать)
func ContainsToolCall(text string) bool {
	return strings.Contains(text, "```tool")
}
