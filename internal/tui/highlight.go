package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Цвета подсветки (совместимы с тёмной темой TermCode) ─────────────────────

var (
	hlKeyword = lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD"))
	hlBuiltin = lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2"))
	hlString  = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))
	hlNumber  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D19A66"))
	hlComment = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Italic(true)
	hlFunc    = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	hlType    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
	hlAttr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75"))
)

// ── Определения языков ────────────────────────────────────────────────────────

type tokenKind int

const (
	tkString tokenKind = iota
	tkNumber
)

// langDef описывает правила токенизации для одного языка
type langDef struct {
	lineComment  string   // однострочный комментарий
	blockStart   string   // начало блочного комментария
	blockEnd     string   // конец блочного комментария
	keywords     []string
	builtins     []string
	types        []string
}

var langs = map[string]langDef{
	"go": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"package", "import", "func", "var", "const", "type", "struct",
			"interface", "map", "chan", "go", "defer", "return", "if", "else",
			"for", "range", "switch", "case", "default", "break", "continue",
			"select", "fallthrough", "goto", "make", "new", "nil", "true", "false",
		},
		builtins: []string{
			"len", "cap", "append", "copy", "delete", "close", "panic", "recover",
			"print", "println", "error", "string", "int", "int64", "int32", "uint",
			"uint64", "uint32", "byte", "rune", "float64", "float32", "bool",
			"complex64", "complex128",
		},
		types: []string{
			"string", "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
			"float32", "float64", "complex64", "complex128",
			"bool", "byte", "rune", "error",
		},
	},
	"python": {
		lineComment: "#",
		keywords: []string{
			"def", "class", "return", "if", "elif", "else", "for", "while",
			"in", "not", "and", "or", "is", "import", "from", "as", "with",
			"try", "except", "finally", "raise", "pass", "break", "continue",
			"lambda", "yield", "async", "await", "global", "nonlocal", "del",
		},
		builtins: []string{
			"print", "len", "range", "enumerate", "zip", "map", "filter",
			"list", "dict", "set", "tuple", "str", "int", "float", "bool",
			"type", "isinstance", "hasattr", "getattr", "setattr", "super",
			"open", "None", "True", "False",
		},
	},
	"javascript": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"const", "let", "var", "function", "return", "if", "else", "for",
			"while", "do", "switch", "case", "default", "break", "continue",
			"new", "delete", "typeof", "instanceof", "in", "of", "class",
			"extends", "import", "export", "from", "async", "await", "try",
			"catch", "finally", "throw", "null", "undefined", "true", "false",
			"this", "super", "static", "get", "set",
		},
		builtins: []string{
			"console", "document", "window", "Array", "Object", "String",
			"Number", "Boolean", "Promise", "Math", "JSON", "Date", "Error",
			"Map", "Set", "Symbol", "Proxy", "Reflect",
		},
	},
	"typescript": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"const", "let", "var", "function", "return", "if", "else", "for",
			"while", "do", "switch", "case", "default", "break", "continue",
			"new", "delete", "typeof", "instanceof", "in", "of", "class",
			"extends", "import", "export", "from", "async", "await", "try",
			"catch", "finally", "throw", "null", "undefined", "true", "false",
			"this", "super", "static", "interface", "type", "enum", "namespace",
			"abstract", "implements", "readonly", "declare", "as", "keyof", "infer",
		},
		types: []string{
			"string", "number", "boolean", "void", "never", "any", "unknown",
			"object", "symbol", "bigint", "null", "undefined",
		},
	},
	"bash": {
		lineComment: "#",
		keywords: []string{
			"if", "then", "else", "elif", "fi", "for", "do", "done", "while",
			"until", "case", "esac", "in", "function", "return", "exit",
			"break", "continue", "local", "export", "readonly", "unset",
			"shift", "source", "echo", "printf", "read",
		},
		builtins: []string{
			"cd", "ls", "pwd", "mkdir", "rm", "cp", "mv", "cat", "grep",
			"sed", "awk", "cut", "sort", "uniq", "wc", "find", "chmod",
			"chown", "tar", "curl", "wget", "git", "go", "make",
		},
	},
	"sh": {lineComment: "#"},
	"rust": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"fn", "let", "mut", "const", "static", "struct", "enum", "trait",
			"impl", "for", "in", "while", "loop", "if", "else", "match",
			"return", "break", "continue", "use", "mod", "pub", "crate",
			"super", "self", "Self", "type", "where", "async", "await",
			"move", "ref", "unsafe", "extern", "dyn", "box", "as",
			"true", "false", "None", "Some", "Ok", "Err",
		},
		builtins: []string{
			"println", "print", "eprintln", "eprint", "format", "panic",
			"assert", "assert_eq", "assert_ne", "vec", "todo", "unimplemented",
			"unreachable", "dbg", "include_str", "env",
		},
		types: []string{
			"i8", "i16", "i32", "i64", "i128", "isize",
			"u8", "u16", "u32", "u64", "u128", "usize",
			"f32", "f64", "bool", "char", "str", "String",
			"Vec", "HashMap", "HashSet", "Option", "Result",
			"Box", "Rc", "Arc", "Cell", "RefCell",
		},
	},
	"java": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"public", "private", "protected", "static", "final", "abstract",
			"class", "interface", "enum", "extends", "implements", "new",
			"return", "if", "else", "for", "while", "do", "switch", "case",
			"default", "break", "continue", "try", "catch", "finally", "throw",
			"throws", "import", "package", "this", "super", "null", "true",
			"false", "instanceof", "void", "synchronized", "volatile",
			"transient", "native", "strictfp",
		},
		builtins: []string{
			"System", "String", "Integer", "Long", "Double", "Boolean",
			"Object", "Class", "Math", "Arrays", "Collections",
			"ArrayList", "HashMap", "HashSet", "List", "Map", "Set",
			"Optional", "Stream", "StringBuilder",
		},
		types: []string{
			"int", "long", "double", "float", "boolean", "char", "byte",
			"short", "void", "String", "Object",
		},
	},
	"kotlin": {
		lineComment: "//",
		blockStart:  "/*",
		blockEnd:    "*/",
		keywords: []string{
			"fun", "val", "var", "class", "object", "interface", "enum",
			"data", "sealed", "abstract", "open", "override", "final",
			"private", "public", "protected", "internal", "companion",
			"return", "if", "else", "when", "for", "while", "do",
			"break", "continue", "try", "catch", "finally", "throw",
			"import", "package", "this", "super", "null", "true", "false",
			"in", "is", "as", "by", "get", "set", "init", "constructor",
			"suspend", "inline", "reified", "crossinline", "noinline",
			"lateinit", "lazy", "tailrec", "operator", "infix", "extension",
		},
		builtins: []string{
			"println", "print", "listOf", "mapOf", "setOf", "mutableListOf",
			"mutableMapOf", "arrayOf", "emptyList", "emptyMap", "TODO",
			"also", "let", "run", "apply", "with", "takeIf", "takeUnless",
		},
		types: []string{
			"Int", "Long", "Double", "Float", "Boolean", "Char", "Byte",
			"Short", "String", "Unit", "Any", "Nothing",
			"List", "Map", "Set", "Array", "Pair", "Triple",
		},
	},
	"sql": {
		keywords: []string{
			"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "INSERT", "INTO",
			"VALUES", "UPDATE", "SET", "DELETE", "CREATE", "TABLE", "DROP",
			"ALTER", "ADD", "COLUMN", "INDEX", "JOIN", "LEFT", "RIGHT", "INNER",
			"OUTER", "ON", "GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET",
			"AS", "DISTINCT", "COUNT", "SUM", "AVG", "MAX", "MIN",
			"NULL", "IS", "IN", "LIKE", "BETWEEN", "EXISTS", "UNION",
		},
	},
}

