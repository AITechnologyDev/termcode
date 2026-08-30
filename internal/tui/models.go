package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Ollama: список моделей и автодетект контекста ──────────────────────

func fetchContextLength(baseURL, model string) tea.Cmd {
	return func() tea.Msg {
		limits, err := ai.FetchOllamaModelLimits(baseURL, model)
		return contextDetectedMsg{
			contextLength:   limits.ContextLength,
			maxOutputTokens: limits.MaxOutputTokens,
			err:             err,
		}
	}
}

func fetchOllamaModels(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		pc, ok := cfg.ActiveProviderConfig()
		if !ok {
			return ollamaModelsMsg{err: fmt.Errorf("provider config not found")}
		}

		baseURL := strings.TrimRight(pc.BaseURL, "/")
		tagsURL := baseURL + "/tags"
		if !strings.Contains(baseURL, "/api") {
			tagsURL = baseURL + "/api/tags"
		}

		req, err := http.NewRequest("GET", tagsURL, nil)
		if err != nil {
			return ollamaModelsMsg{err: err}
		}
		if pc.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+pc.APIKey)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return ollamaModelsMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return ollamaModelsMsg{err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))}
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return ollamaModelsMsg{err: err}
		}

		type ollamaModel struct {
			Name string `json:"name"`
		}
		type ollamaTagsResp struct {
			Models []ollamaModel `json:"models"`
		}

		var tagsResp ollamaTagsResp
		if err := json.Unmarshal(data, &tagsResp); err != nil {
			return ollamaModelsMsg{err: err}
		}

		names := make([]string, 0, len(tagsResp.Models))
		for _, m := range tagsResp.Models {
			names = append(names, m.Name)
		}

		if len(names) == 0 {
			return ollamaModelsMsg{err: fmt.Errorf("no models found (check API key and plan)")}
		}

		return ollamaModelsMsg{models: names}
	}
}

func (m Model) selectModel(name string) (tea.Model, tea.Cmd) {
	pc := m.cfg.Providers[m.cfg.ActiveProvider]
	pc.Model = name
	pc.ContextLength = 0
	m.cfg.Providers[m.cfg.ActiveProvider] = pc
	_ = m.cfg.Save()

	provider, err := ai.New(pc, m.cfg.ActiveProvider)
	if err != nil {
		m.errMsg = "Model switch error: " + err.Error()
		m.currentState = stateChat
		return m, nil
	}

	m.provider = provider
	m.sess.Model = name
	m.currentState = stateChat
	m.refreshViewport()

	if m.cfg.ActiveProvider == config.ProviderOllama {
		return m, fetchContextLength(pc.BaseURL, name)
	}
	return m, nil
}

// ── Ollama pull ─────────────────────────────────────────────────────────

func (m Model) startPull(modelName string) (tea.Model, tea.Cmd) {
	if modelName == "" {
		m.errMsg = "Enter model name: /pull qwen2.5-coder:7b"
		return m, nil
	}

	m.currentState = statePulling
	m.pullModelName = modelName
	m.pullStatus = "Connecting..."
	m.pullCompleted = 0
	m.pullTotal = 0

	pc, _ := m.cfg.ActiveProviderConfig()
	baseURL := strings.TrimRight(pc.BaseURL, "/")

	return m, tea.Batch(m.spinner.Tick, streamOllamaPull(baseURL, modelName, m.tr().PullDone))
}

func streamOllamaPull(baseURL, modelName, pullDoneStr string) tea.Cmd {
	return func() tea.Msg {
		url := baseURL + "/api/pull"
		body := fmt.Sprintf(`{"name":%q,"stream":true}`, modelName)
		resp, err := httpPostStream(url, body)
		if err != nil {
			return pullProgressMsg{err: err}
		}
		defer resp.Close()

		scanner := newLineScanner(resp)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			status, completed, total, done, parseErr := parsePullLine(line)
			if parseErr != nil {
				continue
			}
			if done {
				return pullProgressMsg{status: pullDoneStr, done: true}
			}
			return pullProgressMsg{
				status:    status,
				completed: completed,
				total:     total,
				done:      false,
			}
		}
		return pullProgressMsg{done: true}
	}
}

// ── HTTP хелперы ────────────────────────────────────────────────────────

