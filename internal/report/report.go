// Package report formats colored human-facing CLI stdout (not slog).
package report

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// RoadmapStatus is a checklist item state.
type RoadmapStatus int

const (
	StatusPending RoadmapStatus = iota
	StatusCurrent
	StatusDone
)

// KV is a labeled value row.
type KV struct {
	Key   string
	Value string
}

// RoadmapItem is one step in a progress checklist.
type RoadmapItem struct {
	Label  string
	Detail string
	Status RoadmapStatus
}

// Theme styles human reports. When Color is false, output has no ANSI codes.
type Theme struct {
	Color bool
}

// New returns a theme. color=false disables ANSI (for --no-color / NO_COLOR / tests).
func New(color bool) *Theme {
	return &Theme{Color: color}
}

func (t *Theme) style(base lipgloss.Style) lipgloss.Style {
	if !t.Color {
		return lipgloss.NewStyle()
	}
	return base
}

func (t *Theme) brand() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true))
}

func (t *Theme) title() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true))
}

func (t *Theme) dim() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")))
}

func (t *Theme) done() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")))
}

func (t *Theme) current() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true))
}

func (t *Theme) pending() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")))
}

func (t *Theme) accent() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")))
}

func (t *Theme) warn() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")))
}

func (t *Theme) ok() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Bold(true))
}

func (t *Theme) errStyle() lipgloss.Style {
	return t.style(lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true))
}

func (t *Theme) chip(kind int) lipgloss.Style {
	if !t.Color {
		return lipgloss.NewStyle()
	}
	switch kind % 3 {
	case 0:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#86efac")).Background(lipgloss.Color("#14532d")).Padding(0, 1)
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#93c5fd")).Background(lipgloss.Color("#1e3a5f")).Padding(0, 1)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#fcd34d")).Background(lipgloss.Color("#3b2f1a")).Padding(0, 1)
	}
}

func (t *Theme) render(style lipgloss.Style, s string) string {
	if !t.Color {
		return s
	}
	return style.Render(s)
}

const splashArt = `  ____  _   _ ____     ____ _     ___
 |  _ \| \ | / ___|   / ___| |   |_ _|
 | | | |  \| \___ \  | |   | |    | |
 | |_| | |\  |___) | | |___| |___ | |
 |____/|_| \_|____/   \____|_____|___|`

