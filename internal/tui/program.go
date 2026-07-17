package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// RunDashboard starts the Layout A operator dashboard.
func RunDashboard(opts DashboardOpts) error {
	p := tea.NewProgram(initialModel(opts))
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}
	return nil
}
