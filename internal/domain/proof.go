package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/blake2b"
)

// ProofBundle is the static Handshake proof for TLD registration/activation.
type ProofBundle struct {
	TLD                string `json:"tld"`
	OwnerPublicKey     string `json:"ownerPublicKey"`
	OwnerSignature     string `json:"ownerSignature"`
	RegistrarPublicKey string `json:"registrarPublicKey,omitempty"`
	RegistrarSignature string `json:"registrarSignature"`
}

// ParsedProof holds decoded proof bytes.
type ParsedProof struct {
	TLD                Label
	OwnerPublicKey     []byte
	OwnerSignature     []byte
	RegistrarPublicKey []byte
	RegistrarSignature []byte
}

// LoadProofBundle loads and validates a proof JSON file.
func LoadProofBundle(path string, expectedTLD string) (ParsedProof, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ParsedProof{}, err
	}
	var b ProofBundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return ParsedProof{}, fmt.Errorf("parse proof: %w", err)
	}
	label, err := ParseLabel(b.TLD)
	if err != nil {
		return ParsedProof{}, fmt.Errorf("proof.tld: %w", err)
	}
	if expectedTLD != "" {
		want, err := ParseLabel(expectedTLD)
		if err != nil {
			return ParsedProof{}, err
		}
		if label.Canonical != want.Canonical {
			return ParsedProof{}, fmt.Errorf("proof.tld %q does not match requested %q", label.Canonical, want.Canonical)
		}
	}
	ownerPK, err := decodeKey(b.OwnerPublicKey, "ownerPublicKey")
	if err != nil {
		return ParsedProof{}, err
	}
	ownerSig, err := decodeSig(b.OwnerSignature, "ownerSignature")
	if err != nil {
		return ParsedProof{}, err
	}
	regSig, err := decodeSig(b.RegistrarSignature, "registrarSignature")
	if err != nil {
		return ParsedProof{}, err
	}
	var regPK []byte
	if b.RegistrarPublicKey != "" {
		regPK, err = decodeKey(b.RegistrarPublicKey, "registrarPublicKey")
		if err != nil {
			return ParsedProof{}, err
		}
	}
	p := ParsedProof{
		TLD:                label,
		OwnerPublicKey:     ownerPK,
		OwnerSignature:     ownerSig,
		RegistrarPublicKey: regPK,
		RegistrarSignature: regSig,
	}
	return p, nil
}

// ValidateProofForRegistration checks registrar proof fields required for register-tld.
func ValidateProofForRegistration(p ParsedProof, tld []byte) error {
	if len(p.RegistrarPublicKey) == 0 {
		return fmt.Errorf("registrarPublicKey is required for registration")
	}
	if err := VerifyTLDSignature(p.RegistrarPublicKey, tld, p.RegistrarSignature); err != nil {
		return fmt.Errorf("invalid registrar signature: %w", err)
	}
	return nil
}

// ValidateProofForActivation checks owner proof fields required for activate-tld.
func ValidateProofForActivation(p ParsedProof, tld []byte) error {
	if err := VerifyTLDSignature(p.OwnerPublicKey, tld, p.OwnerSignature); err != nil {
		return fmt.Errorf("invalid owner signature: %w", err)
	}
	return nil
}

// VerifyTLDSignature mirrors utils.verify_tld_signature: ECDSA-secp256k1(pk, blake2b_256(tld), sig).
func VerifyTLDSignature(publicKey, tld, signature []byte) error {
	msg := Blake2b256(tld)
	pub, err := secp256k1.ParsePubKey(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}
	sig, err := ecdsa.ParseDERSignature(signature)
	if err != nil {
		// Try compact 64-byte R||S used by some Handshake tooling / script fixtures.
		if len(signature) == 64 {
			r := new(secp256k1.ModNScalar)
			s := new(secp256k1.ModNScalar)
			if overflow := r.SetByteSlice(signature[:32]); overflow {
				return fmt.Errorf("invalid compact signature R")
			}
			if overflow := s.SetByteSlice(signature[32:]); overflow {
				return fmt.Errorf("invalid compact signature S")
			}
			sig = ecdsa.NewSignature(r, s)
		} else {
			return fmt.Errorf("invalid signature: %w", err)
		}
	}
	if !sig.Verify(msg, pub) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func decodeKey(s, field string) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(s), "0x"))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid hex", field)
	}
	// Compressed secp256k1 pubkey is 33 bytes; some fixtures include uncompressed (65).
	if len(b) != 33 && len(b) != 65 {
		return nil, fmt.Errorf("%s: want 33 or 65 bytes, got %d", field, len(b))
	}
	return b, nil
}

func decodeSig(s, field string) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(s), "0x"))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid hex", field)
	}
	if len(b) < 64 {
		return nil, fmt.Errorf("%s: too short (%d)", field, len(b))
	}
	return b, nil
}

// Blake2b256 hashes msg with BLAKE2b-256 (Handshake TLD message digest).
func Blake2b256(msg []byte) []byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		sum := sha256.Sum256(msg)
		return sum[:]
	}
	h.Write(msg)
	return h.Sum(nil)
}

// Fingerprint returns a short non-secret hex prefix for display.
func Fingerprint(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := Blake2b256(b)
	return hex.EncodeToString(sum[:4])
}
