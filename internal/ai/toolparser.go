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
	remaining, c5 := parseFormatParenParams(remaining) // read_file(params={...})

	calls = append(calls, c1...)
	calls = append(calls, c2...)
	calls = append(calls, c3...)
	calls = append(calls, c4...)
	calls = append(calls, c5...)

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

// bracketToolRe — однострочный JSON после [tool:name]
var bracketToolRe = regexp.MustCompile(`(?m)\[tool:(\w+)\]\s*\n(\{[^\n]+\})`)

// bracketToolMultiRe — многострочный JSON после [tool:name]
var bracketToolMultiRe = regexp.MustCompile(`(?s)\[tool:(\w+)\]\s*\n(\{.*?\})`)

func parseFormatBracketTool(text string) (remaining string, calls []ToolCallRequest) {
	// Сначала пробуем многострочный вариант
	matches := bracketToolMultiRe.FindAllStringSubmatchIndex(text, -1)
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

// ── Формат 5: tool_name(params={"key":"val"}) или read_file(path="go.mod") ────

// parenParamsRe ловит: read_file(params={"path":"go.mod"})
var parenParamsRe = regexp.MustCompile(`(?s)(read_file|write_file|patch_file|list_files|run_command)\s*\(\s*params\s*=\s*(\{.*?\})\s*\)`)

// parenSimpleRe ловит: read_file(path="go.mod", ...)  — без обёртки params=
var parenSimpleRe = regexp.MustCompile(`(?m)^(read_file|write_file|patch_file|list_files|run_command)\s*\(([^)]+)\)`)

func parseFormatParenParams(text string) (remaining string, calls []ToolCallRequest) {
	// Вариант A: read_file(params={"path":"go.mod"})
	matches := parenParamsRe.FindAllStringSubmatchIndex(text, -1)
	var sb strings.Builder
	pos := 0
	for _, m := range matches {
		sb.WriteString(text[pos:m[0]])
		toolName := text[m[2]:m[3]]
		jsonStr := strings.TrimSpace(text[m[4]:m[5]])
		pos = m[1]
		if params := jsonToStringMap(jsonStr); params != nil {
			calls = append(calls, ToolCallRequest{Tool: toolName, Params: params})
		}
	}
	sb.WriteString(text[pos:])
	intermediate := sb.String()

	// Вариант B: read_file(path="go.mod") — key=value без JSON
	matches2 := parenSimpleRe.FindAllStringSubmatchIndex(intermediate, -1)
	if len(matches2) == 0 {
		return intermediate, calls
	}
	var sb2 strings.Builder
	pos2 := 0
	for _, m := range matches2 {
		sb2.WriteString(intermediate[pos2:m[0]])
		toolName := intermediate[m[2]:m[3]]
		argsStr := strings.TrimSpace(intermediate[m[4]:m[5]])
		pos2 = m[1]
		if params := parseKeyValueArgs(argsStr); len(params) > 0 {
			calls = append(calls, ToolCallRequest{Tool: toolName, Params: params})
		}
	}
	sb2.WriteString(intermediate[pos2:])
	return sb2.String(), calls
}

// parseKeyValueArgs парсит строку вида: path="go.mod", old_str="foo", new_str="bar"
// или path='go.mod' или просто path=go.mod
func parseKeyValueArgs(s string) map[string]string {
	result := make(map[string]string)
	// Сначала пробуем как JSON объект
	if params := jsonToStringMap("{" + s + "}"); params != nil {
		return params
	}
	// Ручной парсинг key="value" или key='value'
	kvRe := regexp.MustCompile(`(\w+)\s*=\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'|(\S+))`)
	for _, m := range kvRe.FindAllStringSubmatch(s, -1) {
		key := m[1]
		val := m[2]
		if val == "" {
			val = m[3]
		}
		if val == "" {
			val = m[4]
		}
		result[key] = val
	}
	return result
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

// jsonToStringMap парсит JSON объект в map[string]string.
// Если значение само является объектом — разворачивает его (один уровень).
// Это нужно для случаев когда модель пишет {"params": {"path": "go.mod"}}
// вместо {"path": "go.mod"}.
func jsonToStringMap(s string) map[string]string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}

	// Если единственный ключ — "params" и значение объект — разворачиваем
	if len(raw) == 1 {
		if nested, ok := raw["params"]; ok {
			if nestedMap, ok := nested.(map[string]interface{}); ok {
				raw = nestedMap
			}
		}
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			result[k] = val
		case map[string]interface{}:
			// Вложенный объект — сериализуем в JSON строку
			b, _ := json.Marshal(val)
			result[k] = string(b)
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
		strings.Contains(text, `"action":`) ||
		parenParamsRe.MatchString(text)
}
