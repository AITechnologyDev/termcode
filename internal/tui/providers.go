package tui

import (
	"fmt"
	"strings"

	"github.com/NekoFemDev/termcode/internal/ai"
	"github.com/NekoFemDev/termcode/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// switchProvider — применяет выбранного провайдера.
func (m Model) switchProvider(id config.Provider) (Model, tea.Cmd) {
	m.cfg.ActiveProvider = id
	_ = m.cfg.Save()

	pc, _ := m.cfg.ActiveProviderConfig()
	if newProvider, err := ai.New(pc, id); err == nil {
		m.provider = newProvider
	}

	if id == config.ProviderOllama {
		return m, fetchContextLength(pc.BaseURL, pc.Model)
	}
	return m, nil
}

// renderProviderSelect — богатый экран выбора провайдера с метаданными.
// Режим providerEditMode:
//
//	0 = обычный выбор (Enter переключает)
//	1 = редактирование API key
//	2 = редактирование Base URL
//	3 = редактирование model
func (m Model) renderProviderSelect() string {
	t := m.tr()
	metas := config.ProvidersMeta()

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(t.ProviderTitle) + "\n\n")
	sb.WriteString(keyHintStyle.Render(t.ProviderHint))

	for i, pm := range metas {
		pc := m.cfg.Providers[pm.ID]
		isActive := pm.ID == m.cfg.ActiveProvider
		isCursor := i == m.providerCursor

		// Активная плашка слева
		var activeMark string
		if isActive {
			activeMark = providerActiveStyle.Render(" ● ")
		} else {
			activeMark = "   "
		}

		// Иконка + label
		iconLabel := fmt.Sprintf("%s %s", pm.Icon, pm.Label)

		// Тег типа провайдера
		var kindTag string
		switch pm.Kind {
		case config.KindLocal:
			kindTag = tagLocalStyle.Render(" " + t.ProviderSectionLocal + " ")
		case config.KindFree:
			kindTag = tagFreeStyle.Render(" " + t.ProviderSectionFree + " ")
		case config.KindCloud:
			kindTag = tagCloudStyle.Render(" " + t.ProviderSectionCloud + " ")
		}

		// Статус ключа
		var keyStatus string
		if pc.NeedsAPIKey(pm.ID) {
			if pc.APIKey != "" {
				keyStatus = keyStatusOKStyle.Render(" " + t.ProviderKeyPresent + " ")
			} else {
				keyStatus = keyStatusBadStyle.Render(" " + t.ProviderKeyMissing + " ")
			}
		} else {
			keyStatus = keyStatusNAStyle.Render(" " + t.ProviderKeyLocal + " ")
		}

		// Модель
		modelLine := keyHintStyle.Render("model: " + pc.Model)

		// Сборка строки
		header := fmt.Sprintf("%s%s  %s%s", activeMark, iconLabel, kindTag, keyStatus)
		if isCursor {
			header = providerSelectedStyle.Render(header)
		}
		sb.WriteString(header + "\n")

		// Описание (только на текущей строке, чтобы не раздувать)
		if isCursor {
			desc := keyHintStyle.Render("    " + pm.Description)
			sb.WriteString(desc + "\n")
			sb.WriteString("    " + modelLine + "\n")
		}
	}

	// Подвал с подсказками
	sb.WriteString("\n")
	current := metas[m.providerCursor]
	pc := m.cfg.Providers[current.ID]
	if pc.NeedsAPIKey(current.ID) && pc.APIKey == "" && current.KeyURL != "" {
		sb.WriteString(keyHintStyle.Render(t.ProviderKeyURL+" ") + linkStyle.Render(current.KeyURL) + "\n")
	}

	if m.providerEditMode != 0 {
		sb.WriteString("\n")
		sb.WriteString(m.renderProviderEditField())
	} else {
		sb.WriteString(keyHintStyle.Render(t.ProviderPickModel + "\n"))
	}

	sb.WriteString("\n" + keyHintStyle.Render(t.OverlayBack))
	return sb.String()
}

// renderProviderEditField — показывает textarea для редактирования поля провайдера.
func (m Model) renderProviderEditField() string {
	t := m.tr()
	fieldName := ""
	switch m.providerEditMode {
	case 1:
		fieldName = t.FldKey
	case 2:
		fieldName = t.FldURL
	case 3:
		fieldName = t.FldModel
	}
	return inputContainerFocusStyle.Width(max(1, m.width-4)).Render(
		lipgloss.NewStyle().Bold(true).Render(fieldName+":\n") + m.editInput.View(),
	)
}
