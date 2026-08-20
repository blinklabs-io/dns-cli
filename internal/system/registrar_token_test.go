package system_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/system"
)

func TestPrepareRegistrarTokenWithFakeAiken(t *testing.T) {
	tmp := t.TempDir()
	blueprintDir := filepath.Join(tmp, "contracts")
	if err := os.MkdirAll(blueprintDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bp := map[string]any{
		"preamble": map[string]any{"plutusVersion": "v3", "title": "test"},
		"validators": []any{
			map[string]any{
				"title":        "tld_registration/registrar_nft.registrar_token.mint",
				"compiledCode": "010100",
				"hash":         strings.Repeat("dd", 28),
			},
		},
	}
	raw, _ := json.MarshalIndent(bp, "", "  ")
	blueprintPath := filepath.Join(blueprintDir, "plutus.json")
	if err := os.WriteFile(blueprintPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	fake := NewFakeRunner()
	outDir := filepath.Join(tmp, "out")
	txHash := make([]byte, 32)
	for i := range txHash {
		txHash[i] = byte(i)
	}
	result, err := system.PrepareRegistrarToken(context.Background(), system.RegistrarTokenOptions{
		Blueprint: blueprintPath,
		TxHash:    txHash,
		TxIndex:   0,
		OutDir:    outDir,
		Runner:    fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PolicyID == "" {
		t.Fatal("empty policy id")
	}
	if result.AssetNameHex == "" {
		t.Fatal("empty asset name")
	}
	if _, err := os.Stat(result.PlutusFile); err != nil {
		t.Fatalf("missing plutus file: %v", err)
	}
	if fake.ApplyN != 1 {
		t.Fatalf("apply calls %d want 1", fake.ApplyN)
	}

	if _, err := system.PrepareRegistrarToken(context.Background(), system.RegistrarTokenOptions{
		Blueprint: blueprintPath,
		TxHash:    txHash[:31],
		OutDir:    outDir,
		Runner:    fake,
	}); err == nil {
		t.Fatal("expected tx hash length error")
	}
}
