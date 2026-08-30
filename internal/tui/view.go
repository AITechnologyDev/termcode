package tui

import (
	"fmt"
	"strings"

	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/charmbracelet/lipgloss"
)

// View — корневой рендер. Разруливает режимы (оверлеи) и собирает финальный экран.
func (m Model) View() string {
	if m.width == 0 {
		return m.tr().LoadingModels
	}

	switch m.currentState {
	case stateModelSelect:
		return m.renderModelSelect()
	case statePulling:
		return m.renderPullScreen()
	case stateSessionLoad:
		return m.renderSessionLoad()
	case stateProviderSelect:
		return m.renderProviderSelect()
	case stateProfileEdit:
		return m.renderProfileEdit()
	case stateInstructEdit:
		return m.renderInstructEdit()
	}

	header := m.renderHeader()
	chatView := m.viewport.View()
	statusBar := m.renderStatusBar()

	var inputArea string
	if m.currentState == stateQuestion {
		inputArea = m.renderQuestionPanel()
	} else {
		inputArea = m.renderInput()
	}

	hints := m.renderHints()

	base := lipgloss.JoinVertical(lipgloss.Left,
		header,
		chatView,
		dividerStyle.Render(strings.Repeat("─", m.width)),
		inputArea,
		statusBar,
		hints,
	)

	if m.currentState == statePalette {
		return renderOverlay(base, m.renderPalette(), m.width, m.height)
	}

	return base
}

// renderHeader — компактная полоска: бренд · провайдер · модель · cwd · язык
func (m Model) renderHeader() string {
	title := headerStyle.Render(" TermCode ")

	lang := m.cfg.Language
	if lang == "" {
		lang = "en"
	}

	pm := config.GetProviderMeta(m.cfg.ActiveProvider)
	providerPill := headerPillStyle.Render(fmt.Sprintf(" %s %s ", pm.Icon, pm.Label))
	modelPill := headerInfoStyle.Render(" " + m.provider.Model() + " ")

	dirInfo := headerInfoStyle.Render(" " + workdirShort(m.workDir) + " ")
	langPill := langPillStyle.Render(" " + strings.ToUpper(lang) + " ")

	left := title + providerPill + modelPill
	right := dirInfo + langPill

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

// renderInput — поле ввода
func (m Model) renderInput() string {
	m.input.Placeholder = m.tr().Placeholder

	var style lipgloss.Style
	if m.currentState == stateThinking {
		style = inputContainerStyle
	} else {
		style = inputContainerFocusStyle
	}

	prompt := inputPromptStyle.Render("❯ ")
	inputView := m.input.View()

	return style.Width(max(1, m.width-2)).Render(prompt + inputView)
}

// renderStatusBar — статус: состояние слева, скорость и контекст справа
func (m Model) renderStatusBar() string {
	var left, right string

	switch m.currentState {
	case stateThinking, stateRetrying:
		speed := ""
		if m.genSpeed > 0 {
			speed = fmt.Sprintf("  %.1f tok/s · %s tok", m.genSpeed, formatTok(m.genTokens))
		}
		left = statusBusyStyle.Render(m.spinner.View()+" "+m.tr().StatusGenerating) +
			keyHintStyle.Render(speed)
	case stateChat:
		if m.errMsg != "" {
			left = statusErrStyle.Render("✗ " + m.errMsg)
		} else {
			left = statusOKStyle.Render("● " + fmt.Sprintf(m.tr().StatusReady, len(m.sess.Messages)))
		}
		if m.genSpeed > 0 {
			right = keyHintStyle.Render(fmt.Sprintf(m.tr().StatusLastTok, m.genSpeed, m.genTokens))
		}
	}

	if m.contextLimit > 0 {
		pct := 0
		if m.contextLimit > 0 {
			pct = m.contextUsed * 100 / m.contextLimit
		}
		ctxStyle := keyHintStyle
		if pct >= 80 {
			ctxStyle = statusErrStyle
		} else if pct >= 60 {
			ctxStyle = statusBusyStyle
		}
		ctxStr := ctxStyle.Render(fmt.Sprintf("ctx %d%% (%s/%s)",
			pct, formatTok(m.contextUsed), formatTok(m.contextLimit),
		))
		if right != "" {
			right = right + "   " + ctxStr
		} else {
			right = ctxStr
		}
	}

	if right != "" {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
		if gap < 1 {
			gap = 1
		}
		return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
	}
	return statusBarStyle.Width(m.width).Render(left)
}

// renderHints — подсказки клавиш
func (m Model) renderHints() string {
	t := m.tr()
	lang := m.cfg.Language
	if lang == "" {
		lang = "en"
	}
	hints := []string{
		keyStyle.Render("Enter") + keyHintStyle.Render(t.HintSend),
		keyStyle.Render("Shift+Enter") + keyHintStyle.Render(t.HintNewline),
		keyStyle.Render("Ctrl+P") + keyHintStyle.Render(t.HintCommands),
		keyStyle.Render("/models") + keyHintStyle.Render(t.HintModels),
		keyStyle.Render("Ctrl+S") + keyHintStyle.Render(t.HintSave),
	}
	return keyHintStyle.Render(strings.Join(hints, "   "))
}
