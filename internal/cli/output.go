package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// OutputMode controls how command results are rendered.
type OutputMode string

const (
	OutputHuman OutputMode = "human"
	OutputJSON  OutputMode = "json"
)

// Result is the stable machine-readable envelope for command success/failure details.
type Result struct {
	OK          bool           `json:"ok"`
	Command     string         `json:"command,omitempty"`
	Network     string         `json:"network,omitempty"`
	Operation   string         `json:"operation,omitempty"`
	Artifact    string         `json:"artifact,omitempty"`
	TxID        string         `json:"txId,omitempty"`
	ExplorerURL string         `json:"explorerUrl,omitempty"`
	Message     string         `json:"message,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Error       *ResultError   `json:"error,omitempty"`
}

// ResultError is structured error detail for JSON output.
type ResultError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// Printer writes human or JSON results to stdout and logs to stderr.
type Printer struct {
	Mode   OutputMode
	Stdout io.Writer
	Stderr io.Writer
	Color  bool
}

// NewPrinter creates a printer using the given mode.
func NewPrinter(mode OutputMode) *Printer {
	return &Printer{
		Mode:   mode,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Color:  true,
	}
}

// Success writes a successful result.
func (p *Printer) Success(r Result) error {
	r.OK = true
	if err := p.write(r); err != nil {
		return err
	}
	p.logSuccess(r)
	return nil
}

// Failure writes a failure result.
func (p *Printer) Failure(r Result) error {
	r.OK = false
	return p.write(r)
}

func (p *Printer) write(r Result) error {
	switch p.Mode {
	case OutputJSON:
		enc := json.NewEncoder(p.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case OutputHuman, "":
		return p.writeHuman(r)
	default:
		return fmt.Errorf("unsupported output mode %q", p.Mode)
	}
}

func (p *Printer) writeHuman(r Result) error {
	var b strings.Builder
	if r.OK {
		if r.Message != "" {
			b.WriteString(r.Message)
			b.WriteByte('\n')
		} else {
			b.WriteString("ok\n")
		}
	} else {
		msg := "error"
		if r.Error != nil && r.Error.Message != "" {
			msg = r.Error.Message
		} else if r.Message != "" {
			msg = r.Message
		}
		b.WriteString(msg)
		b.WriteByte('\n')
	}
	if r.Operation != "" {
		fmt.Fprintf(&b, "operation: %s\n", r.Operation)
	}
	if r.Network != "" {
		fmt.Fprintf(&b, "network: %s\n", r.Network)
	}
	if r.Artifact != "" {
		fmt.Fprintf(&b, "artifact: %s\n", r.Artifact)
	}
	if r.TxID != "" {
		fmt.Fprintf(&b, "txId: %s\n", r.TxID)
	}
	if r.ExplorerURL != "" {
		fmt.Fprintf(&b, "explorer: %s\n", r.ExplorerURL)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", w)
	}
	for k, v := range r.Data {
		fmt.Fprintf(&b, "%s: %v\n", k, v)
	}
	_, err := io.WriteString(p.Stdout, b.String())
	return err
}

func (p *Printer) logSuccess(r Result) {
	attrs := []any{"ok", true}
	if r.Command != "" {
		attrs = append(attrs, "command", r.Command)
	}
	if r.Network != "" {
		attrs = append(attrs, "network", r.Network)
	}
	if r.Operation != "" {
		attrs = append(attrs, "operation", r.Operation)
	}
	if r.Artifact != "" {
		attrs = append(attrs, "artifact", r.Artifact)
	}
	if r.TxID != "" {
		attrs = append(attrs, "txId", r.TxID)
	}
	if r.ExplorerURL != "" {
		attrs = append(attrs, "explorerUrl", r.ExplorerURL)
	}
	if r.Message != "" {
		attrs = append(attrs, "message", r.Message)
	}
	if len(r.Warnings) > 0 {
		attrs = append(attrs, "warnings", r.Warnings)
	}
	if len(attrs) == 2 && r.Message == "" {
		slog.Info("command completed", attrs...)
		return
	}
	if r.Message != "" {
		slog.Info(r.Message, attrs...)
		return
	}
	slog.Info("command completed", attrs...)
}

// ParseOutputMode validates an output mode flag value.
func ParseOutputMode(s string) (OutputMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "human":
		return OutputHuman, nil
	case "json":
		return OutputJSON, nil
	default:
		return "", fmt.Errorf("invalid --output %q (want human|json)", s)
	}
}
