package wallet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blinklabs-io/bursa"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func TestGenerateWalletKeyEnvelopeRoundTrip(t *testing.T) {
	outDir := t.TempDir()
	generated, err := GenerateWallet(GenerateOptions{
		Name:    "demo",
		Network: "preprod",
		Format:  FormatKeyEnvelope,
		OutDir:  outDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if generated.Address == "" {
		t.Fatal("expected address")
	}
	for _, name := range []string{"payment.skey", "payment.vkey", "stake.skey", "stake.vkey", "payment.addr"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	env, err := ReadKeyEnvelope(filepath.Join(outDir, "payment.skey"))
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != TypePaymentExtendedSigningKey {
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
	if len(keyBytes) != 128 {
		t.Fatalf("expected 128-byte extended payment key, got %d", len(keyBytes))
	}

	src := &Source{
		Name:           "demo",
		Address:        mustAddress(t, generated.Address),
		SigningKeyFile: filepath.Join(outDir, "payment.skey"),
		Network:        "preprod",
	}
	w, err := src.LoadWallet()
	if err != nil {
		t.Fatal(err)
	}
	if w.Address().String() != generated.Address {
		t.Fatalf("address mismatch: got %s want %s", w.Address().String(), generated.Address)
	}
}

func TestGenerateWalletBothFormat(t *testing.T) {
	outDir := t.TempDir()
	generated, err := GenerateWallet(GenerateOptions{
		Name:    "both",
		Network: "preprod",
		Format:  FormatBoth,
		OutDir:  outDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "mnemonic.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "payment.skey")); err != nil {
		t.Fatal(err)
	}
	if generated.PaymentKeyHash == "" || generated.StakeKeyHash == "" {
		t.Fatal("expected key hashes")
	}
}

func TestGenerateWalletMnemonicFormat(t *testing.T) {
	outDir := t.TempDir()
	_, err := GenerateWallet(GenerateOptions{
		Name:    "mnemonic",
		Network: "preprod",
		Format:  FormatMnemonic,
		OutDir:  outDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "mnemonic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record MnemonicRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record.Mnemonic == "" {
		t.Fatal("expected mnemonic in record")
	}
	if record.BursaNetwork != "preprod" {
		t.Fatalf("expected preprod bursa network, got %q", record.BursaNetwork)
	}
}

func TestKeyEnvelopeRoundTrip(t *testing.T) {
	kf, err := bursa.GetPaymentSKey(testPaymentKey(t))
	if err != nil {
		t.Fatal(err)
	}
	env := KeyEnvelopeFromBursa(kf)
	out := filepath.Join(t.TempDir(), "payment.skey")
	if err := WriteKeyEnvelope(out, env); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadKeyEnvelope(out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Type != env.Type || loaded.CBORHex != env.CBORHex {
		t.Fatal("envelope round-trip mismatch")
	}
}

func TestGenerateWalletRejectsExistingWithoutForce(t *testing.T) {
	outDir := t.TempDir()
	opts := GenerateOptions{
		Name:    "demo",
		Network: "preprod",
		Format:  FormatKeyEnvelope,
		OutDir:  outDir,
	}
	if _, err := GenerateWallet(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateWallet(opts); err == nil {
		t.Fatal("expected error when wallet already exists")
	}
}

func testPaymentKey(t *testing.T) []byte {
	t.Helper()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	rootKey, err := bursa.GetRootKeyFromMnemonic(mnemonic, "")
	if err != nil {
		t.Fatal(err)
	}
	accountKey, err := bursa.GetAccountKey(rootKey, 0)
	if err != nil {
		t.Fatal(err)
	}
	paymentKey, err := bursa.GetPaymentKey(accountKey, 0)
	if err != nil {
		t.Fatal(err)
	}
	return paymentKey
}

func mustAddress(t *testing.T, addr string) common.Address {
	t.Helper()
	a, err := common.NewAddress(addr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
