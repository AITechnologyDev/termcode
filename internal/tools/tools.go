package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	// Нормализуем WorkDir с trailing slash чтобы избежать
	// /workdir2 матча при проверке HasPrefix("/workdir")
	safeRoot := filepath.Clean(e.WorkDir) + string(filepath.Separator)

	if filepath.IsAbs(p) {
		clean := filepath.Clean(p) + string(filepath.Separator)
		if !strings.HasPrefix(clean, safeRoot) {
			return "", fmt.Errorf("путь за пределами рабочей директории: %s", p)
		}
		return filepath.Clean(p), nil
	}
	full := filepath.Clean(filepath.Join(e.WorkDir, p))
	if !strings.HasPrefix(full+string(filepath.Separator), safeRoot) {
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

	patched := strings.Replace(content, oldStr, newStr, 1)

	if err := os.WriteFile(abs, []byte(patched), 0640); err != nil {
		return fail(fmt.Sprintf("запись патча: %v", err))
	}

	// Считаем статистику изменений
	diffStat := diffStats(path, oldStr, newStr)
	return ok(fmt.Sprintf("патч применён в %s\n%s", path, diffStat))
}

// diffStats генерирует компактную diff-статистику в стиле git
func diffStats(path, oldStr, newStr string) string {
	oldLines := strings.Split(strings.TrimRight(oldStr, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(newStr, "\n"), "\n")

	removed := 0
	added := 0

	// Простой подсчёт: строки только в old = удалены, только в new = добавлены
	oldSet := make(map[string]int)
	for _, l := range oldLines {
		oldSet[l]++
	}
	newSet := make(map[string]int)
	for _, l := range newLines {
		newSet[l]++
	}
	for l, cnt := range oldSet {
		diff := cnt - newSet[l]
		if diff > 0 {
			removed += diff
		}
	}
	for l, cnt := range newSet {
		diff := cnt - oldSet[l]
		if diff > 0 {
			added += diff
		}
	}

	// Визуальная полоска как в git --stat
	total := added + removed
	barWidth := 20
	addBars := 0
	delBars := 0
	if total > 0 {
		addBars = added * barWidth / total
		delBars = removed * barWidth / total
		if addBars+delBars < barWidth && (added > 0 || removed > 0) {
			// Минимум одна полоска для ненулевого значения
			if added > 0 && addBars == 0 {
				addBars = 1
			}
			if removed > 0 && delBars == 0 {
				delBars = 1
			}
		}
	}

	bar := strings.Repeat("+", addBars) + strings.Repeat("-", delBars)
	return fmt.Sprintf("%s | +%d -%d %s", path, added, removed, bar)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
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
	if ctx.Err() == context.DeadlineExceeded {
		output += "\n[таймаут: команда выполнялась более 30 секунд]"
	}
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

	case "download_file":
		url, ok := params["url"]
		if !ok || url == "" {
			return fail("download_file: нужен параметр 'url'")
		}
		dest := params["path"]
		return e.DownloadFile(url, dest)

	case "web_search":
		query, ok := params["query"]
		if !ok || query == "" {
			return fail("web_search: нужен параметр 'query'")
		}
		maxResults := 5
		return WebSearch(query, maxResults)

	case "fetch_page":
		url, ok := params["url"]
		if !ok || url == "" {
			return fail("fetch_page: нужен параметр 'url'")
		}
		return FetchPage(url)

	default:
		return fail(fmt.Sprintf("неизвестный инструмент: %s", name))
	}
}

// DownloadFile скачивает файл по URL и сохраняет в рабочую директорию
func (e *Executor) DownloadFile(rawURL, destPath string) Result {
	// Базовая проверка URL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fail("download_file: поддерживаются только http/https URL")
	}

	// Если путь не указан — берём имя файла из URL
	if destPath == "" {
		parts := strings.Split(rawURL, "/")
		destPath = parts[len(parts)-1]
		if destPath == "" || strings.Contains(destPath, "?") {
			destPath = "downloaded_file"
		}
		// Убираем query string
		if idx := strings.Index(destPath, "?"); idx >= 0 {
			destPath = destPath[:idx]
		}
	}

	abs, err := e.resolvePath(destPath)
	if err != nil {
		return fail(err.Error())
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return fail(fmt.Sprintf("создание директории: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fail(fmt.Sprintf("ошибка запроса: %v", err))
	}
	req.Header.Set("User-Agent", "TermCode/0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail(fmt.Sprintf("ошибка загрузки: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("сервер вернул %d: %s", resp.StatusCode, rawURL))
	}

	// Ограничение 50 MB
	const maxSize = 50 * 1024 * 1024
	limited := io.LimitReader(resp.Body, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fail(fmt.Sprintf("ошибка чтения: %v", err))
	}
	if len(data) > maxSize {
		return fail("файл превышает лимит 50 MB")
	}

	if err := os.WriteFile(abs, data, 0640); err != nil {
		return fail(fmt.Sprintf("ошибка записи: %v", err))
	}

	return ok(fmt.Sprintf("скачано %d байт → %s", len(data), destPath))
}