// Splash renders the ASCII brand banner with optional chips and subtitle.
func (t *Theme) Splash(subtitle string, chips ...string) string {
	var b strings.Builder
	b.WriteString(t.render(t.brand(), splashArt))
	b.WriteByte('\n')
	if subtitle != "" {
		b.WriteString(t.render(t.dim(), subtitle))
		b.WriteByte('\n')
	}
	if len(chips) > 0 {
		parts := make([]string, 0, len(chips))
		for i, c := range chips {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if t.Color {
				parts = append(parts, t.chip(i).Render(c))
			} else {
				parts = append(parts, "["+c+"]")
			}
		}
		if len(parts) > 0 {
			b.WriteString(strings.Join(parts, " "))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// Title renders a section heading.
func (t *Theme) Title(s string) string {
	return t.render(t.title(), "◆ "+s)
}

// Panel renders a titled key/value block.
func (t *Theme) Panel(title string, rows []KV) string {
	var b strings.Builder
	b.WriteString(t.Title(title))
	b.WriteByte('\n')
	keyWidth := 0
	for _, row := range rows {
		if n := len(row.Key); n > keyWidth {
			keyWidth = n
		}
	}
	for _, row := range rows {
		key := row.Key
		if keyWidth > 0 {
			key = fmt.Sprintf("%-*s", keyWidth, row.Key)
		}
		b.WriteString(t.render(t.dim(), "  "+key+"  "))
		b.WriteString(row.Value)
		b.WriteByte('\n')
	}
	return b.String()
}

// Roadmap renders a done/current/pending checklist.
func (t *Theme) Roadmap(title, subtitle string, items []RoadmapItem) string {
	var b strings.Builder
	if title != "" {
		b.WriteString(t.render(t.title(), "◆ "+title))
		b.WriteByte('\n')
	}
	if subtitle != "" {
		b.WriteString(t.render(t.dim(), subtitle))
		b.WriteByte('\n')
	}
	for _, item := range items {
		glyph, style := t.roadmapStyle(item.Status)
		line := glyph + " " + item.Label
		b.WriteString(t.render(style, line))
		if item.Detail != "" {
			b.WriteString("  ")
			b.WriteString(t.render(t.dim(), item.Detail))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (t *Theme) roadmapStyle(st RoadmapStatus) (string, lipgloss.Style) {
	switch st {
	case StatusDone:
		return "●", t.done()
	case StatusCurrent:
		return "◉", t.current()
	default:
		return "○", t.pending()
	}
}

// Step announces a major phase with optional CLI hint lines.
func (t *Theme) Step(title, what string, cliLines ...string) string {
	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(t.render(t.brand(), "━━ DEMO ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	b.WriteByte('\n')
	b.WriteString(t.render(t.current(), "  "+title))
	b.WriteByte('\n')
	if what != "" {
		b.WriteString(t.render(t.accent(), "  "+what))
		b.WriteByte('\n')
	}
	if len(cliLines) > 0 {
		b.WriteString(t.render(t.dim(), "  Equivalent CLI:"))
		b.WriteByte('\n')
		for _, line := range cliLines {
			b.WriteString(t.render(t.brand(), "    "+line))
			b.WriteByte('\n')
		}
	}
	b.WriteString(t.render(t.dim(), "────────────────────────────────────────────"))
	b.WriteByte('\n')
	return b.String()
}

// Note prints a short one-line demo note.
func (t *Theme) Note(msg string) string {
	return t.render(t.dim(), "── DEMO · ") + msg + "\n"
}

// Dim styles secondary text (or returns plain when color is off).
func (t *Theme) Dim(s string) string {
	return t.render(t.dim(), s)
}

// Warn formats a warning line.
func (t *Theme) Warn(msg string) string {
	return t.render(t.warn(), "warning: ") + msg + "\n"
}

// Kv formats a single key/value line.
func (t *Theme) Kv(key, value string) string {
	return t.render(t.dim(), key+": ") + value + "\n"
}

// TxLine formats a labeled transaction id and optional explorer URL.
func (t *Theme) TxLine(label, txID, explorerPrefix string) string {
	var b strings.Builder
	labelPad := fmt.Sprintf("%-12s", label)
	if strings.TrimSpace(txID) == "" {
		b.WriteString("  ")
		b.WriteString(t.render(t.dim(), labelPad))
		b.WriteString(t.render(t.pending(), "  (not confirmed)"))
		b.WriteByte('\n')
		return b.String()
	}
	b.WriteString("  ")
	b.WriteString(t.render(t.dim(), labelPad+"  "))
	b.WriteString(t.render(t.brand(), txID))
	b.WriteByte('\n')
	if explorerPrefix != "" {
		b.WriteString("                 ")
		b.WriteString(t.render(t.accent(), explorerPrefix+txID))
		b.WriteByte('\n')
	}
	return b.String()
}

// Completion renders the demo-complete report header + body blocks.
func (t *Theme) Completion(title string, meta []KV, txBlock, pathsBlock, nextBlock string) string {
	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(t.render(t.ok(), "◆ "+title))
	b.WriteByte('\n')
	for _, row := range meta {
		b.WriteString("  ")
		b.WriteString(t.render(t.dim(), fmt.Sprintf("%-10s", row.Key)))
		b.WriteString(row.Value)
		b.WriteByte('\n')
	}
	if txBlock != "" {
		b.WriteByte('\n')
		b.WriteString(t.render(t.dim(), "  Transactions (Preprod explorer):"))
		b.WriteByte('\n')
		b.WriteString(txBlock)
	}
	if pathsBlock != "" {
		b.WriteByte('\n')
		b.WriteString(t.render(t.dim(), "  Artifacts / state:"))
		b.WriteByte('\n')
		b.WriteString(pathsBlock)
	}
	if nextBlock != "" {
		b.WriteByte('\n')
		b.WriteString(nextBlock)
	}
	b.WriteString(t.render(t.dim(), "────────────────────────────────────────────"))
	b.WriteByte('\n')
	return b.String()
}

// ResultPanel formats a generic command success/failure human result.
func (t *Theme) ResultPanel(ok bool, message string, rows []KV, warnings []string) string {
	var b strings.Builder
	title := "ok"
	style := t.ok()
	if !ok {
		title = "error"
		style = t.errStyle()
	}
	if message != "" {
		title = message
	}
	b.WriteString(t.render(style, "◆ "+title))
	b.WriteByte('\n')
	keyWidth := 0
	for _, row := range rows {
		if n := len(row.Key); n > keyWidth {
			keyWidth = n
		}
	}
	for _, row := range rows {
		key := row.Key
		if keyWidth > 0 {
			key = fmt.Sprintf("%-*s", keyWidth, row.Key)
		}
		b.WriteString(t.render(t.dim(), key+"  "))
		b.WriteString(row.Value)
		b.WriteByte('\n')
	}
	for _, w := range warnings {
		b.WriteString(t.Warn(w))
	}
	return b.String()
}

// WaitInfo is one confirmation-poll snapshot for the wait panel.
type WaitInfo struct {
	Stage       string
	Process     string
	State       string
	Elapsed     string
	Remain      string
	Poll        int
	TxID        string
	ExplorerURL string
	Indexes     string
	Error       string
	ShowKeys    bool
}

// WaitPanel renders the live dns-cli wait dashboard.
func (t *Theme) WaitPanel(info WaitInfo) string {
	stage := info.Stage
	if stage == "" {
		stage = "tx.confirm"
	}
	process := info.Process
	if process == "" {
		process = "waiting for outputs"
	}
	state := info.State
	if state == "" {
		state = "waiting"
	}
	stateStyled := state
	switch state {
	case "confirmed":
		stateStyled = t.render(t.ok(), state)
	case "failed":
		stateStyled = t.render(t.errStyle(), state)
	default:
		stateStyled = t.render(t.current(), state)
	}
	rows := []KV{
		{Key: "stage", Value: fmt.Sprintf("%s (%s)", stage, stateStyled)},
		{Key: "process", Value: strings.TrimSpace(process + " " + info.Indexes)},
		{Key: "elapsed", Value: info.Elapsed},
		{Key: "timeout", Value: info.Remain + " remaining"},
		{Key: "poll", Value: fmt.Sprintf("#%d", info.Poll)},
		{Key: "txId", Value: info.TxID},
	}
	if info.ExplorerURL != "" {
		rows = append(rows, KV{Key: "explorer", Value: info.ExplorerURL})
	}
	if info.Error != "" {
		rows = append(rows, KV{Key: "error", Value: info.Error})
	}
	if info.ShowKeys {
		rows = append(rows, KV{Key: "keys", Value: "c/q cancel · ctrl+c quit"})
	}
	return t.Panel("dns-cli wait", rows)
}

// WaitLine is a single-line wait status for non-TTY / CI pipes.
func (t *Theme) WaitLine(info WaitInfo) string {
	stage := info.Stage
	if stage == "" {
		stage = "tx.confirm"
	}
	state := info.State
	if state == "" {
		state = "waiting"
	}
	msg := fmt.Sprintf("[wait] stage=%s state=%s poll=#%d elapsed=%s remaining=%s txId=%s",
		stage, state, info.Poll, info.Elapsed, info.Remain, info.TxID)
	if info.Error != "" {
		msg += " error=" + info.Error
	}
	prefix := t.render(t.brand(), "◆ wait")
	return prefix + "  " + t.render(t.dim(), msg)
}

// Guide renders a titled self-serve instruction block.
func (t *Theme) Guide(title string, lines ...string) string {
	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(t.render(t.title(), "◆ "+title))
	b.WriteByte('\n')
	for _, line := range lines {
		b.WriteString(t.render(t.accent(), "  "+line))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

// SortedDataRows converts a data map to stably sorted KV rows.
func SortedDataRows(data map[string]any) []KV {
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]KV, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, KV{Key: k, Value: fmt.Sprint(data[k])})
	}
	return rows
}

// HasANSI reports whether s contains an ESC CSI sequence (for tests).
func HasANSI(s string) bool {
	return strings.Contains(s, "\x1b[")
}
