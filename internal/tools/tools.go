package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result — результат выполнения инструмента
type Result struct {
	Output string
	Error  string
	OK     bool
}

func ok(output string) Result {
	return Result{Output: output, OK: true}
}

func fail(err string) Result {
	return Result{Error: err, OK: false}
}

// Executor выполняет инструменты с привязкой к рабочей директории
type Executor struct {
	WorkDir string
}

// New создаёт Executor
func New(workDir string) *Executor {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &Executor{WorkDir: workDir}
}

// resolvePath делает путь абсолютным относительно WorkDir
// и проверяет что он не вылезает за пределы WorkDir (path traversal защита)
func (e *Executor) resolvePath(p string) (string, error) {
	if filepath.IsAbs(p) {
		// Абсолютный путь — разрешаем только если внутри WorkDir
		clean := filepath.Clean(p)
		if !strings.HasPrefix(clean, e.WorkDir) {
			return "", fmt.Errorf("путь за пределами рабочей директории: %s", p)
		}
		return clean, nil
	}
	full := filepath.Clean(filepath.Join(e.WorkDir, p))
	if !strings.HasPrefix(full, e.WorkDir) {
		return "", fmt.Errorf("путь за пределами рабочей директории: %s", p)
	}
	return full, nil
}

// ReadFile читает файл и возвращает его содержимое
func (e *Executor) ReadFile(path string) Result {
	abs, err := e.resolvePath(path)
	if err != nil {
		return fail(err.Error())
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fail(fmt.Sprintf("чтение файла %s: %v", path, err))
	}

	// Ограничение: не более 100KB за раз
	const maxBytes = 100 * 1024
	if len(data) > maxBytes {
		data = append(data[:maxBytes], []byte("\n... [файл обрезан, >100KB]")...)
	}

	return ok(string(data))
}

// WriteFile записывает содержимое в файл (создаёт директории если нужно)
func (e *Executor) WriteFile(path, content string) Result {
	abs, err := e.resolvePath(path)
	if err != nil {
		return fail(err.Error())
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return fail(fmt.Sprintf("создание директории: %v", err))
	}

	if err := os.WriteFile(abs, []byte(content), 0640); err != nil {
		return fail(fmt.Sprintf("запись файла %s: %v", path, err))
	}

	return ok(fmt.Sprintf("файл записан: %s (%d байт)", path, len(content)))
}

// PatchFile заменяет oldStr на newStr в файле (первое вхождение)
func (e *Executor) PatchFile(path, oldStr, newStr string) Result {
	abs, err := e.resolvePath(path)
	if err != nil {
		return fail(err.Error())
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fail(fmt.Sprintf("чтение файла для патча: %v", err))
	}

	content := string(data)
	if !strings.Contains(content, oldStr) {
		return fail(fmt.Sprintf("строка для замены не найдена в %s", path))
	}

	// Заменяем только первое вхождение
	patched := strings.Replace(content, oldStr, newStr, 1)

	if err := os.WriteFile(abs, []byte(patched), 0640); err != nil {
		return fail(fmt.Sprintf("запись патча: %v", err))
	}

	return ok(fmt.Sprintf("патч применён в %s", path))
}

// ListFiles возвращает дерево файлов (рекурсивно, макс глубина 4)
func (e *Executor) ListFiles(subPath string) Result {
	base := e.WorkDir
	if subPath != "" && subPath != "." {
		var err error
		base, err = e.resolvePath(subPath)
		if err != nil {
			return fail(err.Error())
		}
	}

	var sb strings.Builder
	err := walkTree(base, base, 0, 4, &sb)
	if err != nil {
		return fail(fmt.Sprintf("обход директории: %v", err))
	}

	if sb.Len() == 0 {
		return ok("(директория пуста)")
	}
	return ok(sb.String())
}

// walkTree рекурсивно строит дерево файлов
func walkTree(root, dir string, depth, maxDepth int, sb *strings.Builder) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	indent := strings.Repeat("  ", depth)
	for _, entry := range entries {
		// Пропускаем скрытые и типичные шумовые директории
		name := entry.Name()
		if strings.HasPrefix(name, ".") ||
			name == "node_modules" ||
			name == "__pycache__" ||
			name == "vendor" ||
			name == "target" {
			continue
		}

		if entry.IsDir() {
			fmt.Fprintf(sb, "%s📁 %s/\n", indent, name)
			subDir := filepath.Join(dir, name)
			_ = walkTree(root, subDir, depth+1, maxDepth, sb)
		} else {
			fmt.Fprintf(sb, "%s📄 %s\n", indent, name)
		}
	}
	return nil
}

