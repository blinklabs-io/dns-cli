package domain

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// Values from dns-contracts/hns-sig/secp256k1_output.json (hello-handshake).
const (
	helloHandshakePrivHex = "69131f21dcd7f83e6012100adeb70111912c64976c6af985bd662e8eb4e9f5b1"
	helloHandshakePubHex  = "035c43ce6ca8c0947c7c1e3a792df724eb1dd8f5940933969ec1c490d2a1d7b066"
	helloHandshakeHashHex = "4c85a7c5eb1f33d1abb38fdf6882b0346e700a7b8216799ae7030367afe70684"
	helloHandshakeSigHex  = "72c4e48ec2317de8260eea4ed060e6d55ac81c22981e3170e0d51b9c55c0a1bc0dafea71ebb9e2d95d391b48efb8627da7a473353442a5907d9ba3b921d565da"
)

func TestBlake2b256HelloHandshake(t *testing.T) {
	got := hex.EncodeToString(Blake2b256([]byte("hello-handshake")))
	if got != helloHandshakeHashHex {
		t.Fatalf("hash got %s want %s", got, helloHandshakeHashHex)
	}
}

func TestSignMessageHelloHandshake(t *testing.T) {
	privBytes, err := hex.DecodeString(helloHandshakePrivHex)
	if err != nil {
		t.Fatal(err)
	}
	priv := secp256k1.PrivKeyFromBytes(privBytes)
	tld := []byte("hello-handshake")

	sig, err := SignMessage(priv, tld)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length %d want 64", len(sig))
	}
	if hex.EncodeToString(sig) != helloHandshakeSigHex {
		t.Fatalf("signature got %s want %s", hex.EncodeToString(sig), helloHandshakeSigHex)
	}

	pub, err := hex.DecodeString(helloHandshakePubHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyTLDSignature(pub, tld, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSignedHNSKeyFileHelloHandshake(t *testing.T) {
	privBytes, err := hex.DecodeString(helloHandshakePrivHex)
	if err != nil {
		t.Fatal(err)
	}
	priv := secp256k1.PrivKeyFromBytes(privBytes)
	tld := []byte("hello-handshake")

	f, err := SignedHNSKeyFile(priv, tld)
	if err != nil {
		t.Fatal(err)
	}
	if f.PrivateKey != helloHandshakePrivHex {
		t.Fatalf("privateKey mismatch")
	}
	if f.PublicKey != helloHandshakePubHex {
		t.Fatalf("publicKey got %s want %s", f.PublicKey, helloHandshakePubHex)
	}
	if f.MessageHash != helloHandshakeHashHex {
		t.Fatalf("messageHash got %s want %s", f.MessageHash, helloHandshakeHashHex)
	}
	if f.Signature != helloHandshakeSigHex {
		t.Fatalf("signature got %s want %s", f.Signature, helloHandshakeSigHex)
	}
}

func TestLoadHNSKeyFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.hns")
	want := HNSKeyFile{
		PrivateKey:  helloHandshakePrivHex,
		PublicKey:   helloHandshakePubHex,
		MessageHash: helloHandshakeHashHex,
		Signature:   helloHandshakeSigHex,
	}
	raw, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	priv, loaded, err := LoadHNSKeyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PrivateKey != want.PrivateKey {
		t.Fatalf("loaded privateKey mismatch")
	}
	sig, err := SignMessage(priv, []byte("hello-handshake"))
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(sig) != helloHandshakeSigHex {
		t.Fatalf("re-sign mismatch")
	}
}

func TestGenerateProofBundle(t *testing.T) {
	dir := t.TempDir()
	out, err := GenerateProofBundle("hello-handshake", dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{out.RegistrarHNSPath, out.OwnerHNSPath, out.ProofBundlePath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	proof, err := LoadProofBundle(out.ProofBundlePath, "hello-handshake")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateProofForRegistration(proof, proof.TLD.Bytes); err != nil {
		t.Fatalf("registration proof: %v", err)
	}
}

func TestValidateProofForRegistrationRequiresRegistrarKey(t *testing.T) {
	p := ParsedProof{
		OwnerPublicKey:     mustDecodeHex(t, helloHandshakePubHex),
		RegistrarSignature: mustDecodeHex(t, helloHandshakeSigHex),
	}
	if err := ValidateProofForRegistration(p, []byte("hello-handshake")); err == nil {
		t.Fatal("expected error without registrarPublicKey")
	}
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
