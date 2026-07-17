package demo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

func TestGuideStepDistinctFromLogStyle(t *testing.T) {
	var buf bytes.Buffer
	g := guide{out: &buf, th: report.New(false)}
	g.Step("Mint SLD", "Create www under the activated TLD.",
		"dns-cli owner mint-sld --tld demo --sld www --sld-owner sldOwner --out artifacts/04-mint-sld",
	)
	out := buf.String()
	if !strings.Contains(out, "DEMO") {
		t.Fatalf("missing DEMO banner: %q", out)
	}
	if !strings.Contains(out, "Equivalent CLI:") {
		t.Fatalf("missing CLI section: %q", out)
	}
	if strings.Contains(out, "INF ") || strings.Contains(out, "slog") {
		t.Fatalf("guide should not look like slog: %q", out)
	}
	if report.HasANSI(out) {
		t.Fatalf("expected plain guide with color=false: %q", out)
	}
}

func TestGuideDoneListsAllTxLinks(t *testing.T) {
	var buf bytes.Buffer
	g := guide{out: &buf, th: report.New(false)}
	g.Done(CompletionReport{
		TLD:      "cardano",
		SLD:      "www",
		RunID:    "20260717-183808",
		Provider: "blockfrost",
		Explorer: ExplorerURLPrefix,
		Steps: []ReportStep{
			{Label: "fund", TxID: "aaa"},
			{Label: "deploy", TxID: "bbb"},
			{Label: "register", TxID: "ccc"},
			{Label: "activate", TxID: "ddd"},
			{Label: "mint-sld", TxID: "eee"},
			{Label: "update-sld", TxID: "fff"},
		},
	})
	out := buf.String()
	for _, want := range []string{
		"DEMO COMPLETE",
		"www.cardano",
		"fund", "deploy", "register", "activate", "mint-sld", "update-sld",
		ExplorerURLPrefix + "aaa",
		ExplorerURLPrefix + "fff",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatHistoryHumanRoadmap(t *testing.T) {
	out := FormatHistoryHumanAt(History{
		TLDs: []HistoryTLD{{
			TLD:      "demo",
			Provider: "blockfrost",
			Mode:     "fresh",
			Network:  "preprod",
			Confirmed: map[string]HistoryTx{
				"fund":     {TxID: "aaa", ExplorerURL: ExplorerURLPrefix + "aaa"},
				"deploy":   {},
				"register": {},
				"activate": {},
			},
		}},
	}, "/tmp/demo/runs", false)
	for _, want := range []string{
		"◆ Demo history · /tmp/demo/runs",
		"◆ TLD demo",
		"provider", "blockfrost",
		"fund", "aaa",
		"explorer", ExplorerURLPrefix + "aaa",
		"deploy", "(empty)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if report.HasANSI(out) {
		t.Fatalf("unexpected ANSI: %q", out)
	}
}
