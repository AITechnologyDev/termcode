package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/charmbracelet/lipgloss"
)

// resize пересчитывает размеры компонентов под текущий размер терминала.
func (m Model) resize() Model {
	headerH := 1
	statusH := 1
	hintsH := 1
	dividerH := 1

	inputH := 5
	if m.currentState == stateQuestion && len(m.questionOptions) > 0 {
		inputH = 3 + len(m.questionOptions) + 3
		if inputH > m.height/2 {
			inputH = m.height / 2
		}
	}

	vpHeight := m.height - headerH - inputH - statusH - hintsH - dividerH
	if vpHeight < 3 {
		vpHeight = 3
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	m.input.SetWidth(max(1, m.width-4))
	m.refreshViewport()
	return m
}

// refreshViewport перерисовывает содержимое viewport.
func (m *Model) refreshViewport() {
	m.viewport.SetContent(m.renderMessages())
}

// formatTok форматирует число токенов: 1200 → "1.2k", 500 → "500".
func formatTok(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

// truncate обрезает строку до maxLen рун.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-2]) + ".."
}

// stripANSI убирает ANSI escape коды из текста.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[()][AB012]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// formatAge — "2m ago", "3h ago", "5d ago".
func (m Model) formatAge(t time.Time) string {
	tr := m.tr()
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return tr.AgeJustNow
	case d < time.Hour:
		return fmt.Sprintf(tr.AgeMin, int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf(tr.AgeHour, int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf(tr.AgeDay, int(d.Hours()/24))
	default:
		return t.Format("02.01.06")
	}
}

// max returns the larger of two ints (replaces builtin for older Go targets).
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Размеры терминала (Termux) ─────────────────────────────────────────────

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTermSize возвращает размеры терминала через TIOCGWINSZ.
// В Termux v0.118.0+ поля Xpixel и Ypixel заполняются корректно.
func getTermSize() (*winsize, error) {
	ws := &winsize{}
	ret, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))
	if int(ret) == -1 {
		return nil, err
	}
	return ws, nil
}

// renderOverlay накладывает overlay поверх base по центру (чуть выше центра).
func renderOverlay(base, overlay string, width, height int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if lw := lipgloss.Width(l); lw > overlayW {
			overlayW = lw
		}
	}

	startY := (height - overlayH) / 3
	startX := (width - overlayW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	baseLines := strings.Split(base, "\n")
	for i, ol := range overlayLines {
		y := startY + i
		if y >= len(baseLines) {
			baseLines = append(baseLines, "")
		}
		bl := baseLines[y]
		blRunes := []rune(bl)
		olRunes := []rune(ol)
		for len(blRunes) < startX+len(olRunes) {
			blRunes = append(blRunes, ' ')
		}
		copy(blRunes[startX:], olRunes)
		baseLines[y] = string(blRunes)
	}
	return strings.Join(baseLines, "\n")
}

// workdirShort — сокращает путь, заменяя home на ~.
func workdirShort(p string) string {
	if home, err := os.UserHomeDir(); err == nil {
		return strings.Replace(p, home, "~", 1)
	}
	return p
}

// cancelStream — отменяет текущий стрим, если он активен.
func (m *Model) cancelStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
}
