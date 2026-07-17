package tui

import "charm.land/lipgloss/v2"

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	stylePane   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	styleTitle  = lipgloss.NewStyle().Bold(true)
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleHelp   = lipgloss.NewStyle().Faint(true).Padding(0, 1)
)
