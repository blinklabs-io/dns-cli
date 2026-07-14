package wallet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blinklabs-io/bursa"
)

const (
	TypePaymentSigningKey         = "PaymentSigningKeyShelley_ed25519"
	TypePaymentExtendedSigningKey = "PaymentExtendedSigningKeyShelley_ed25519_bip32"
	TypePaymentVKey               = "PaymentVerificationKeyShelley_ed25519"
	TypeStakeSigningKey           = "StakeSigningKeyShelley_ed25519"
	TypeStakeExtendedSigningKey   = "StakeExtendedSigningKeyShelley_ed25519_bip32"
	TypeStakeVKey                 = "StakeVerificationKeyShelley_ed25519"
)

// KeyEnvelope is a Cardano CLI-compatible signing or verification key text envelope.
type KeyEnvelope struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	CBORHex     string `json:"cborHex"`
}

// KeyEnvelopeFromBursa converts a bursa key file into a wallet key envelope.
func KeyEnvelopeFromBursa(kf bursa.KeyFile) KeyEnvelope {
	return KeyEnvelope{
		Type:        kf.Type,
		Description: kf.Description,
		CBORHex:     kf.CborHex,
	}
}

// ReadKeyEnvelope loads a Cardano text envelope from path.
func ReadKeyEnvelope(path string) (*KeyEnvelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseKeyEnvelope(raw)
}

// ParseKeyEnvelope unmarshals a Cardano text envelope from JSON bytes.
func ParseKeyEnvelope(raw []byte) (*KeyEnvelope, error) {
	var env KeyEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse key envelope: %w", err)
	}
	if env.CBORHex == "" {
		return nil, fmt.Errorf("key envelope missing cborHex")
	}
	return &env, nil
}

// WriteKeyEnvelope writes a key envelope atomically with owner-only permissions.
func WriteKeyEnvelope(path string, env KeyEnvelope) error {
	content, err := bursa.GetKeyFile(bursa.KeyFile{
		Type:        env.Type,
		Description: env.Description,
		CborHex:     env.CBORHex,
	})
	if err != nil {
		return fmt.Errorf("format key envelope: %w", err)
	}
	return writeSecretFile(path, []byte(content))
}

// WriteBursaKeyFile writes a bursa key file using Cardano CLI text-envelope formatting.
func WriteBursaKeyFile(path string, kf bursa.KeyFile) error {
	content, err := bursa.GetKeyFile(kf)
	if err != nil {
		return fmt.Errorf("format key file: %w", err)
	}
	return writeSecretFile(path, []byte(content))
}

// DecodeKeyEnvelopeCBOR decodes the CBOR hex payload from a key envelope.
func DecodeKeyEnvelopeCBOR(env *KeyEnvelope) ([]byte, error) {
	cborBytes, err := hex.DecodeString(env.CBORHex)
	if err != nil {
		return nil, fmt.Errorf("invalid cborHex: %w", err)
	}
	return cborBytes, nil
}

// ExtractKeyBytes extracts raw signing key bytes from Cardano CLI key CBOR.
func ExtractKeyBytes(cborBytes []byte) ([]byte, error) {
	if len(cborBytes) < 2 {
		return nil, fmt.Errorf("key CBOR too short")
	}
	mt := cborBytes[0] >> 5
	ai := cborBytes[0] & 0x1f
	if mt != 2 {
		if len(cborBytes) == 32 || len(cborBytes) == 64 || len(cborBytes) == 96 {
			return cborBytes, nil
		}
		return nil, fmt.Errorf("unexpected CBOR major type %d", mt)
	}
	var n int
	var off int
	switch {
	case ai < 24:
		n = int(ai)
		off = 1
	case ai == 24:
		if len(cborBytes) < 2 {
			return nil, fmt.Errorf("truncated CBOR")
		}
		n = int(cborBytes[1])
		off = 2
	case ai == 25:
		if len(cborBytes) < 3 {
			return nil, fmt.Errorf("truncated CBOR")
		}
		n = int(cborBytes[1])<<8 | int(cborBytes[2])
		off = 3
	default:
		return nil, fmt.Errorf("unsupported CBOR length")
	}
	if off+n > len(cborBytes) {
		return nil, fmt.Errorf("truncated key bytes")
	}
	return cborBytes[off : off+n], nil
}

func writeSecretFile(path string, content []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
