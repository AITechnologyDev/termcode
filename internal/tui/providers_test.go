package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/NekoFemDev/termcode/internal/config"
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
