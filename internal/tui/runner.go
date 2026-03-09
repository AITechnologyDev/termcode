package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// wrapper — обёртка над Model для перехвата нестандартных сообщений
// Нужна потому что BubbleTea вызывает Update через интерфейс tea.Model,
// а streamReaderMsg нужно обрабатывать отдельно
type wrapper struct {
	m Model
}

func (w wrapper) Init() tea.Cmd {
	return w.m.Init()
}

func (w wrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if srm, ok := msg.(streamReaderMsg); ok {
		newM, cmd := w.m.updateStream(srm)
		return wrapper{m: newM}, cmd
	}
	newM, cmd := w.m.Update(msg)
	return wrapper{m: newM.(Model)}, cmd
}

func (w wrapper) View() string {
	return w.m.View()
}

// Start запускает TUI с готовой моделью
func Start(m *Model) error {
	p := tea.NewProgram(
		wrapper{m: *m},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
