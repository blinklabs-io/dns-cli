package report

import (
	"strings"
	"testing"
)

func TestSplashNoColorPlain(t *testing.T) {
	out := New(false).Splash("Handshake DNS on Cardano · Preprod demo", "fresh", "blockfrost", "www.demo")
	if HasANSI(out) {
		t.Fatalf("expected no ANSI, got %q", out)
	}
	if !strings.Contains(out, "dns-cli") && !strings.Contains(out, "____") {
		t.Fatalf("missing splash art: %q", out)
	}
	if !strings.Contains(out, "[fresh]") || !strings.Contains(out, "[blockfrost]") {
		t.Fatalf("missing plain chips: %q", out)
	}
}

func TestSplashColorHasANSI(t *testing.T) {
	out := New(true).Splash("subtitle", "fresh")
	if !HasANSI(out) {
		t.Fatalf("expected ANSI when color enabled: %q", out)
	}
}

func TestRoadmapStatuses(t *testing.T) {
	out := New(false).Roadmap("Preflight · www.demo", "provider blockfrost", []RoadmapItem{
		{Label: "fund", Detail: "done", Status: StatusDone},
		{Label: "register", Detail: "← next", Status: StatusCurrent},
		{Label: "activate", Status: StatusPending},
	})
	if HasANSI(out) {
		t.Fatalf("unexpected ANSI: %q", out)
	}
	for _, want := range []string{"◆ Preflight", "● fund", "◉ register", "○ activate"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStepAndNote(t *testing.T) {
	th := New(false)
	step := th.Step("4/7 Register TLD", "Claim the TLD.", "dns-cli registrar register-tld")
	if !strings.Contains(step, "DEMO") || !strings.Contains(step, "Equivalent CLI:") {
		t.Fatalf("bad step:\n%s", step)
	}
	note := th.Note("Skip fund — already confirmed")
	if !strings.Contains(note, "DEMO ·") {
		t.Fatalf("bad note: %q", note)
	}
}

func TestResultPanelSortedData(t *testing.T) {
	rows := SortedDataRows(map[string]any{"z": 1, "a": 2})
	if len(rows) != 2 || rows[0].Key != "a" || rows[1].Key != "z" {
		t.Fatalf("unsorted: %#v", rows)
	}
	out := New(false).ResultPanel(true, "built", []KV{{Key: "artifact", Value: "out.json"}}, nil)
	if !strings.Contains(out, "◆ built") || !strings.Contains(out, "artifact") {
		t.Fatalf("bad panel:\n%s", out)
	}
}

func TestTxLineMissing(t *testing.T) {
	out := New(false).TxLine("fund", "", "https://example/")
	if !strings.Contains(out, "(not confirmed)") {
		t.Fatalf("expected missing tx: %q", out)
	}
}
