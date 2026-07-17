package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

func TestWriteHumanPanelNoColor(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Mode: OutputHuman, Stdout: &buf, Color: false}
	if err := p.writeHuman(Result{
		OK:        true,
		Message:   "built unsigned tx",
		Command:   "owner mint-sld",
		Operation: "owner.mint_sld",
		Network:   "preprod",
		Artifact:  "out.unsigned.json",
		TxID:      "abc",
		Data:      map[string]any{"z": 1, "a": 2},
		Warnings:  []string{"slow provider"},
	}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if report.HasANSI(out) {
		t.Fatalf("unexpected ANSI: %q", out)
	}
	for _, want := range []string{"◆ built unsigned tx", "artifact", "txId", "warning:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "z") || !strings.Contains(out, "2") || !strings.Contains(out, "1") {
		t.Fatalf("missing sorted data keys in:\n%s", out)
	}
	// Stable Data key order: a before z.
	if ai, zi := strings.Index(out, "\na"), strings.Index(out, "\nz"); ai < 0 || zi < 0 || ai > zi {
		t.Fatalf("expected data key a before z in:\n%s", out)
	}
}

func TestWriteHumanColorHasANSI(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Mode: OutputHuman, Stdout: &buf, Color: true}
	if err := p.writeHuman(Result{OK: true, Message: "ok"}); err != nil {
		t.Fatal(err)
	}
	if !report.HasANSI(buf.String()) {
		t.Fatalf("expected ANSI when Color=true: %q", buf.String())
	}
}

func TestFormatVersionHuman(t *testing.T) {
	out := formatVersionHuman(VersionInfo{
		Version: "1.2.3", GitCommit: "deadbeef", BuildDate: "today",
		GoVersion: "go1.25", ApolloRevision: "v2", ContractRevision: "rev",
	}, false)
	if report.HasANSI(out) {
		t.Fatalf("unexpected ANSI: %q", out)
	}
	if !strings.Contains(out, "dns-cli 1.2.3") || !strings.Contains(out, "deadbeef") {
		t.Fatalf("bad version panel:\n%s", out)
	}
}
