package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func exampleConfigPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/config -> repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "config", "dns-cli.example.json")
}

func TestExampleConfigOfflineValid(t *testing.T) {
	path := exampleConfigPath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	body := string(raw)
	for _, secret := range []string{
		"preprodXXXXXXXX",
		"mainnetXXXXXXXX",
		"dmtr_api_",
		"sk_",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("example must not contain secret-looking value %q", secret)
		}
	}
	for _, envName := range []string{
		"DNS_CLI_BLOCKFROST_PROJECT_ID",
		"DNS_CLI_UTXORPC_URL",
		"DNS_CLI_UTXORPC_HEADERS",
	} {
		if !strings.Contains(body, envName) {
			t.Fatalf("example missing env name %q", envName)
		}
	}

	var doc Document
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("parse example: %v", err)
	}
	if doc.DefaultProfile != "preprod-blockfrost" {
		t.Fatalf("defaultProfile: got %q", doc.DefaultProfile)
	}
	for _, name := range []string{"preprod-blockfrost", "preprod-utxorpc"} {
		eff, err := Load(path, Overrides{Network: name})
		if err != nil {
			t.Fatalf("Load(%s): %v", name, err)
		}
		if err := ValidateOffline(eff); err != nil {
			t.Fatalf("ValidateOffline(%s): %v", name, err)
		}
	}
}

func TestApplyStarterRelativePathsConfigDir(t *testing.T) {
	doc, err := DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join("config", "dns-cli.json")
	if err := ApplyStarterRelativePaths(doc, out); err != nil {
		t.Fatal(err)
	}
	p := doc.Profiles["preprod"]
	if got := filepath.ToSlash(p.Contracts.BlueprintPath); got != "../../dns-contracts/onchain/plutus.json" {
		t.Fatalf("blueprint: got %q", got)
	}
	if got := filepath.ToSlash(p.Transaction.ArtifactDir); got != "../artifacts" {
		t.Fatalf("artifactDir: got %q", got)
	}
	if got := filepath.ToSlash(p.Actors["registrar"].SigningKeyFile); got != "../keys/registrar.skey" {
		t.Fatalf("registrar key: got %q", got)
	}
}

func TestApplyStarterRelativePathsRootConfig(t *testing.T) {
	doc, err := DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyStarterRelativePaths(doc, "dns-cli.json"); err != nil {
		t.Fatal(err)
	}
	p := doc.Profiles["preprod"]
	if got := filepath.ToSlash(p.Contracts.BlueprintPath); got != "../dns-contracts/onchain/plutus.json" {
		t.Fatalf("blueprint: got %q", got)
	}
	if got := filepath.ToSlash(p.Transaction.ArtifactDir); got != "artifacts" {
		t.Fatalf("artifactDir: got %q", got)
	}
	if got := filepath.ToSlash(p.Actors["registrar"].SigningKeyFile); got != "keys/registrar.skey" {
		t.Fatalf("registrar key: got %q", got)
	}
}
