package wallet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadKeyEnvelopeFromFixture(t *testing.T) {
	path := filepath.Join("..", "..", "demo", "fixtures", "preprod", "wallets", "user1.skey")
	if _, err := os.Stat(path); err != nil {
		t.Skip("demo fixture not available")
	}
	env, err := ReadKeyEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != TypePaymentSigningKey {
		t.Fatalf("unexpected type %q", env.Type)
	}
	cborBytes, err := DecodeKeyEnvelopeCBOR(env)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := ExtractKeyBytes(cborBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(keyBytes) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(keyBytes))
	}
}

func TestGenerateWalletRejectsNonPreprod(t *testing.T) {
	_, err := GenerateWallet(GenerateOptions{
		Name:    "demo",
		Network: "preview",
		Format:  FormatKeyEnvelope,
		OutDir:  t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for non-preprod network")
	}
}