// ── Веб-поиск ─────────────────────────────────────────────────────────────────

// ddgResult — один результат поиска DuckDuckGo
type ddgResult struct {
	Title   string `json:"Text"`
	URL     string `json:"FirstURL"`
	// для RelatedTopics которые вложены
	Topics []ddgResult `json:"Topics,omitempty"`
}

// ddgResponse — ответ DDG Instant Answer API
type ddgResponse struct {
	AbstractText   string       `json:"AbstractText"`
	AbstractURL    string       `json:"AbstractURL"`
	AbstractSource string       `json:"AbstractSource"`
	RelatedTopics  []ddgResult  `json:"RelatedTopics"`
	Results        []ddgResult  `json:"Results"`
	Answer         string       `json:"Answer"`
	AnswerType     string       `json:"AnswerType"`
}

// WebSearch выполняет поиск через DuckDuckGo Instant Answer API + HTML scrape fallback
func WebSearch(query string, maxResults int) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Сначала пробуем DDG Instant Answer API (JSON, без ключа)
	apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) +
		"&format=json&no_html=1&skip_disambig=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fail("web_search: " + err.Error())
	}
	req.Header.Set("User-Agent", "TermCode/0.1 (terminal AI assistant)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("web_search: ошибка запроса: " + err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return fail("web_search: ошибка чтения: " + err.Error())
	}

	var ddg ddgResponse
	if err := json.Unmarshal(body, &ddg); err != nil {
		return fail("web_search: ошибка парсинга: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Результаты поиска: %q\n\n", query))

	count := 0

	// 1. Прямой ответ (калькулятор, конвертер и т.д.)
	if ddg.Answer != "" {
		sb.WriteString(fmt.Sprintf("💡 Ответ: %s\n\n", ddg.Answer))
		count++
	}

	// 2. Краткая выжимка (Wikipedia и т.д.)
	if ddg.AbstractText != "" {
		src := ddg.AbstractSource
		if ddg.AbstractURL != "" {
			src = fmt.Sprintf("%s (%s)", src, ddg.AbstractURL)
		}
		sb.WriteString(fmt.Sprintf("📖 %s\n   Источник: %s\n\n", ddg.AbstractText, src))
		count++
	}

	// 3. Прямые результаты
	for _, r := range ddg.Results {
		if count >= maxResults {
			break
		}
		if r.Title == "" || r.URL == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("• %s\n  %s\n\n", r.Title, r.URL))
		count++
	}

	// 4. Связанные темы
	for _, r := range ddg.RelatedTopics {
		if count >= maxResults {
			break
		}
		// Вложенные группы
		if len(r.Topics) > 0 {
			for _, t := range r.Topics {
				if count >= maxResults {
					break
				}
				if t.Title == "" || t.URL == "" {
					continue
				}
				sb.WriteString(fmt.Sprintf("• %s\n  %s\n\n", t.Title, t.URL))
				count++
			}
			continue
		}
		if r.Title == "" || r.URL == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("• %s\n  %s\n\n", r.Title, r.URL))
		count++
	}

	if count == 0 {
		// DDG API дал пустой ответ — fallback: HTML поиск через Lite версию
		return webSearchLite(query, maxResults)
	}

	sb.WriteString(fmt.Sprintf("---\n%d результатов. Используй fetch_page для чтения страницы.", count))
	return ok(sb.String())
}

// webSearchLite — fallback через DDG Lite (HTML scraping без JS)
func webSearchLite(query string, maxResults int) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	liteURL := "https://lite.duckduckgo.com/lite/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, liteURL, nil)
	if err != nil {
		return fail("web_search fallback: " + err.Error())
	}
	req.Header.Set("User-Agent", "TermCode/0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("web_search fallback: " + err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return fail("web_search fallback: " + err.Error())
	}

	// Простой парсинг HTML — ищем ссылки результатов
	html := string(body)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Результаты поиска: %q\n\n", query))

	count := 0
	// DDG lite результаты имеют паттерн: href="//duckduckgo.com/l/?uddg=URL"
	// или просто прямые ссылки в <a class="result-link">
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if count >= maxResults {
			break
		}
		line = strings.TrimSpace(line)
		// Ищем строки с uddg= (encoded result URLs)
		if strings.Contains(line, "uddg=") {
			start := strings.Index(line, "uddg=")
			if start < 0 {
				continue
			}
			rawURL := line[start+5:]
			// Декодируем до следующего &amp; или "
			end := strings.IndexAny(rawURL, "\"&>")
			if end > 0 {
				rawURL = rawURL[:end]
			}
			decoded, err2 := url.QueryUnescape(rawURL)
			if err2 != nil || decoded == "" {
				continue
			}
			if !strings.HasPrefix(decoded, "http") {
				continue
			}
			sb.WriteString(fmt.Sprintf("• %s\n\n", decoded))
			count++
		}
	}

	if count == 0 {
		return fail(fmt.Sprintf(
			"web_search: DDG не вернул результатов для %q. Попробуй другой запрос.", query))
	}

	sb.WriteString(fmt.Sprintf("---\n%d результатов. Используй fetch_page для чтения.", count))
	return ok(sb.String())
}

