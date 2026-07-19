package logging

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatWaitBoxContainsFields(t *testing.T) {
	p := WaitProgress{
		Stage:       "tx.confirm",
		Process:     "waiting for outputs",
		TxID:        "abc123",
		ExplorerURL: "https://example/tx/abc123",
		Indexes:     []uint32{0, 1},
		Poll:        2,
		StartedAt:   time.Now().Add(-5 * time.Second),
		Deadline:    time.Now().Add(10 * time.Minute),
	}
	lines := formatWaitBox(p, false, nil)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"tx.confirm", "waiting for outputs", "#0,#1", "abc123", "https://example/tx/abc123", "timing", "poll #2", "dns-cli wait"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("box missing %q:\n%s", want, joined)
		}
	}
}

func TestStatusBoxPlainModePrintsNewlines(t *testing.T) {
	var buf bytes.Buffer
	box := NewStatusBox(StatusBoxOptions{Writer: &buf, ForcePlain: true, ExplorerEveryN: 1})
	p := WaitProgress{
		Stage:       "tx.confirm",
		TxID:        "deadbeef",
		ExplorerURL: "https://explorer/deadbeef",
		Poll:        1,
		StartedAt:   time.Now(),
		Deadline:    time.Now().Add(time.Minute),
	}
	box.Tick(p)
	out := buf.String()
	if !strings.Contains(out, "◆ wait") || !strings.Contains(out, "deadbeef") {
		t.Fatalf("expected wait line, got %q", out)
	}
	if !strings.Contains(out, "explorer: https://explorer/deadbeef") {
		t.Fatalf("expected explorer line, got %q", out)
	}
	box.Done(p, nil)
	if !strings.Contains(buf.String(), "confirmed") {
		t.Fatalf("expected confirmed Done line, got %q", buf.String())
	}
}

func TestFormatIndexes(t *testing.T) {
	if got := formatIndexes([]uint32{0, 2}); got != "[#0,#2]" {
		t.Fatalf("got %q", got)
	}
}
