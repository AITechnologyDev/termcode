package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/NekoFemDev/termcode/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiRe2 = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// TestRenderProviderSelect_NoOverflow проверяет, что выделенная строка
// провайдера не «вылезает» за пределы ширины и фоновая подсветка
// покрывает всю строку (включая плашки тегов).
func TestRenderProviderSelect_NoOverflow(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ActiveProvider = config.ProviderOllama
	pc := cfg.Providers[config.ProviderOllama]
	pc.Model = "qwen3-coder-next"
	cfg.Providers[config.ProviderOllama] = pc

	m, err := New(cfg, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	m.width = 120
	m.height = 30
	m.currentState = stateProviderSelect
	m.providerCursor = 1

	out := m.renderProviderSelect()
	t.Logf("\n%s", out)

	// Каждая видимая строка без ANSI должна помещаться в ширину
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		visible := ansiRe2.ReplaceAllString(l, "")
		if len(visible) > m.width {
			t.Errorf("line %d too wide: %d > %d: %q", i, len(visible), m.width, visible)
		}
	}

	// Подсвеченная строка (OpenRouter, cursor=1) содержит оба тега
	var selLine string
	for _, l := range lines {
		if strings.Contains(l, "OpenRouter") {
			selLine = l
			break
		}
	}
	if selLine == "" {
		t.Fatal("OpenRouter line not found")
	}
	if !strings.Contains(selLine, "Free tier") {
		t.Errorf("selected row missing 'Free tier' tag: %q", selLine)
	}
	if !strings.Contains(selLine, "no key") {
		t.Errorf("selected row missing 'no key' tag: %q", selLine)
	}

	// Активный провайдер (Ollama) помечен ●
	var activeLine string
	for _, l := range lines {
		if strings.Contains(l, "Ollama") {
			activeLine = l
			break
		}
	}
	if !strings.Contains(activeLine, "●") {
		t.Errorf("active provider missing '●' marker: %q", activeLine)
	}
}

// TestRenderProviderSelect_ColorDump дампит ANSI-последовательности
// выделенной строки при TrueColor, чтобы понять, покрывает ли фон теги.
func TestRenderProviderSelect_ColorDump(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	cfg := config.DefaultConfig()
	cfg.ActiveProvider = config.ProviderOllama
	pc := cfg.Providers[config.ProviderOllama]
	pc.Model = "qwen3-coder-next"
	cfg.Providers[config.ProviderOllama] = pc

	m, err := New(cfg, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	m.width = 100
	m.height = 30
	m.currentState = stateProviderSelect
	m.providerCursor = 1

	out := m.renderProviderSelect()
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.Contains(l, "OpenRouter") {
			t.Logf("selected row line %d (raw length %d):", i, len(l))
			visible := ansiRe2.ReplaceAllString(l, "")
			t.Logf("  visible length: %d (expected ≤ %d)", len(visible), m.width)

			// Проверяем, что между "OpenRouter" и "Free tier" есть хотя бы
			// один серый фон (bg 55;65;81 = colorBgSubtle).
			idxLabel := strings.Index(visible, "OpenRouter")
			idxTag := strings.Index(visible, "Free tier")
			if idxLabel < 0 || idxTag < 0 {
				t.Fatal("missing label or tag in visible output")
			}
			// Ищем последний bg-параметр ДО позиции "Free tier"
			gapRegion := l[:strings.Index(l, "Free")]
			grayCount := strings.Count(gapRegion, "55;65;81")
			if grayCount < 2 {
				t.Errorf("expected gray background between 'OpenRouter' and 'Free tier' (count >=2), got %d", grayCount)
			}
		}
	}
}
