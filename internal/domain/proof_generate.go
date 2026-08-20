package domain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// HNSKeyFile matches the hns-sig/secp256k1_output.json schema.
type HNSKeyFile struct {
	PrivateKey  string `json:"privateKey"`
	PublicKey   string `json:"publicKey"`
	MessageHash string `json:"messageHash,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

// ProofBundleOutput lists paths written by GenerateProofBundle.
type ProofBundleOutput struct {
	OwnerHNSPath    string
	ProofBundlePath string
}

// GenerateHNSKey creates a new secp256k1 key pair.
func GenerateHNSKey() (*secp256k1.PrivateKey, HNSKeyFile, error) {
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, HNSKeyFile{}, fmt.Errorf("generate private key: %w", err)
	}
	pub := priv.PubKey().SerializeCompressed()
	return priv, HNSKeyFile{
		PrivateKey: hex.EncodeToString(priv.Serialize()),
		PublicKey:  hex.EncodeToString(pub),
	}, nil
}

// LoadHNSKeyFile loads an HNS key JSON file and returns the parsed private key.
func LoadHNSKeyFile(path string) (*secp256k1.PrivateKey, HNSKeyFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, HNSKeyFile{}, err
	}
	var f HNSKeyFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, HNSKeyFile{}, fmt.Errorf("parse hns key: %w", err)
	}
	privHex := strings.TrimPrefix(strings.TrimSpace(f.PrivateKey), "0x")
	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, HNSKeyFile{}, fmt.Errorf("privateKey: invalid hex")
	}
	if len(privBytes) != 32 {
		return nil, HNSKeyFile{}, fmt.Errorf("privateKey: want 32 bytes, got %d", len(privBytes))
	}
	return secp256k1.PrivKeyFromBytes(privBytes), f, nil
}

// SignMessage signs blake2b_256(message) with privateKey and returns a
// compact 64-byte R||S signature. Used both for the legacy tld-alone
// message and for OwnerAction's tld+receiver_address+output_reference
// message — the caller decides what message means.
func SignMessage(privateKey *secp256k1.PrivateKey, message []byte) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	hash := Blake2b256(message)
	sig := ecdsa.Sign(privateKey, hash)
	r := sig.R()
	s := sig.S()
	var out [64]byte
	rBytes := (&r).Bytes()
	sBytes := (&s).Bytes()
	copy(out[:32], rBytes[:])
	copy(out[32:], sBytes[:])
	return out[:], nil
}

// SignedHNSKeyFile returns an HNSKeyFile with messageHash and signature for tldBytes.
func SignedHNSKeyFile(privateKey *secp256k1.PrivateKey, tldBytes []byte) (HNSKeyFile, error) {
	if privateKey == nil {
		return HNSKeyFile{}, fmt.Errorf("private key is nil")
	}
	sig, err := SignMessage(privateKey, tldBytes)
	if err != nil {
		return HNSKeyFile{}, err
	}
	pub := privateKey.PubKey().SerializeCompressed()
	return HNSKeyFile{
		PrivateKey:  hex.EncodeToString(privateKey.Serialize()),
		PublicKey:   hex.EncodeToString(pub),
		MessageHash: hex.EncodeToString(Blake2b256(tldBytes)),
		Signature:   hex.EncodeToString(sig),
	}, nil
}

// GenerateProofBundle signs tld with the owner key and writes owner.hns and
// proof-bundle.json into outDir. An empty key path generates a fresh key.
// Registrar authority is proven on-chain by holding the registrar NFT, not
// by a signature, so no registrar key is involved.
func GenerateProofBundle(tld, outDir, ownerKeyPath string) (ProofBundleOutput, error) {
	label, err := ParseLabel(tld)
	if err != nil {
		return ProofBundleOutput{}, err
	}
	ownerPriv, err := loadOrGenerateHNSKey(ownerKeyPath)
	if err != nil {
		return ProofBundleOutput{}, fmt.Errorf("owner key: %w", err)
	}
	ownerFile, err := SignedHNSKeyFile(ownerPriv, label.Bytes)
	if err != nil {
		return ProofBundleOutput{}, err
	}
	bundle := ProofBundle{
		TLD:            label.Canonical,
		OwnerPublicKey: ownerFile.PublicKey,
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return ProofBundleOutput{}, err
	}
	out := ProofBundleOutput{
		OwnerHNSPath:    filepath.Join(outDir, "owner.hns"),
		ProofBundlePath: filepath.Join(outDir, "proof-bundle.json"),
	}
	if err := writeHNSKeyFile(out.OwnerHNSPath, ownerFile); err != nil {
		return ProofBundleOutput{}, err
	}
	if err := writeProofBundle(out.ProofBundlePath, bundle); err != nil {
		return ProofBundleOutput{}, err
	}
	return out, nil
}

func loadOrGenerateHNSKey(path string) (*secp256k1.PrivateKey, error) {
	if path == "" {
		priv, _, err := GenerateHNSKey()
		return priv, err
	}
	priv, _, err := LoadHNSKeyFile(path)
	return priv, err
}

func writeHNSKeyFile(path string, f HNSKeyFile) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

func writeProofBundle(path string, b ProofBundle) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}
