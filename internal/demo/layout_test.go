package demo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLatestSLDRunIDAndComplete(t *testing.T) {
	root := t.TempDir()
	sldRoot := filepath.Join(root, "www")
	mustMkdir(t, filepath.Join(sldRoot, "20260101-100000"))
	mustMkdir(t, filepath.Join(sldRoot, "20260102-120000"))
	mustWrite(t, filepath.Join(sldRoot, "20260102-120000", "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "blockfrost",
		"tld": "cardano",
		"sld": "www",
		"runId": "20260102-120000",
		"confirmed": {
			"mintSld": {"txId": "aaa", "manifest": "m"},
			"updateSld": {"txId": "", "manifest": ""}
		}
	}`)
	latest, err := latestSLDRunID(sldRoot)
	if err != nil {
		t.Fatal(err)
	}
	if latest != "20260102-120000" {
		t.Fatalf("got %s", latest)
	}
	if sldRunComplete(filepath.Join(sldRoot, latest)) {
		t.Fatal("expected incomplete")
	}
}

func TestWriteJSONAtomicAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := newTLDState("demo", "fresh", "blockfrost")
	st.Confirmed.Fund = StepResult{TxID: "abc", Manifest: "m"}
	if err := writeJSONAtomic(path, st); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTLDState(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.stepTxID("fund") != "abc" {
		t.Fatalf("got %#v", loaded.Confirmed.Fund)
	}
}

func TestEnvFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	t.Setenv("DEMO_TEST_KEY", "")
	if err := SaveEnvVar(path, "DEMO_TEST_KEY", "value1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("DEMO_TEST_KEY"); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DEMO_TEST_KEY"); got != "value1" {
		t.Fatalf("got %q", got)
	}
}
