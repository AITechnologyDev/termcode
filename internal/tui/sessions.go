package tui

import (
	"fmt"
	"strings"

	"github.com/NekoFemDev/termcode/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

func loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, _ := session.LoadAll()
		return sessionsLoadedMsg{sessions: sessions}
	}
}

func (m Model) loadSession(s *session.Session) (tea.Model, tea.Cmd) {
	_ = m.sess.Save()

	m.sess = s
	m.currentState = stateChat
	m.streaming = ""
	m.fullResponse = ""
	m.toolsExtracted = false
	m.displayedLen = 0
	m.errMsg = ""
	m.contextUsed = 0
	m.thinkExpanded = make(map[int]bool)
	m.retryCount = 0
	m.pendingContent = ""

	m.refreshViewport()
	m.scrollToBottom = true
	return m, nil
}

func (m Model) deleteSession(s *session.Session) (tea.Model, tea.Cmd) {
	_ = session.Delete(s.ID)
	m.sessionsLoading = true
	return m, loadSessions()
}

func (m Model) renderSessionLoad() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(m.tr().SessionsTitle) + "\n\n")

	if m.sessionsLoading {
		sb.WriteString(fmt.Sprintf(m.tr().SessionsLoading, m.spinner.View()))
		return sb.String()
	}

	if len(m.savedSessions) == 0 {
		sb.WriteString(keyHintStyle.Render(m.tr().SessionsEmpty))
		sb.WriteString(keyStyle.Render("  Esc") + keyHintStyle.Render(" — back\n"))
		return sb.String()
	}

	sb.WriteString(keyHintStyle.Render(m.tr().SessionHint))

	for i, s := range m.savedSessions {
		age := m.formatAge(s.UpdatedAt)
		msgs := fmt.Sprintf(m.tr().SessionsMsgs, len(s.Messages))
		model := s.Model
		if len(model) > 22 {
			model = model[:20] + ".."
		}

		line := fmt.Sprintf("  %-40s  %-8s  %-22s  %s",
			truncate(s.Title, 40),
			msgs,
			model,
			age,
		)

		if i == m.sessionCursor {
			sb.WriteString(userBubbleStyle.Width(max(1, m.width-2)).Render("▶"+line) + "\n")
		} else {
			sb.WriteString(keyHintStyle.Render(" "+line) + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf(m.tr().SessionsCount, len(m.savedSessions)))
	return sb.String()
}
