package cli

import (
	"errors"
	"testing"
)

func TestExitCodeRequiredFlag(t *testing.T) {
	err := errors.New(`required flag(s) "actor" not set`)
	if got := ExitCode(err); got != ExitUsage {
		t.Fatalf("got %d want %d", got, ExitUsage)
	}
}

func TestUnsignedBuildData(t *testing.T) {
	data := unsignedBuildData("artifacts/02-register", map[string]any{"tld": "demo"})
	if data["out"] != "artifacts/02-register" {
		t.Fatalf("out: %v", data["out"])
	}
	if data["unsigned"] != "artifacts/02-register.unsigned.json" {
		t.Fatalf("unsigned: %v", data["unsigned"])
	}
	if data["manifest"] != "artifacts/02-register.manifest.json" {
		t.Fatalf("manifest: %v", data["manifest"])
	}
	if data["tld"] != "demo" {
		t.Fatalf("tld: %v", data["tld"])
	}
}

func TestFirstNonEmptyFlag(t *testing.T) {
	if got := firstNonEmptyFlag("", "signed.json"); got != "signed.json" {
		t.Fatalf("got %q", got)
	}
	if got := firstNonEmptyFlag("a", "b"); got != "a" {
		t.Fatalf("got %q", got)
	}
}