// FetchPage скачивает страницу и извлекает читаемый текст (без HTML тегов)
func FetchPage(rawURL string) Result {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fail("fetch_page: поддерживаются только http/https")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fail("fetch_page: " + err.Error())
	}
	req.Header.Set("User-Agent", "TermCode/0.1 (terminal AI assistant)")
	req.Header.Set("Accept", "text/html,text/plain;q=0.9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("fetch_page: ошибка запроса: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("fetch_page: сервер вернул %d", resp.StatusCode))
	}

	// Ограничиваем 512KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return fail("fetch_page: ошибка чтения: " + err.Error())
	}

	// Убираем HTML теги — простой стриппер без зависимостей
	text := stripHTML(string(body))

	// Ограничиваем вывод до 8000 символов
	const maxOut = 8000
	if len(text) > maxOut {
		text = text[:maxOut] + "\n\n... [страница обрезана, первые 8000 символов]"
	}

	if strings.TrimSpace(text) == "" {
		return fail("fetch_page: страница пуста или не содержит текста")
	}

	return ok(fmt.Sprintf("📄 %s\n\n%s", rawURL, text))
}

// stripHTML убирает HTML теги и нормализует пробелы
func stripHTML(html string) string {
	var sb strings.Builder
	inTag := false
	inScript := false
	inStyle := false

	lower := strings.ToLower(html)
	i := 0
	for i < len(html) {
		// Пропускаем <script>...</script>
		if !inTag && i+7 <= len(lower) && lower[i:i+7] == "<script" {
			end := strings.Index(lower[i:], "</script>")
			if end >= 0 {
				i += end + 9
				inScript = false
				continue
			}
			inScript = true
		}
		if inScript {
			i++
			continue
		}
		// Пропускаем <style>...</style>
		if !inTag && i+6 <= len(lower) && lower[i:i+6] == "<style" {
			end := strings.Index(lower[i:], "</style>")
			if end >= 0 {
				i += end + 8
				inStyle = false
				continue
			}
			inStyle = true
		}
		if inStyle {
			i++
			continue
		}

		c := html[i]
		if c == '<' {
			inTag = true
			// Заменяем блочные теги на перенос строки
			if i+2 < len(lower) {
				tag := lower[i+1:]
				if strings.HasPrefix(tag, "p") || strings.HasPrefix(tag, "br") ||
					strings.HasPrefix(tag, "div") || strings.HasPrefix(tag, "h") ||
					strings.HasPrefix(tag, "li") || strings.HasPrefix(tag, "tr") ||
					strings.HasPrefix(tag, "/p") || strings.HasPrefix(tag, "/div") {
					sb.WriteByte('\n')
				}
			}
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			// Декодируем базовые HTML entities
			if c == '&' && i+3 < len(html) {
				rest := html[i:]
				switch {
				case strings.HasPrefix(rest, "&amp;"):
					sb.WriteByte('&')
					i += 5
					continue
				case strings.HasPrefix(rest, "&lt;"):
					sb.WriteByte('<')
					i += 4
					continue
				case strings.HasPrefix(rest, "&gt;"):
					sb.WriteByte('>')
					i += 4
					continue
				case strings.HasPrefix(rest, "&nbsp;"):
					sb.WriteByte(' ')
					i += 6
					continue
				case strings.HasPrefix(rest, "&#"):
					// Пропускаем числовые entities
					end := strings.Index(rest, ";")
					if end > 0 && end < 8 {
						i += end + 1
						continue
					}
				}
			}
			sb.WriteByte(c)
		}
		i++
	}

	// Нормализуем пробелы и пустые строки
	lines := strings.Split(sb.String(), "\n")
	var result []string
	prevEmpty := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !prevEmpty {
				result = append(result, "")
			}
			prevEmpty = true
		} else {
			result = append(result, line)
			prevEmpty = false
		}
	}
	return strings.Join(result, "\n")
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

### download_file
Download a file from the internet. Params: url (required), path (optional save location)
Example:
` + "```" + `tool
{"tool": "download_file", "params": {"url": "https://example.com/asset.png", "path": "assets/asset.png"}}
` + "```" + `

### web_search
Search the web using DuckDuckGo. Returns top results with titles and URLs.
Params: query (search query string)
Use this to find documentation, examples, answers to errors, latest API info.
Example:
` + "```" + `tool
{"tool": "web_search", "params": {"query": "mindustry mod javascript API blocks"}}
` + "```" + `

### fetch_page
Fetch and read a web page as plain text. Use after web_search to read full content.
Params: url (full URL to fetch)
Example:
` + "```" + `tool
{"tool": "fetch_page", "params": {"url": "https://github.com/Anuken/Mindustry/wiki/Modding"}}
` + "```" + `
`
}