// Псевдонимы языков
var langAliases = map[string]string{
	"js":   "javascript",
	"ts":   "typescript",
	"py":   "python",
	"sh":   "bash",
	"zsh":  "bash",
	"fish": "bash",
	"rs":   "rust",
	"kt":   "kotlin",
	"kts":  "kotlin",
}

// ── Главная функция подсветки ─────────────────────────────────────────────────

// HighlightCode применяет syntax highlighting к блоку кода.
// lang — язык (go, python, js и т.д.), может быть пустым.
// Возвращает строки с ANSI-раскраской, готовые для вывода в терминал.
func HighlightCode(code, lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if alias, ok := langAliases[lang]; ok {
		lang = alias
	}

	// JSON обрабатываем отдельно
	if lang == "json" {
		return highlightJSON(code)
	}

	def, ok := langs[lang]
	if !ok {
		// Неизвестный язык — просто возвращаем как есть
		return code
	}

	var sb strings.Builder
	lines := strings.Split(code, "\n")
	inBlockComment := false

	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(highlightLine(line, def, &inBlockComment))
	}
	return sb.String()
}

// highlightLine подсвечивает одну строку кода
func highlightLine(line string, def langDef, inBlock *bool) string {
	if line == "" {
		return ""
	}

	// Конец блочного комментария
	if *inBlock {
		if def.blockEnd != "" {
			idx := strings.Index(line, def.blockEnd)
			if idx >= 0 {
				*inBlock = false
				end := idx + len(def.blockEnd)
				return hlComment.Render(line[:end]) + highlightLine(line[end:], def, inBlock)
			}
		}
		return hlComment.Render(line)
	}

	// Строчный комментарий — всё после него серое
	if def.lineComment != "" {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, def.lineComment) {
			// Находим позицию комментария в оригинальной строке
			idx := strings.Index(line, def.lineComment)
			if idx >= 0 {
				return line[:idx] + hlComment.Render(line[idx:])
			}
		}
	}

	// Начало блочного комментария
	if def.blockStart != "" {
		idx := strings.Index(line, def.blockStart)
		if idx >= 0 {
			endIdx := strings.Index(line[idx+len(def.blockStart):], def.blockEnd)
			if endIdx < 0 {
				// Многострочный блок
				*inBlock = true
				return highlightLine(line[:idx], def, inBlock) + hlComment.Render(line[idx:])
			}
			// Однострочный блок /*...*/
			end := idx + len(def.blockStart) + endIdx + len(def.blockEnd)
			return highlightLine(line[:idx], def, inBlock) +
				hlComment.Render(line[idx:end]) +
				highlightLine(line[end:], def, inBlock)
		}
	}

	return tokenizeLine(line, def)
}