// RunCommand выполняет shell-команду в рабочей директории
// Только безопасные команды (без rm -rf /, sudo и т.д.)
func (e *Executor) RunCommand(command string) Result {
	// Минимальная защита от деструктивных команд
	dangerous := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "fork bomb"}
	lower := strings.ToLower(command)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return fail(fmt.Sprintf("команда заблокирована по соображениям безопасности: %s", d))
		}
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = e.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Таймаут не через context чтобы не тянуть лишний импорт —
	// пользователь может прервать через Ctrl+C в TUI
	err := cmd.Run()

	var sb strings.Builder
	if stdout.Len() > 0 {
		sb.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n[stderr]\n")
		}
		sb.WriteString(stderr.String())
	}

	output := sb.String()
	// Ограничение вывода
	const maxOut = 8000
	if len(output) > maxOut {
		output = output[:maxOut] + "\n... [вывод обрезан]"
	}

	if err != nil {
		if output != "" {
			return Result{Output: output, Error: err.Error(), OK: false}
		}
		return fail(err.Error())
	}

	return ok(output)
}

// Dispatch вызывает нужный инструмент по имени с параметрами
func (e *Executor) Dispatch(name string, params map[string]string) Result {
	switch name {
	case "read_file":
		path, ok := params["path"]
		if !ok || path == "" {
			return fail("read_file: нужен параметр 'path'")
		}
		return e.ReadFile(path)

	case "write_file":
		path, ok1 := params["path"]
		content, ok2 := params["content"]
		if !ok1 || !ok2 {
			return fail("write_file: нужны параметры 'path' и 'content'")
		}
		return e.WriteFile(path, content)

	case "patch_file":
		path := params["path"]
		oldStr := params["old_str"]
		newStr := params["new_str"]
		if path == "" || oldStr == "" {
			return fail("patch_file: нужны 'path', 'old_str', 'new_str'")
		}
		return e.PatchFile(path, oldStr, newStr)

	case "list_files":
		return e.ListFiles(params["path"])

	case "run_command":
		cmd, ok := params["command"]
		if !ok || cmd == "" {
			return fail("run_command: нужен параметр 'command'")
		}
		return e.RunCommand(cmd)

	default:
		return fail(fmt.Sprintf("неизвестный инструмент: %s", name))
	}
}

// ToolDefs возвращает описание инструментов для системного промпта
func ToolDefs() string {
	return `## Tools

You have access to tools. To call a tool, output ONLY this exact format in your response:

` + "```" + `tool
{"tool": "tool_name", "params": {"key": "value"}}
` + "```" + `

IMPORTANT RULES:
- Use EXACTLY the ` + "```tool" + ` format shown above — no other format
- Do NOT write [tool:name] or {"action": ...} formats
- Do NOT explain the tool call, just output the block
- After the tool result is shown, continue your response normally
- Call ONE tool at a time

### read_file
Read a file. Params: path (relative to project root)
Example:
` + "```" + `tool
{"tool": "read_file", "params": {"path": "main.go"}}
` + "```" + `

### write_file
Write entire file content. Params: path, content
Example:
` + "```" + `tool
{"tool": "write_file", "params": {"path": "hello.go", "content": "package main\n"}}
` + "```" + `

### patch_file
Replace first occurrence of old_str with new_str in file. Params: path, old_str, new_str
Prefer this over write_file for small changes.
Example:
` + "```" + `tool
{"tool": "patch_file", "params": {"path": "main.go", "old_str": "v0.1.0", "new_str": "v0.2.0"}}
` + "```" + `

### list_files
Show project file tree. Params: path (optional, default = root)
Example:
` + "```" + `tool
{"tool": "list_files", "params": {}}
` + "```" + `

### run_command
Run a shell command. Params: command
Example:
` + "```" + `tool
{"tool": "run_command", "params": {"command": "go build ./..."}}
` + "```" + `
`
}
