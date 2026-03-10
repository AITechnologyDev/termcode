package ai

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCallRequest — распарсенный вызов инструмента из ответа AI
type ToolCallRequest struct {
	Tool   string            `json:"tool"`
	Params map[string]string `json:"params"`
}

// ParseToolCalls ищет вызовы инструментов в тексте ответа AI.
// Поддерживает несколько форматов которые используют разные модели:
//
// Формат 1 (основной, наш):
//
//	```tool
//	{"tool": "read_file", "params": {"path": "main.go"}}
//	```
//
// Формат 2 (GLM-4, некоторые Qwen):
//
//	[tool:read_file]
//	{"path": "main.go"}
//
// Формат 3 (некоторые модели пишут просто JSON с "action"):
//
//	{"action": "read_file", "action_input": {"path": "main.go"}}
//
// Формат 4 (markdown с именем инструмента):
//
//	```read_file
//	{"path": "main.go"}
//	```
//
// Возвращает список вызовов и текст без tool-блоков.
func ParseToolCalls(text string) (calls []ToolCallRequest, cleanText string) {
	remaining := text

	// Применяем парсеры по приоритету — каждый вырезает своё из remaining
	remaining, c1 := parseFormatBacktickTool(remaining)
	remaining, c2 := parseFormatBacktickNamed(remaining)
	remaining, c3 := parseFormatBracketTool(remaining)
	remaining, c4 := parseFormatActionJSON(remaining)

	calls = append(calls, c1...)
	calls = append(calls, c2...)
	calls = append(calls, c3...)
	calls = append(calls, c4...)

	cleanText = strings.TrimSpace(remaining)
	return calls, cleanText
}

// ── Формат 1: ```tool\n{...}\n``` ────────────────────────────────────────────

func parseFormatBacktickTool(text string) (remaining string, calls []ToolCallRequest) {
	const openTag = "```tool"
	const closeTag = "```"
	var sb strings.Builder
	rest := text

	for {
		start := strings.Index(rest, openTag)
		if start == -1 {
			sb.WriteString(rest)
			break
		}
		sb.WriteString(rest[:start])
		rest = rest[start+len(openTag):]

		end := strings.Index(rest, closeTag)
		if end == -1 {
			// Незакрытый блок — пробуем парсить до конца
			jsonStr := strings.TrimSpace(rest)
			if c, ok := parseToolJSON(jsonStr); ok {
				calls = append(calls, c)
			}
			break
		}
		jsonStr := strings.TrimSpace(rest[:end])
		rest = rest[end+len(closeTag):]
		if c, ok := parseToolJSON(jsonStr); ok {
			calls = append(calls, c)
		}
	}
	return sb.String(), calls
}

// ── Формат 2: ```read_file\n{...}\n``` (имя инструмента в теге) ──────────────

var backtickNamedRe = regexp.MustCompile("(?s)```(read_file|write_file|patch_file|list_files|run_command)\n(.*?)```")

func parseFormatBacktickNamed(text string) (remaining string, calls []ToolCallRequest) {
	matches := backtickNamedRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var sb strings.Builder
	pos := 0
	for _, m := range matches {
		sb.WriteString(text[pos:m[0]])
		toolName := text[m[2]:m[3]]
		jsonStr := strings.TrimSpace(text[m[4]:m[5]])
		pos = m[1]

		params := jsonToStringMap(jsonStr)
		if params != nil {
			calls = append(calls, ToolCallRequest{Tool: toolName, Params: params})
		}
	}
	sb.WriteString(text[pos:])
	return sb.String(), calls
}

// ── Формат 3: [tool:read_file]\n{"path": "..."} (GLM-4 стиль) ───────────────

var bracketToolRe = regexp.MustCompile(`(?m)\[tool:(\w+)\]\s*\n(\{[^\n]+\})`)

func parseFormatBracketTool(text string) (remaining string, calls []ToolCallRequest) {
	matches := bracketToolRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var sb strings.Builder
	pos := 0
	for _, m := range matches {
		sb.WriteString(text[pos:m[0]])
		toolName := text[m[2]:m[3]]
		jsonStr := strings.TrimSpace(text[m[4]:m[5]])
		pos = m[1]

		params := jsonToStringMap(jsonStr)
		if params != nil {
			calls = append(calls, ToolCallRequest{Tool: toolName, Params: params})
		}
	}
	sb.WriteString(text[pos:])
	return sb.String(), calls
}

// ── Формат 4: {"action": "tool_name", "action_input": {...}} ─────────────────

var actionJSONRe = regexp.MustCompile(`(?m)^\s*\{"action":\s*"(\w+)",\s*"action_input":\s*(\{[^}]+\})\}`)

func parseFormatActionJSON(text string) (remaining string, calls []ToolCallRequest) {
	matches := actionJSONRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var sb strings.Builder
	pos := 0
	for _, m := range matches {
		sb.WriteString(text[pos:m[0]])
		toolName := text[m[2]:m[3]]
		jsonStr := strings.TrimSpace(text[m[4]:m[5]])
		pos = m[1]

		params := jsonToStringMap(jsonStr)
		if params != nil {
			calls = append(calls, ToolCallRequest{Tool: toolName, Params: params})
		}
	}
	sb.WriteString(text[pos:])
	return sb.String(), calls
}

// ── Хелперы ───────────────────────────────────────────────────────────────────

// parseToolJSON парсит стандартный {"tool": "...", "params": {...}}
func parseToolJSON(s string) (ToolCallRequest, bool) {
	var call ToolCallRequest
	if err := json.Unmarshal([]byte(s), &call); err == nil && call.Tool != "" {
		return call, true
	}
	return ToolCallRequest{}, false
}

// jsonToStringMap парсит JSON объект в map[string]string
// Значения не-строки конвертируются через JSON обратно в строку
func jsonToStringMap(s string) map[string]string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			result[k] = val
		default:
			b, _ := json.Marshal(v)
			result[k] = string(b)
		}
	}
	return result
}

// ContainsToolCall проверяет есть ли в тексте tool-блок любого формата
func ContainsToolCall(text string) bool {
	return strings.Contains(text, "```tool") ||
		strings.Contains(text, "[tool:") ||
		strings.Contains(text, `"action":`)
}
