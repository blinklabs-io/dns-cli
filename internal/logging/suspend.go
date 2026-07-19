package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// suspendGate drops non-warning slog records while an interactive wait UI owns
// the terminal. Warn/Error still pass through so failures remain visible.
type suspendGate struct {
	suspended atomic.Bool
}

var gate suspendGate

// SuspendQuietLogs pauses Info/Debug/Trace slog output so in-place wait UIs
// are not corrupted by concurrent stderr writes.
func SuspendQuietLogs() {
	gate.suspended.Store(true)
}

// ResumeQuietLogs restores normal slog output after a wait UI finishes.
func ResumeQuietLogs() {
	gate.suspended.Store(false)
}

// QuietLogsSuspended reports whether quiet logs are currently paused.
func QuietLogsSuspended() bool {
	return gate.suspended.Load()
}

type gatedHandler struct {
	inner slog.Handler
}

func (h *gatedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if gate.suspended.Load() && level < slog.LevelWarn {
		return false
	}
	return h.inner.Enabled(ctx, level)
}

func (h *gatedHandler) Handle(ctx context.Context, r slog.Record) error {
	if gate.suspended.Load() && r.Level < slog.LevelWarn {
		return nil
	}
	return h.inner.Handle(ctx, r)
}

func (h *gatedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &gatedHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *gatedHandler) WithGroup(name string) slog.Handler {
	return &gatedHandler{inner: h.inner.WithGroup(name)}
}

// wrapHandler gates an slog handler behind SuspendQuietLogs/ResumeQuietLogs.
func wrapHandler(inner slog.Handler) slog.Handler {
	return &gatedHandler{inner: inner}
}
