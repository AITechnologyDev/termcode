package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/NekoFemDev/termcode/internal/session"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderProfileEdit — экран редактирования профиля пользователя.
func (m Model) renderProfileEdit() string {
	t := m.tr()
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(t.ProfileTitle) + "\n\n")
	sb.WriteString(keyHintStyle.Render("  Tell AI who you are — name, role, expertise.\n  This is added to every conversation.\n\n"))
	sb.WriteString(inputContainerFocusStyle.Width(max(1, m.width-2)).Render(m.editInput.View()))
	sb.WriteString("\n\n" + keyHintStyle.Render(t.ProfileSaveHint))
	return sb.String()
}

// renderInstructEdit — экран редактирования инструкций для AI.
func (m Model) renderInstructEdit() string {
	t := m.tr()
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(t.InstructTitle) + "\n\n")
	sb.WriteString(keyHintStyle.Render("  Tell AI how to respond — style, depth, format.\n  Applied to every message.\n\n"))
	sb.WriteString(inputContainerFocusStyle.Width(max(1, m.width-2)).Render(m.editInput.View()))
	sb.WriteString("\n\n" + keyHintStyle.Render(t.InstructSaveHint))
	return sb.String()
}

// newEditTextarea — textarea для редактирования профиля/инструкций.
func newEditTextarea(content string, width int) textarea.Model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")
	ta.SetValue(content)
	ta.Focus()
	ta.SetWidth(width - 4)
	ta.SetHeight(10)
	return ta
}

// renderQuestionPanel — Q&A от AI с чекбоксами.
func (m Model) renderQuestionPanel() string {
	var sb strings.Builder
	w := m.width - 2

	questionText := lipgloss.NewStyle().
		Foreground(colorPrimary).Bold(true).
		Render("❓ " + m.question)

	hintText := m.tr().QAHint
	if m.questionMulti {
		hintText += "  [multi-select]"
	}
	hint := lipgloss.NewStyle().
		Foreground(colorMuted).Italic(true).
		Render(hintText)
	sb.WriteString(questionText + "\n")
	sb.WriteString(hint + "\n\n")

	for i, opt := range m.questionOptions {
		isSelected := m.questionSelected[i]
		isCursor := m.questionCursor == i

		var checkbox string
		if isSelected {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")).Bold(true).Render("✓")
		} else {
			checkbox = lipgloss.NewStyle().Foreground(colorMuted).Render("○")
		}

		label := fmt.Sprintf(" %s  %s", checkbox, opt)

		var btn string
		switch {
		case isCursor && isSelected:
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#2D4A2D")).
				Foreground(lipgloss.Color("#98C379")).Bold(true).
				Padding(0, 1).Width(w - 2).
				Render("▶" + label)
		case isCursor:
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#2C313A")).
				Foreground(colorText).Bold(true).
				Padding(0, 1).Width(w - 2).
				Render("▶" + label)
		case isSelected:
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E3A1E")).
				Foreground(lipgloss.Color("#98C379")).
				Padding(0, 1).Width(w - 2).
				Render(" " + label)
		default:
			btn = lipgloss.NewStyle().
				Background(lipgloss.Color("#21252B")).
				Foreground(colorText).
				Padding(0, 1).Width(w - 2).
				Render(" " + label)
		}

		sb.WriteString(btn + "\n")
	}

	sb.WriteString("\n")
	isInputFocused := m.questionCursor == len(m.questionOptions)
	prompt := inputPromptStyle.Render("✏ ")

	var inputBox string
	if isInputFocused {
		inputBox = inputContainerFocusStyle.Width(w).Render(prompt + m.input.View())
	} else {
		inputBox = inputContainerStyle.Width(w).Render(prompt + m.input.View())
	}
	sb.WriteString(inputBox)

	if len(m.questionSelected) > 0 {
		count := 0
		for _, v := range m.questionSelected {
			if v {
				count++
			}
		}
		if count > 0 {
			sb.WriteString("\n" + lipgloss.NewStyle().
				Foreground(lipgloss.Color("#98C379")).
				Render(fmt.Sprintf(m.tr().QASelected, count)))
		}
	}

	return sb.String()
}

// submitQuestionAnswer — отправляет выбранный ответ.
func (m Model) submitQuestionAnswer() (tea.Model, tea.Cmd) {
	var answer string
	customText := strings.TrimSpace(m.input.Value())

	if customText != "" {
		answer = customText
	} else if len(m.questionSelected) > 0 {
		var selected []string
		for i, opt := range m.questionOptions {
			if m.questionSelected[i] {
				selected = append(selected, opt)
			}
		}
		if len(selected) == 1 {
			answer = selected[0]
		} else if len(selected) > 1 {
			answer = strings.Join(selected, ", ")
		}
	} else if m.questionCursor < len(m.questionOptions) {
		answer = m.questionOptions[m.questionCursor]
	}

	if answer == "" {
		return m, nil
	}

	wasToolCall := m.questionToolCall
	savedQuestion := m.question
	m.question = ""
	m.questionOptions = nil
	m.questionCursor = 0
	m.questionSelected = make(map[int]bool)
	m.questionMulti = false
	m.questionToolCall = false
	m.input.Reset()
	m.currentState = stateThinking
	m = m.resize()
	m.streaming = ""
	m.fullResponse = ""
	m.toolsExtracted = false
	m.displayedLen = 0
	m.genStartTime = time.Now()
	m.genTokens = 0
	m.genSpeed = 0
	m.retryCount = 0
	m.pendingContent = ""

	if wasToolCall {
		m.sess.AddMessage(session.RoleUser,
			fmt.Sprintf("[Answer to: %s]\n%s", savedQuestion, answer))
	} else {
		m.sess.AddMessage(session.RoleUser, answer)
	}

	m.refreshViewport()
	m.scrollToBottom = true

	return m, tea.Batch(m.streamAI(), m.spinner.Tick)
}
