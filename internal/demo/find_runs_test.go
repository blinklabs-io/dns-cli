package demo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRunsRootFromDemoDir(t *testing.T) {
	root := t.TempDir()
	demoDir := filepath.Join(root, "demo")
	runs := filepath.Join(demoDir, "runs")
	mustMkdir(t, filepath.Join(runs, "states"))
	mustWrite(t, filepath.Join(runs, ".gitkeep"), "")

	got, err := FindRunsRoot(demoDir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(runs)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindRunsRootFromModuleRoot(t *testing.T) {
	root := t.TempDir()
	cliRoot := filepath.Join(root, "dns-cli")
	runs := filepath.Join(cliRoot, "demo", "runs")
	mustMkdir(t, filepath.Join(runs, "states"))
	mustWrite(t, filepath.Join(runs, "ada", "state.json"), `{"schemaVersion":2,"mode":"fresh","network":"preprod","provider":"blockfrost","tld":"ada","confirmed":{"fund":{"txId":"","manifest":""},"deploy":{"txId":"","manifest":""},"register":{"txId":"","manifest":""},"activate":{"txId":"","manifest":""}}}`)

	got, err := FindRunsRoot(cliRoot)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(runs)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveRunsRootExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveRunsRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindRunsRootMissing(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	_, err = FindRunsRoot(dir)
	if err == nil {
		t.Fatal("expected error when runs/ missing")
	}
}
