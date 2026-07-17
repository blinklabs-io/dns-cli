package demo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDemoRootFromModuleRoot(t *testing.T) {
	root := t.TempDir()
	demoDir := filepath.Join(root, "demo")
	mustMkdir(t, filepath.Join(demoDir, "config"))
	mustMkdir(t, filepath.Join(demoDir, "fixtures"))
	mustMkdir(t, filepath.Join(demoDir, "runs"))

	got, err := FindDemoRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(demoDir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindDemoRootFromInsideDemo(t *testing.T) {
	root := t.TempDir()
	demoDir := filepath.Join(root, "demo")
	mustMkdir(t, filepath.Join(demoDir, "config"))
	mustMkdir(t, filepath.Join(demoDir, "fixtures"))
	mustMkdir(t, filepath.Join(demoDir, "runs"))

	got, err := FindDemoRoot(demoDir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(demoDir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindDemoRootMissing(t *testing.T) {
	_, err := FindDemoRoot(t.TempDir())
	if err == nil {
		t.Fatal("expected error when demo/ missing")
	}
}

func TestResolveDemoRootExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveDemoRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLooksLikeDemoRootRequiresAllDirs(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "config"))
	mustMkdir(t, filepath.Join(dir, "fixtures"))
	if looksLikeDemoRoot(dir) {
		t.Fatal("should reject without runs/")
	}
	mustMkdir(t, filepath.Join(dir, "runs"))
	if !looksLikeDemoRoot(dir) {
		t.Fatal("expected true with config/fixtures/runs")
	}
	_ = os.RemoveAll(filepath.Join(dir, "config"))
}
