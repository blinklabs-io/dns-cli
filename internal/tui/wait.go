package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/report"
	"github.com/mattn/go-isatty"
)

// WaitOptions configures wait UI selection.
type WaitOptions struct {
	ForcePlain bool
	Color      bool
	Writer     io.Writer // defaults to stderr
	OnCancel   context.CancelFunc
}

// NewWaitReporter returns a plain StatusBox or an interactive Bubble Tea waiter.
func NewWaitReporter(opts WaitOptions) logging.WaitReporter {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	if opts.ForcePlain || !writerIsTTY(w) {
		return logging.NewStatusBox(logging.StatusBoxOptions{
			Writer:     w,
			ForcePlain: true,
			Color:      opts.Color && !opts.ForcePlain,
		})
	}
	return newBubbleWaitReporter(w, opts.OnCancel, opts.Color)
}

func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// cancelSetter is implemented by interactive wait reporters.
type cancelSetter interface {
	SetCancel(context.CancelFunc)
}

// AttachCancel sets OnCancel on reporters that support it (Bubble Tea wait UI).
func AttachCancel(reporter logging.WaitReporter, cancel context.CancelFunc) {
	if cs, ok := reporter.(cancelSetter); ok {
		cs.SetCancel(cancel)
	}
}

type waitTickMsg struct {
	Progress logging.WaitProgress
}

type waitDoneMsg struct {
	Progress logging.WaitProgress
	Err      error
}

type waitModel struct {
	progress logging.WaitProgress
	done     bool
	err      error
	width    int
	color    bool
}

func (m waitModel) Init() tea.Cmd { return nil }

func (m waitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case waitTickMsg:
		m.progress = msg.Progress
		m.done = false
		m.err = nil
		return m, nil
	case waitDoneMsg:
		m.progress = msg.Progress
		m.done = true
		m.err = msg.Err
		return m, tea.Quit
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "c", "esc":
			return m, tea.Quit
		}
	case tea.QuitMsg:
		return m, nil
	}
	return m, nil
}

func (m waitModel) View() tea.View {
	// Alt screen isolates the wait panel from slog/stderr so redraws stay clean.
	v := tea.NewView(renderWaitView(m.progress, m.done, m.err, m.color))
	v.AltScreen = true
	return v
}

func renderWaitView(p logging.WaitProgress, done bool, err error, color bool) string {
	elapsed := time.Since(p.StartedAt).Truncate(time.Second).String()
	remain := "unknown"
	if !p.Deadline.IsZero() {
		d := time.Until(p.Deadline)
		if d < 0 {
			remain = "0s"
		} else {
			remain = d.Truncate(time.Second).String()
		}
	}
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
	indexes := ""
	if len(p.Indexes) > 0 {
		parts := make([]string, len(p.Indexes))
		for i, idx := range p.Indexes {
			parts[i] = fmt.Sprintf("#%d", idx)
		}
		indexes = "[" + strings.Join(parts, ",") + "]"
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
		Indexes:     indexes,
		ShowKeys:    !done,
	}
	if err != nil {
		info.Error = err.Error()
	}
	return strings.TrimRight(report.New(color).WaitPanel(info), "\n")
}

type bubbleWaitReporter struct {
	mu       sync.Mutex
	prog     *tea.Program
	cancel   context.CancelFunc
	started  bool
	finished chan struct{}
	out      io.Writer
	color    bool
}

func newBubbleWaitReporter(out io.Writer, cancel context.CancelFunc, color bool) *bubbleWaitReporter {
	return &bubbleWaitReporter{
		cancel:   cancel,
		finished: make(chan struct{}),
		out:      out,
		color:    color,
	}
}

func (r *bubbleWaitReporter) SetCancel(cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancel = cancel
}

func (r *bubbleWaitReporter) ensureStarted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true
	m := waitModel{color: r.color}
	opts := []tea.ProgramOption{}
	if f, ok := r.out.(*os.File); ok {
		opts = append(opts, tea.WithOutput(f))
	}
	r.prog = tea.NewProgram(m, opts...)
	go func() {
		_, _ = r.prog.Run()
		r.mu.Lock()
		cancel := r.cancel
		r.mu.Unlock()
		// If the user quit early while still waiting, cancel the await context.
		if cancel != nil {
			cancel()
		}
		close(r.finished)
	}()
}

func (r *bubbleWaitReporter) closed() bool {
	select {
	case <-r.finished:
		return true
	default:
		return false
	}
}

func (r *bubbleWaitReporter) Tick(p logging.WaitProgress) {
	if r.closed() {
		return
	}
	logging.SuspendQuietLogs()
	r.ensureStarted()
	if r.prog != nil {
		r.prog.Send(waitTickMsg{Progress: p})
	}
}

func (r *bubbleWaitReporter) Done(p logging.WaitProgress, err error) {
	// Prevent cancel-on-quit from firing after successful/failed completion.
	r.mu.Lock()
	r.cancel = nil
	r.mu.Unlock()
	defer logging.ResumeQuietLogs()
	if r.closed() {
		return
	}
	r.ensureStarted()
	if r.prog != nil {
		r.prog.Send(waitDoneMsg{Progress: p, Err: err})
	}
	select {
	case <-r.finished:
	case <-time.After(2 * time.Second):
		if r.prog != nil {
			r.prog.Quit()
		}
	}
	// Leave a clean newline on the primary screen after alt-screen exit.
	fmt.Fprintln(r.out)
}
