package tui

import (
	"fmt"
	"strings"

	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/charmbracelet/lipgloss"
)

// renderMessages — рендер всей истории чата + текущего стрима.
func (m Model) renderMessages() string {
	if len(m.sess.Messages) == 0 && m.streaming == "" {
		welcome := lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(2).
			Render(m.tr().WelcomeMsg)
		return welcome
	}

	var sb strings.Builder
	contentWidth := m.width - 4

	for msgIdx, msg := range m.sess.Messages {
		switch msg.Role {
		case session.RoleUser:
			sb.WriteString(userLabelStyle.Render("▶ "+m.tr().UserLabel) + "\n")
			sb.WriteString(userBubbleStyle.Width(contentWidth).Render(msg.Content))
			sb.WriteString("\n\n")

		case session.RoleAssistant:
			sb.WriteString(assistantLabelStyle.Render("◆ TermCode") + "\n")
			rendered := m.renderAssistantContent(msgIdx, msg.Content, contentWidth)
			sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(rendered))
			for _, tc := range msg.ToolCalls {
				sb.WriteString(renderToolCall(tc, contentWidth))
			}
			sb.WriteString("\n\n")

		case session.RoleTool:
			// результаты tool calls уже показаны в сообщении ассистента
		}
	}

	// Текущий стриминг
	if m.streaming != "" {
		sb.WriteString(assistantLabelStyle.Render("◆ TermCode") + "\n")

		displayText := m.streaming
		if m.displayedLen > 0 {
			runes := []rune(m.streaming)
			if m.displayedLen < len(runes) {
				displayText = string(runes[:m.displayedLen])
			}
		}

		inThink := isInsideThink(displayText)
		visible := filterThinkTags(displayText)

		if inThink {
			thinkIndicator := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5C6370")).Italic(true).
				Render(m.tr().Thinking)
			if visible != "" {
				rendered := renderMarkdown(visible, contentWidth)
				sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(thinkIndicator + "\n\n" + rendered))
			} else {
				sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(thinkIndicator))
			}
		} else {
			rendered := renderMarkdown(visible, contentWidth)
			sb.WriteString(assistantBubbleStyle.Width(contentWidth).Render(rendered))
		}
		sb.WriteString(" ▋\n")
	}

	return sb.String()
}

// renderToolCall — карточка вызова инструмента с результатом.
func renderToolCall(tc session.ToolCall, width int) string {
	var sb strings.Builder

	params := make([]string, 0, len(tc.Params))
	for k, v := range tc.Params {
		short := v
		if len(short) > 40 {
			short = short[:37] + "..."
		}
		params = append(params, k+"="+short)
	}
	header := fmt.Sprintf("⚡ %s(%s)", tc.Name, strings.Join(params, ", "))
	sb.WriteString(toolCallStyle.Width(width - 2).Render(header))
	sb.WriteString("\n")

	if tc.Error != "" {
		errShort := tc.Error
		if len(errShort) > 200 {
			errShort = errShort[:197] + "..."
		}
		sb.WriteString(toolErrorStyle.Width(width - 2).Render("✗ " + errShort))
	} else if tc.Result != "" {
		resultShort := tc.Result
		if len(resultShort) > 500 {
			resultShort = resultShort[:497] + "..."
		}
		sb.WriteString(toolResultStyle.Width(width - 2).Render(resultShort))
	}
	sb.WriteString("\n")
	return sb.String()
}

// renderMarkdown — минимальный markdown-рендер: код-блоки с подсветкой, заголовки, жирный.
func renderMarkdown(text string, width int) string {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	inCodeBlock := false
	var codeLines []string
	codeLang := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				code := strings.Join(codeLines, "\n")
				label := ""
				if codeLang != "" {
					label = lipgloss.NewStyle().Foreground(colorMuted).Render(" "+codeLang+" ") + "\n"
				}
				highlighted := HighlightCode(code, codeLang)
				sb.WriteString(codeBlockStyle.Width(width - 4).Render(label + highlighted))
				sb.WriteString("\n")
				inCodeBlock = false
				codeLines = nil
				codeLang = ""
			} else {
				inCodeBlock = true
				codeLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Заголовки
		if strings.HasPrefix(line, "### ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).Render(line[4:]))
			sb.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(line[3:]))
			sb.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(line[2:]))
			sb.WriteString("\n")
			continue
		}

		// Жирный **text**
		line = renderInlineBold(line)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Незакрытый код-блок
	if inCodeBlock && len(codeLines) > 0 {
		highlighted := HighlightCode(strings.Join(codeLines, "\n"), codeLang)
		sb.WriteString(codeBlockStyle.Width(width - 4).Render(highlighted))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderInlineBold(line string) string {
	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	result := line
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end = start + 2 + end
		bold := boldStyle.Render(result[start+2 : end])
		result = result[:start] + bold + result[end+2:]
	}
	return result
}
