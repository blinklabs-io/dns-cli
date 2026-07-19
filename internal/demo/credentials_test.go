package demo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "(not set)" {
		t.Fatalf("empty: %q", got)
	}
	got := maskSecret("preprodABCDEFGH1234")
	if !strings.HasPrefix(got, "prep") || !strings.HasSuffix(got, "1234") || !strings.Contains(got, "…") {
		t.Fatalf("unexpected mask: %q", got)
	}
	if strings.Contains(got, "ABCDEF") {
		t.Fatalf("mask leaked middle: %q", got)
	}
}

func TestSettingPromptSource(t *testing.T) {
	if got := settingPrompt("TLD", "alice", "alice", "demo"); got != "TLD · last run" {
		t.Fatalf("last run: %q", got)
	}
	if got := settingPrompt("SLD", "www", "", "www"); got != "SLD · default" {
		t.Fatalf("default: %q", got)
	}
}

func TestEnsureBlockfrostKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "preprodTESTSECRETVALUE99")
	t.Setenv("DNS_CLI_BLOCKFROST_URL", "")

	var out bytes.Buffer
	r := &Runner{
		opts:   Options{Yes: true, SkipInstall: false, SkipInstallSet: true},
		paths:  Paths{EnvFile: envPath},
		stdin:  strings.NewReader(""),
		stdout: &out,
	}
	r.prompt = NewPrompter(r.stdin, r.stdout, true, false)
	if err := r.ensureBlockfrostCredentials(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{
		"Provider credentials · blockfrost",
		"https://blockfrost.io/dashboard",
		"https://cardano-preprod.blockfrost.io/api/v0",
		"DNS_CLI_BLOCKFROST_PROJECT_ID",
		"prep…UE99",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "TESTSECRET") {
		t.Fatalf("secret leaked:\n%s", s)
	}
}

func TestEnsureBlockfrostMissingWithSkipInstall(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "")
	t.Setenv("DNS_CLI_BLOCKFROST_URL", "")
	var out bytes.Buffer
	r := &Runner{
		opts:   Options{SkipInstall: true, SkipInstallSet: true},
		paths:  Paths{EnvFile: filepath.Join(t.TempDir(), ".env")},
		stdin:  strings.NewReader(""),
		stdout: &out,
	}
	r.prompt = NewPrompter(r.stdin, r.stdout, false, false)
	err := r.ensureBlockfrostCredentials()
	if err == nil || !strings.Contains(err.Error(), "DNS_CLI_BLOCKFROST_PROJECT_ID") {
		t.Fatalf("expected missing project id error, got %v", err)
	}
	if !strings.Contains(out.String(), "dashboard") {
		t.Fatalf("expected credentials panel before error:\n%s", out.String())
	}
}

func TestPromptSecretVarSavesNew(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "")
	var out bytes.Buffer
	r := &Runner{
		opts:   Options{SkipInstall: false, SkipInstallSet: true},
		paths:  Paths{EnvFile: envPath},
		stdin:  strings.NewReader("preprodNEWID12345678\n"),
		stdout: &out,
	}
	r.prompt = NewPrompter(r.stdin, r.stdout, false, false)
	if err := r.promptSecretVar("DNS_CLI_BLOCKFROST_PROJECT_ID", "Blockfrost id", "", true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "DNS_CLI_BLOCKFROST_PROJECT_ID=preprodNEWID12345678") {
		t.Fatalf("env not saved: %s", raw)
	}
}
