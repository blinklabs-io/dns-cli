package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLevelFromVerbose(t *testing.T) {
	tests := []struct {
		n    int
		want slog.Level
		err  bool
	}{
		{0, slog.LevelError, false},
		{1, slog.LevelWarn, false},
		{2, slog.LevelInfo, false},
		{3, slog.LevelDebug, false},
		{4, LevelTrace, false},
		{-1, 0, true},
		{9, 0, true},
	}
	for _, tc := range tests {
		got, err := LevelFromVerbose(tc.n)
		if tc.err {
			if err == nil {
				t.Fatalf("LevelFromVerbose(%d): want error", tc.n)
			}
			continue
		}
		if err != nil {
			t.Fatalf("LevelFromVerbose(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Fatalf("LevelFromVerbose(%d) = %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestConfigureNoColorPlainText(t *testing.T) {
	var buf bytes.Buffer
	if err := Configure(Options{Verbose: 2, NoColor: true, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	slog.Info("hello", "key", "value")
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected plain text, got ANSI: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected log message in output: %q", out)
	}
}

func TestConfigureVerboseFiltersDebug(t *testing.T) {
	var buf bytes.Buffer
	if err := Configure(Options{Verbose: 2, NoColor: true, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	slog.Debug("hidden")
	slog.Info("visible")
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatalf("verbose 2 should filter debug: %q", out)
	}
	if !strings.Contains(out, "visible") {
		t.Fatalf("verbose 2 should allow info: %q", out)
	}
}

func TestConfigureVerboseAllowsDebug(t *testing.T) {
	var buf bytes.Buffer
	if err := Configure(Options{Verbose: 3, NoColor: true, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	slog.Debug("debug-line")
	if !strings.Contains(buf.String(), "debug-line") {
		t.Fatalf("verbose 3 should allow debug: %q", buf.String())
	}
}

func TestConfigureJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := Configure(Options{Verbose: 2, Output: "json", Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	slog.Info("json-msg", "n", 1)
	out := buf.String()
	if !strings.Contains(out, `"msg":"json-msg"`) && !strings.Contains(out, `"msg": "json-msg"`) {
		t.Fatalf("expected JSON log: %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("JSON handler should not emit ANSI: %q", out)
	}
}