func httpGetJSON(url string) ([]byte, error) {
	resp, err := httpDo("GET", url, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func httpPostStream(url, body string) (io.ReadCloser, error) {
	resp, err := httpDo("POST", url, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func httpDo(method, url, body string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 0}
	return client.Do(req)
}

func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 64*1024)
	return s
}

func parsePullLine(line string) (status string, completed, total int64, done bool, err error) {
	var obj struct {
		Status    string `json:"status"`
		Completed int64  `json:"completed"`
		Total     int64  `json:"total"`
	}
	if err = json.Unmarshal([]byte(line), &obj); err != nil {
		return
	}
	status = obj.Status
	completed = obj.Completed
	total = obj.Total
	done = obj.Status == "success" || strings.Contains(line, `"done":true`)
	return
}

// ── Рендер экранов: выбора модели, pull ─────────────────────────────────

func (m Model) renderModelSelect() string {
	t := m.tr()
	w := m.width
	if w < 20 {
		w = 20
	}

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(t.ModelSelectTitle) + "\n\n")

	if m.modelsLoading {
		sb.WriteString(fmt.Sprintf("  %s Loading Ollama models...\n", m.spinner.View()))
		return sb.String()
	}

	if len(m.ollamaModels) == 0 {
		sb.WriteString(statusErrStyle.Render("  Ollama unavailable or no models.") + "\n\n")
		sb.WriteString(keyHintStyle.Render("  Run: ollama serve\n"))
		sb.WriteString(keyHintStyle.Render("  Pull a model: /pull qwen2.5-coder:7b\n\n"))
		sb.WriteString(keyStyle.Render("  q") + keyHintStyle.Render(" — continue without selecting\n"))
		return sb.String()
	}

	sb.WriteString(keyHintStyle.Render(t.ModelSelectHint))

	pc, _ := m.cfg.ActiveProviderConfig()

	maxVisible := m.height - 10
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := 0
	if m.modelCursor >= maxVisible {
		start = m.modelCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.ollamaModels) {
		end = len(m.ollamaModels)
	}

	// Рамка
	boxW := w - 4
	if boxW < 20 {
		boxW = 20
	}
	primary := lipgloss.NewStyle().Foreground(colorPrimary)
	muted := lipgloss.NewStyle().Foreground(colorMuted)
	hlStyle := lipgloss.NewStyle().
		Background(colorPrimary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	sb.WriteString("  " + muted.Render("Available models") + "\n")
	sb.WriteString(primary.Render("  ╭"+strings.Repeat("─", boxW)+"╮") + "\n")

	for i := start; i < end; i++ {
		model := m.ollamaModels[i]

		// Тег: облако / локально
		tag := "local"
		tagStyle := tagLocalStyle
		if strings.HasSuffix(model, ":cloud") {
			tag = "cloud"
			tagStyle = tagCloudStyle
		} else if strings.Contains(model, "free") || strings.HasSuffix(model, ":free") {
			tag = "free"
			tagStyle = tagFreeStyle
		}
		tagStr := tagStyle.Render(" " + tag + " ")

		active := ""
		if model == pc.Model {
			active = " ✓"
		}

		prefix := "  "
		if i == m.modelCursor {
			prefix = "▶ "
		}

		// Сборка строки
		inner := fmt.Sprintf("%s%s  %s%s", prefix, tagStr, model, active)
		innerRunes := []rune(inner)
		if len(innerRunes) > boxW {
			// truncate model name
			modelRunes := []rune(model)
			tagRunes := []rune(tagStr)
			prefixRunes := []rune(prefix)
			tail := []rune(active)
			maxM := boxW - len(prefixRunes) - len(tagRunes) - len(tail) - 2
			if maxM < 1 {
				maxM = 1
			}
			trunc := string(modelRunes[:maxM]) + "…"
			inner = string(prefixRunes) + string(tagRunes) + "  " + trunc + string(tail)
			innerRunes = []rune(inner)
		}
		for len(innerRunes) < boxW {
			innerRunes = append(innerRunes, ' ')
		}

		if i == m.modelCursor {
			sb.WriteString(primary.Render("  │") + hlStyle.Render(string(innerRunes)) + primary.Render("│") + "\n")
		} else {
			sb.WriteString(primary.Render("  │") + string(innerRunes) + primary.Render("│") + "\n")
		}
	}

	sb.WriteString(primary.Render("  ╰"+strings.Repeat("─", boxW)+"╯") + "\n\n")
	sb.WriteString(keyHintStyle.Render(fmt.Sprintf(t.ModelSelectCount, m.modelCursor+1, len(m.ollamaModels))))
	return sb.String()
}

func (m Model) renderPullScreen() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(m.tr().PullTitle) + "\n\n")
	model := assistantLabelStyle.Render(m.pullModelName)
	sb.WriteString(fmt.Sprintf("  Downloading: %s\n\n", model))
	sb.WriteString(fmt.Sprintf("  Status: %s\n\n", m.pullStatus))

	if m.pullTotal > 0 {
		pct := float64(m.pullCompleted) / float64(m.pullTotal)
		barWidth := m.width - 12
		if barWidth < 10 {
			barWidth = 10
		}
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		completedMB := float64(m.pullCompleted) / 1024 / 1024
		totalMB := float64(m.pullTotal) / 1024 / 1024
		sb.WriteString(fmt.Sprintf("  [%s] %.0f%%\n", bar, pct*100))
		sb.WriteString(fmt.Sprintf("  %.1f MB / %.1f MB\n", completedMB, totalMB))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Downloading...\n", m.spinner.View()))
	}
	sb.WriteString("\n")
	sb.WriteString(keyStyle.Render("  Ctrl+C") + keyHintStyle.Render(m.tr().PullInterrupt))
	return sb.String()
}