// ── Токенизация строки ────────────────────────────────────────────────────────

// span — диапазон символов с типом токена
type span struct {
	start, end int
	kind       tokenKind
	text       string
}

var (
	// Строки в кавычках (с поддержкой escape)
	reString1 = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	reString2 = regexp.MustCompile("`[^`]*`")
	reString3 = regexp.MustCompile(`'(?:[^'\\]|\\.)*'`)
	// Числа
	reNumber = regexp.MustCompile(`\b(?:0[xXoObB][0-9a-fA-F_]+|[0-9][0-9_]*(?:\.[0-9_]*)?(?:[eE][+-]?[0-9_]+)?)\b`)
	// Вызовы функций: name(
	reFunc = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	// Идентификаторы
	reIdent = regexp.MustCompile(`\b[a-zA-Z_][a-zA-Z0-9_]*\b`)
)

func tokenizeLine(line string, def langDef) string {
	kwSet := toSet(def.keywords)
	biSet := toSet(def.builtins)
	tySet := toSet(def.types)

	var spans []span

	// Строки (приоритет выше всего)
	for _, re := range []*regexp.Regexp{reString1, reString2, reString3} {
		for _, m := range re.FindAllStringIndex(line, -1) {
			spans = append(spans, span{m[0], m[1], tkString, line[m[0]:m[1]]})
		}
	}

	// Числа (только вне строк — проверим ниже)
	for _, m := range reNumber.FindAllStringIndex(line, -1) {
		spans = append(spans, span{m[0], m[1], tkNumber, line[m[0]:m[1]]})
	}

	// Убираем пересечения — строки имеют приоритет
	spans = removeDuplicates(spans)

	// Строим результат
	var sb strings.Builder
	pos := 0
	for _, sp := range spans {
		if sp.start < pos {
			continue
		}
		// Текст между спанами — токенизируем по словам
		if sp.start > pos {
			sb.WriteString(colorizeWords(line[pos:sp.start], kwSet, biSet, tySet))
		}
		switch sp.kind {
		case tkString:
			sb.WriteString(hlString.Render(sp.text))
		case tkNumber:
			sb.WriteString(hlNumber.Render(sp.text))
		}
		pos = sp.end
	}
	if pos < len(line) {
		sb.WriteString(colorizeWords(line[pos:], kwSet, biSet, tySet))
	}
	return sb.String()
}

