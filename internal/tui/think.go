package tui

import (
	"strings"

	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/charmbracelet/lipgloss"
)

// isInsideThink — true, если текст заканчивается внутри открытого <think> блока.
func isInsideThink(text string) bool {
	openCount := strings.Count(text, "<think>")
	closeCount := strings.Count(text, "</think>")
	return openCount > closeCount
}

// filterThinkTags — убирает <think>...</think> и одиночные теги.
func filterThinkTags(text string) string {
	result := text
	for {
		start := strings.Index(result, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(result, "</think>")
		if end == -1 {
			result = strings.TrimSpace(result[:start])
			break
		}
		result = result[:start] + result[end+len("</think>"):]
	}
	result = strings.ReplaceAll(result, "</think>", "")
	result = strings.ReplaceAll(result, "<think>", "")
	return strings.TrimSpace(result)
}

// extractThinkContent — извлекает первый <think>...</think> блок.
func extractThinkContent(text string) string {
	start := strings.Index(text, "<think>")
	if start == -1 {
		return ""
	}
	inner := text[start+len("<think>"):]
	end := strings.Index(inner, "</think>")
	if end == -1 {
		return strings.TrimSpace(inner)
	}
	content := strings.TrimSpace(inner[:end])
	content = strings.ReplaceAll(content, "</think>", "")
	content = strings.ReplaceAll(content, "<think>", "")
	return content
}

// replaceThinkForSession — оборачивает think в маркер <!--think:...--> для сессии.
func replaceThinkForSession(rawText, cleanText string) string {
	if !strings.Contains(rawText, "<think>") {
		return cleanText
	}
	think := extractThinkContent(rawText)
	if think == "" {
		return cleanText
	}
	if strings.TrimSpace(cleanText) == "" {
		return "<!--think:" + think + "-->"
	}
	return "<!--think:" + think + "-->\n" + cleanText
}

// renderAssistantContent — рендер сообщения ассистента с раскрывающимся think-блоком.
func (m Model) renderAssistantContent(msgIdx int, content string, width int) string {
	const thinkPrefix = "<!--think:"
	const thinkSuffix = "-->"

	if !strings.HasPrefix(content, thinkPrefix) {
		return renderMarkdown(content, width)
	}

	rest := content[len(thinkPrefix):]
	endIdx := strings.Index(rest, thinkSuffix)
	if endIdx < 0 {
		return renderMarkdown(content, width)
	}
	thinkContent := rest[:endIdx]
	visible := strings.TrimPrefix(rest[endIdx+len(thinkSuffix):], "\n")

	expanded := m.thinkExpanded[msgIdx]
	var thinkHeader string
	if expanded {
		thinkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).Italic(true).
			Render("Thinking  [T — hide]")
	} else {
		thinkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).Italic(true).
			Render("Thinking  [T — show]")
	}

	var sb strings.Builder
	sb.WriteString(thinkHeader + "\n")

	if expanded && thinkContent != "" {
		thinkStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#636D83")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#3E4451")).
			PaddingLeft(1)
		display := thinkContent
		if len(display) > 2000 {
			display = display[:1997] + "..."
		}
		sb.WriteString(thinkStyle.Width(width - 4).Render(display))
		sb.WriteString("\n")
	}

	if visible != "" {
		sb.WriteString("\n")
		sb.WriteString(renderMarkdown(visible, width))
	}

	return sb.String()
}

// toggleLastThink — переключает видимость think-блока последнего сообщения.
func (m *Model) toggleLastThink() Model {
	for i := len(m.sess.Messages) - 1; i >= 0; i-- {
		msg := m.sess.Messages[i]
		if msg.Role == session.RoleAssistant && strings.HasPrefix(msg.Content, "<!--think:") {
			m.thinkExpanded[i] = !m.thinkExpanded[i]
			m.refreshViewport()
			return *m
		}
	}
	return *m
}
