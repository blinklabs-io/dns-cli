package demo

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleEntries() []ResumeEntry {
	return []ResumeEntry{
		{TLD: "alpha", SLD: "www", RunID: "20260101-120000", Provider: "blockfrost", Stage: StageFund, Resumable: true},
		{TLD: "alpha", SLD: "app", RunID: "20260102-120000", Provider: "blockfrost", Stage: StageComplete, Resumable: false},
		{TLD: "beta", SLD: "www", RunID: "20260103-120000", Provider: "utxorpc", Stage: StageMintSLD, Resumable: true},
	}
}

func TestSelectResumeEntryAcceptsNumber(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("3\n")
	got, err := SelectResumeEntry(in, &out, sampleEntries(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.TLD != "beta" || got.RunID != "20260103-120000" {
		t.Fatalf("got %#v", got)
	}
}

func TestSelectResumeEntryRejectsCompletedThenAccepts(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("2\n1\n")
	got, err := SelectResumeEntry(in, &out, sampleEntries(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "20260101-120000" {
		t.Fatalf("got %#v", got)
	}
	if !strings.Contains(out.String(), "complete and cannot be resumed") {
		t.Fatalf("expected complete warning:\n%s", out.String())
	}
}

func TestSelectResumeEntryInvalidReprompts(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("x\n0\n1\n")
	got, err := SelectResumeEntry(in, &out, sampleEntries(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.SLD != "www" || got.TLD != "alpha" {
		t.Fatalf("got %#v", got)
	}
	if !strings.Contains(out.String(), "Invalid choice") {
		t.Fatalf("expected invalid message:\n%s", out.String())
	}
}

func TestSelectResumeEntryCancelQ(t *testing.T) {
	var out bytes.Buffer
	_, err := SelectResumeEntry(strings.NewReader("q\n"), &out, sampleEntries(), false)
	if !errors.Is(err, ErrResumeCancelled) {
		t.Fatalf("want cancelled, got %v", err)
	}
}

func TestSelectResumeEntryCancelEOF(t *testing.T) {
	var out bytes.Buffer
	_, err := SelectResumeEntry(strings.NewReader(""), &out, sampleEntries(), false)
	if !errors.Is(err, ErrResumeCancelled) {
		t.Fatalf("want cancelled, got %v", err)
	}
}

func TestLoadSelectedLayoutExactRun(t *testing.T) {
	root := t.TempDir()
	// Shared TLD state + two SLD run dirs (newest is lexicographically later).
	writeResumeFixture(t, root, "alpha", "www", "20260101-111111", "blockfrost",
		map[string]string{"fund": "aaa", "deploy": "bbb"}, map[string]string{"mintSld": "mint-old"}, false)
	// Second run under same TLD (do not rewrite TLD state).
	runDir := filepath.Join(root, "alpha", "www", "20260102-222222")
	mustMkdir(t, runDir)
	mustWrite(t, filepath.Join(runDir, "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "blockfrost",
		"tld": "alpha",
		"sld": "www",
		"runId": "20260102-222222",
		"confirmed": {
			"mintSld": {"txId": "", "manifest": ""},
			"updateSld": {"txId": "", "manifest": ""}
		}
	}`)

	r := &Runner{paths: Paths{RunsRoot: root}}
	entry := ResumeEntry{
		TLD: "alpha", SLD: "www", RunID: "20260101-111111",
		Provider: "blockfrost", Stage: StageUpdateSLD, Resumable: true,
	}
	if err := r.loadSelectedLayout(entry); err != nil {
		t.Fatal(err)
	}
	if r.runID != "20260101-111111" {
		t.Fatalf("runID=%q want older exact run", r.runID)
	}
	if r.sldState.stepTxID("mintSld") != "mint-old" {
		t.Fatalf("loaded wrong SLD state mint=%q", r.sldState.stepTxID("mintSld"))
	}
	newer := filepath.Join(root, "alpha", "www", "20260103-999999")
	if _, err := os.Stat(newer); !os.IsNotExist(err) {
		t.Fatal("unexpected new run dir")
	}
}

func TestLoadSelectedLayoutUsesStoredProvider(t *testing.T) {
	root := t.TempDir()
	writeResumeFixture(t, root, "alpha", "www", "20260101-120000", "utxorpc",
		map[string]string{"fund": "aaa"}, map[string]string{}, false)
	r := &Runner{
		paths:    Paths{RunsRoot: root},
		provider: "blockfrost", // pre-selection default must be ignored
	}
	entry := ResumeEntry{
		TLD: "alpha", SLD: "www", RunID: "20260101-120000",
		Provider: "utxorpc", Stage: StageDeploy, Resumable: true,
	}
	if err := r.loadSelectedLayout(entry); err != nil {
		t.Fatal(err)
	}
	if r.provider != "utxorpc" {
		t.Fatalf("provider=%q", r.provider)
	}
}
