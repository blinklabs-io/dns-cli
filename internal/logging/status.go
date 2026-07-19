package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/report"
	"github.com/mattn/go-isatty"
)

// WaitProgress holds one confirmation-poll snapshot for the live status box.
type WaitProgress struct {
	Stage       string
	Process     string
	TxID        string
	ExplorerURL string
	Indexes     []uint32
	Poll        int
	StartedAt   time.Time
	Deadline    time.Time
}

// WaitReporter receives poll ticks during AwaitOutputs-style waits.
type WaitReporter interface {
	Tick(p WaitProgress)
	Done(p WaitProgress, err error)
}

// StatusBox writes an in-place wait dashboard to stderr (TTY) or newline status lines (non-TTY).
type StatusBox struct {
	w             io.Writer
	interactive   bool
	explorerEvery int
	color         bool
	mu            sync.Mutex
	lastLines     int
}

// StatusBoxOptions configures StatusBox behavior.
type StatusBoxOptions struct {
	Writer         io.Writer
	ForcePlain     bool // always use newline status (e.g. --output json)
	Color          bool // ANSI styling when interactive/plain human output
	ExplorerEveryN int  // reprint explorer URL every N polls (default 3)
}

// NewStatusBox creates a wait status reporter. Writer defaults to os.Stderr.
func NewStatusBox(opts StatusBoxOptions) *StatusBox {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	every := opts.ExplorerEveryN
	if every <= 0 {
		every = 3
	}
	interactive := !opts.ForcePlain && isWriterTTY(w)
	return &StatusBox{
		w:             w,
		interactive:   interactive,
		explorerEvery: every,
		color:         opts.Color && !opts.ForcePlain,
	}
}

func isWriterTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Tick redraws the status box (TTY) or prints a progress line (non-TTY).
func (b *StatusBox) Tick(p WaitProgress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.interactive {
		SuspendQuietLogs()
	}
	th := report.New(b.color)
	if b.interactive {
		// Keep the live panel narrow so wrapped URLs cannot desync cursor clears.
		live := p
		live.ExplorerURL = ""
		b.redrawLocked(th, live, false, nil)
		return
	}
	// Non-TTY: periodic newline so CI/demo pipes still show progress.
	if p.Poll == 1 || p.Poll%b.explorerEvery == 0 {
		fmt.Fprintln(b.w, th.WaitLine(waitInfoFromProgress(p, false, nil)))
		if p.ExplorerURL != "" {
			fmt.Fprint(b.w, th.Kv("explorer", p.ExplorerURL))
		}
	}
}

// Done prints a final status line and stops in-place overwriting.
func (b *StatusBox) Done(p WaitProgress, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer ResumeQuietLogs()
	th := report.New(b.color)
	if b.interactive {
		b.clearLocked()
	}
	fmt.Fprintln(b.w, th.WaitLine(waitInfoFromProgress(p, true, err)))
	if p.ExplorerURL != "" {
		fmt.Fprint(b.w, th.Kv("explorer", p.ExplorerURL))
	}
	b.lastLines = 0
}

func (b *StatusBox) redrawLocked(th *report.Theme, p WaitProgress, done bool, err error) {
	panel := th.WaitPanel(waitInfoFromProgress(p, done, err))
	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	b.clearLocked()
	for _, line := range lines {
		fmt.Fprintln(b.w, line)
	}
	b.lastLines = len(lines)
}

func (b *StatusBox) clearLocked() {
	if b.lastLines <= 0 {
		return
	}
	// Move cursor up and clear each previous status line.
	for i := 0; i < b.lastLines; i++ {
		fmt.Fprint(b.w, "\x1b[1A\x1b[2K")
	}
	b.lastLines = 0
}

func waitInfoFromProgress(p WaitProgress, done bool, err error) report.WaitInfo {
	elapsed := time.Since(p.StartedAt).Truncate(time.Second).String()
	remain := remainingString(p.Deadline)
	stage := p.Stage
	if stage == "" {
		stage = "tx.confirm"
	}
	process := p.Process
	if process == "" {
		process = "waiting for outputs"
	}
	state := "waiting"
	if done {
		if err != nil {
			state = "failed"
		} else {
			state = "confirmed"
		}
	}
	info := report.WaitInfo{
		Stage:       stage,
		Process:     process,
		State:       state,
		Elapsed:     elapsed,
		Remain:      remain,
		Poll:        p.Poll,
		TxID:        p.TxID,
		ExplorerURL: p.ExplorerURL,
		Indexes:     formatIndexes(p.Indexes),
	}
	if err != nil {
		info.Error = err.Error()
	}
	return info
}

// formatWaitBox returns the multi-line status box (for tests).
func formatWaitBox(p WaitProgress, done bool, err error) []string {
	panel := report.New(false).WaitPanel(waitInfoFromProgress(p, done, err))
	return strings.Split(strings.TrimRight(panel, "\n"), "\n")
}

func formatWaitLine(p WaitProgress, done bool, err error) string {
	return report.New(false).WaitLine(waitInfoFromProgress(p, done, err))
}

func remainingString(deadline time.Time) string {
	if deadline.IsZero() {
		return "unknown"
	}
	d := time.Until(deadline)
	if d < 0 {
		return "0s"
	}
	return d.Truncate(time.Second).String()
}

func formatIndexes(indexes []uint32) string {
	if len(indexes) == 0 {
		return ""
	}
	parts := make([]string, len(indexes))
	for i, idx := range indexes {
		parts[i] = fmt.Sprintf("#%d", idx)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
