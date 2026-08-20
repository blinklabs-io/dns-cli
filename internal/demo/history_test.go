package demo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	h, err := ReadHistory(filepath.Join(dir, "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.TLDs) != 0 {
		t.Fatalf("want empty history, got %#v", h)
	}
	if got := FormatHistoryHuman(h, false); !strings.Contains(got, "no demo history yet") {
		t.Fatalf("unexpected human output: %q", got)
	}
}

func TestReadHistorySkipsSharedAndStates(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "shared"))
	mustMkdir(t, filepath.Join(root, "states"))
	mustWrite(t, filepath.Join(root, "shared", "state.json"), `{"schemaVersion":2}`)
	mustMkdir(t, filepath.Join(root, "cardano"))
	mustWrite(t, filepath.Join(root, "cardano", "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "blockfrost",
		"tld": "cardano",
		"confirmed": {
			"mintRegistrarToken": {"txId": "zzz", "manifest": "m0"},
			"fund": {"txId": "aaa", "manifest": "m1"},
			"deploy": {"txId": "", "manifest": ""},
			"register": {"txId": "", "manifest": ""},
			"activate": {"txId": "", "manifest": ""}
		}
	}`)
	mustMkdir(t, filepath.Join(root, "cardano", "artifacts"))
	mustMkdir(t, filepath.Join(root, "cardano", "www", "20260101-120000"))
	mustWrite(t, filepath.Join(root, "cardano", "www", "20260101-120000", "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "blockfrost",
		"tld": "cardano",
		"sld": "www",
		"runId": "20260101-120000",
		"confirmed": {
			"mintSld": {"txId": "bbb", "manifest": "m2"},
			"updateSld": {"txId": "ccc", "manifest": "m3"}
		}
	}`)

	h, err := ReadHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.TLDs) != 1 {
		t.Fatalf("want 1 tld, got %d", len(h.TLDs))
	}
	if h.TLDs[0].Confirmed["fund"].ExplorerURL != ExplorerURLPrefix+"aaa" {
		t.Fatalf("unexpected explorer: %#v", h.TLDs[0].Confirmed["fund"])
	}
	if h.TLDs[0].Confirmed["mintRegistrarToken"].ExplorerURL != ExplorerURLPrefix+"zzz" {
		t.Fatalf("unexpected mintRegistrarToken explorer: %#v", h.TLDs[0].Confirmed["mintRegistrarToken"])
	}
	if len(h.TLDs[0].Runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(h.TLDs[0].Runs))
	}
	if h.TLDs[0].Runs[0].Status != "complete" {
		t.Fatalf("want complete, got %s", h.TLDs[0].Runs[0].Status)
	}
	human := FormatHistoryHuman(h, false)
	if !strings.Contains(human, "TLD cardano") || !strings.Contains(human, "update-sld") || !strings.Contains(human, "ccc") {
		t.Fatalf("unexpected human output: %s", human)
	}
	if !strings.Contains(human, "mint-registrar-token") || !strings.Contains(human, "zzz") {
		t.Fatalf("expected mint-registrar-token step in human output: %s", human)
	}
	if !strings.Contains(human, "explorer") {
		t.Fatalf("expected explorer lines in:\n%s", human)
	}
}

func TestReadHistoryRejectsBadSchema(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "bad"))
	mustWrite(t, filepath.Join(root, "bad", "state.json"), `{"schemaVersion":1,"mode":"fresh","network":"preprod","provider":"blockfrost","tld":"bad","confirmed":{"fund":{"txId":"","manifest":""},"deploy":{"txId":"","manifest":""},"register":{"txId":"","manifest":""},"activate":{"txId":"","manifest":""}}}`)
	_, err := ReadHistory(root)
	if err == nil {
		t.Fatal("expected schema version error")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
