package prereq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContractsOK(t *testing.T) {
	dir := t.TempDir()
	blueprint := filepath.Join(dir, "plutus.json")
	if ContractsOK(blueprint) {
		t.Fatal("missing file should fail")
	}
	mustWrite(t, blueprint, "{}")
	if !ContractsOK(blueprint) {
		t.Fatal("expected OK")
	}
}

func TestMissingDemoAssets(t *testing.T) {
	dir := t.TempDir()
	missing := MissingDemoAssets(dir)
	if len(missing) == 0 {
		t.Fatal("expected missing assets")
	}
	mustWrite(t, filepath.Join(dir, "config", "records.json"), "{}")
	mustWrite(t, filepath.Join(dir, "config", "blockfrost.template.json"), "{}")
	mustWrite(t, filepath.Join(dir, "config", "utxorpc.template.json"), "{}")
	mustWrite(t, filepath.Join(dir, "fixtures", "contracts", "plutus.json"), "{}")
	if got := MissingDemoAssets(dir); len(got) != 0 {
		t.Fatalf("expected complete, still missing %v", got)
	}
}

func TestVersionAtLeast(t *testing.T) {
	ok, err := versionAtLeast("1.1.19", "1.1.19")
	if err != nil || !ok {
		t.Fatalf("got %v %v", ok, err)
	}
	ok, err = versionAtLeast("1.1.18", "1.1.19")
	if err != nil || ok {
		t.Fatalf("expected false, got %v %v", ok, err)
	}
	ok, err = versionAtLeast("1.2.0", "1.1.19")
	if err != nil || !ok {
		t.Fatalf("got %v %v", ok, err)
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
