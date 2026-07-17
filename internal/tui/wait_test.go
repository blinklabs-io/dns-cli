package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/blinklabs-io/dns-cli/internal/logging"
)

func TestNewWaitReporterForcePlain(t *testing.T) {
	var buf bytes.Buffer
	r := NewWaitReporter(WaitOptions{ForcePlain: true, Writer: &buf})
	p := logging.WaitProgress{
		Stage:     "tx.confirm",
		Process:   "waiting",
		TxID:      "abcd",
		Poll:      1,
		StartedAt: time.Now(),
		Deadline:  time.Now().Add(time.Minute),
	}
	r.Tick(p)
	r.Done(p, nil)
	out := buf.String()
	if !strings.Contains(out, "tx.confirm") && !strings.Contains(out, "abcd") {
		t.Fatalf("plain reporter output missing fields: %q", out)
	}
}

func TestWaitModelViewContainsFields(t *testing.T) {
	m := waitModel{
		progress: logging.WaitProgress{
			Stage:     "tx.confirm",
			Process:   "waiting for outputs",
			TxID:      "deadbeef",
			Poll:      7,
			StartedAt: time.Now().Add(-5 * time.Second),
			Deadline:  time.Now().Add(time.Minute),
			Indexes:   []uint32{0, 1},
		},
	}
	s := m.View().Content
	for _, want := range []string{"tx.confirm", "deadbeef", "#7", "waiting", "dns-cli wait"} {
		if !strings.Contains(s, want) {
			t.Fatalf("view missing %q in %q", want, s)
		}
	}
}

func TestRenderWaitViewNoSecrets(t *testing.T) {
	s := renderWaitView(logging.WaitProgress{
		Stage:     "tx.confirm",
		TxID:      "abc",
		StartedAt: time.Now(),
	}, false, nil, false)
	lower := strings.ToLower(s)
	if strings.Contains(lower, "mnemonic") || strings.Contains(lower, "seed phrase") {
		t.Fatalf("wait view must not mention secrets: %q", s)
	}
	if !strings.Contains(s, "◆ dns-cli wait") {
		t.Fatalf("expected report-styled wait panel: %q", s)
	}
}

func TestWaitModelCancelKeyQuits(t *testing.T) {
	m := waitModel{}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Fatal("expected Quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}