// colorizeWords красит слова-идентификаторы в текстовом сегменте
func colorizeWords(segment string, kwSet, biSet, tySet map[string]bool) string {
	if segment == "" {
		return ""
	}
	result := reIdent.ReplaceAllStringFunc(segment, func(word string) string {
		switch {
		case kwSet[word] || kwSet[strings.ToUpper(word)]:
			return hlKeyword.Render(word)
		case tySet[word]:
			return hlType.Render(word)
		case biSet[word]:
			return hlBuiltin.Render(word)
		default:
			return word
		}
	})
	// Подсвечиваем имена функций перед (
	result = reFunc.ReplaceAllStringFunc(result, func(s string) string {
		// s = "funcname(" — но funcname уже мог быть покрашен
		// Ищем последний ( и красим всё до него
		idx := strings.LastIndex(s, "(")
		if idx < 0 {
			return s
		}
		name := s[:idx]
		// Если уже содержит ANSI — не трогаем (уже покрашен как keyword)
		if strings.Contains(name, "\x1b") {
			return s
		}
		return hlFunc.Render(name) + "("
	})
	return result
}

// ── JSON подсветка ────────────────────────────────────────────────────────────

var (
	reJSONKey    = regexp.MustCompile(`"([^"]+)"\s*:`)
	reJSONString = regexp.MustCompile(`:\s*"([^"]*)"`)
	reJSONNum    = regexp.MustCompile(`:\s*(-?[0-9]+(?:\.[0-9]+)?)`)
	reJSONBool   = regexp.MustCompile(`\b(true|false|null)\b`)
)

func highlightJSON(code string) string {
	lines := strings.Split(code, "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(highlightJSONLine(line))
	}
	return sb.String()
}

func highlightJSONLine(line string) string {
	// Ключи
	result := reJSONKey.ReplaceAllStringFunc(line, func(s string) string {
		m := reJSONKey.FindStringSubmatch(s)
		if len(m) < 2 {
			return s
		}
		return hlAttr.Render(`"`+m[1]+`"`) + ":"
	})
	// Строковые значения
	result = regexp.MustCompile(`(:\s*)"([^"]*)"`).ReplaceAllStringFunc(result, func(s string) string {
		colonIdx := strings.Index(s, "\"")
		if colonIdx < 0 {
			return s
		}
		prefix := s[:colonIdx]
		val := s[colonIdx:]
		return prefix + hlString.Render(val)
	})
	// Числа и булевы
	result = reJSONBool.ReplaceAllStringFunc(result, func(s string) string {
		return hlKeyword.Render(s)
	})
	return result
}

// ── Утилиты ───────────────────────────────────────────────────────────────────

func toSet(words []string) map[string]bool {
	s := make(map[string]bool, len(words))
	for _, w := range words {
		s[w] = true
	}
	return s
}

func removeDuplicates(spans []span) []span {
	if len(spans) == 0 {
		return spans
	}
	for i := 0; i < len(spans); i++ {
		for j := i + 1; j < len(spans); j++ {
			if spans[j].start < spans[i].start {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}
	result := spans[:0]
	maxEnd := 0
	for _, sp := range spans {
		if sp.start >= maxEnd {
			result = append(result, sp)
			if sp.end > maxEnd {
				maxEnd = sp.end
			}
		}
	}
	return result
}
