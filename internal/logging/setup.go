package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

// Options configures the process-wide slog default logger.
type Options struct {
	Verbose int
	NoColor bool
	Output  string
	Writer  io.Writer
}

// Configure installs the default slog logger on stderr (or Writer).
func Configure(opts Options) error {
	level, err := LevelFromVerbose(opts.Verbose)
	if err != nil {
		return err
	}
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	useJSON := strings.EqualFold(strings.TrimSpace(opts.Output), "json")
	noColor := opts.NoColor || os.Getenv("NO_COLOR") != ""

	var handler slog.Handler
	if useJSON {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: traceReplaceAttr,
		})
	} else {
		out := w
		if !noColor {
			if f, ok := w.(*os.File); ok && isatty.IsTerminal(f.Fd()) {
				out = colorable.NewColorable(f)
			}
		}
		handler = tint.NewHandler(out, &tint.Options{
			Level:      level,
			TimeFormat: time.StampMilli,
			NoColor:    noColor,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				a = traceReplaceAttr(groups, a)
				if a.Value.Kind() == slog.KindAny {
					if _, ok := a.Value.Any().(error); ok {
						return tint.Attr(9, a)
					}
				}
				return a
			},
		})
	}
	slog.SetDefault(slog.New(wrapHandler(handler)))
	return nil
}

func traceReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey && len(groups) == 0 {
		lvl, ok := a.Value.Any().(slog.Level)
		if ok && lvl <= LevelTrace {
			return slog.String(slog.LevelKey, "TRACE")
		}
	}
	return a
}
