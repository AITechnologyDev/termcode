package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Start запускает TUI
func Start(m *Model) error {
	p := tea.NewProgram(
		*m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
