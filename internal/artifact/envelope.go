// Package artifact stores Cardano text envelopes and manifests.
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	TypeUnwitnessedConway = "Unwitnessed Tx ConwayEra"
	TypeWitnessedConway   = "Witnessed Tx ConwayEra"
)

// Envelope is a Cardano CLI-compatible text envelope.
type Envelope struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	CBORHex     string `json:"cborHex"`
}

// Manifest records non-secret build metadata for offline ceremonies.
type Manifest struct {
	Version          int               `json:"version"`
	Operation        string            `json:"operation"`
	Network          string            `json:"network"`
	Provider         string            `json:"provider"`
	BodyHash         string            `json:"bodyHash"`
	RequiredSigners  []string          `json:"requiredSigners"`
	ExpectedOutputs  []ExpectedOutput  `json:"expectedOutputs"`
	ValidityInterval ValidityInterval  `json:"validityInterval"`
	ConfigDigest     string            `json:"configDigest"`
	ApolloRevision   string            `json:"apolloRevision"`
	ContractRevision string            `json:"contractRevision"`
	CreatedAt        string            `json:"createdAt"`
	Extra            map[string]string `json:"extra,omitempty"`
}

// ExpectedOutput describes a post-submit confirmation target.
type ExpectedOutput struct {
	Role  string `json:"role"`
	Index uint32 `json:"index"`
}

// ValidityInterval captures slot bounds.
type ValidityInterval struct {
	Start int64 `json:"start,omitempty"`
	TTL   int64 `json:"ttl,omitempty"`
}

// WriteUnsigned writes envelope + manifest next to outPrefix.
func WriteUnsigned(outPrefix string, cbor []byte, description string, m Manifest) (envelopePath string, err error) {
	dir := filepath.Dir(outPrefix)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
	}
	envPath := outPrefix + ".unsigned.json"
	manPath := outPrefix + ".manifest.json"
	env := Envelope{
		Type:        TypeUnwitnessedConway,
		Description: description,
		CBORHex:     hex.EncodeToString(cbor),
	}
	if err := writeJSON(envPath, env); err != nil {
		return "", err
	}
	m.Version = 1
	if m.CreatedAt == "" {
		m.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := writeJSON(manPath, m); err != nil {
		return "", err
	}
	slog.Info("Wrote unsigned artifact", "envelope", envPath, "manifest", manPath, "operation", m.Operation)
	slog.Debug("Artifact body hash", "bodyHash", m.BodyHash)
	return envPath, nil
}

// ReadEnvelope loads a text envelope.
func ReadEnvelope(path string) (*Envelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	if env.CBORHex == "" {
		return nil, fmt.Errorf("envelope missing cborHex")
	}
	return &env, nil
}

// ReadManifest loads a sibling or explicit manifest.
func ReadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// WriteEnvelope writes a (possibly signed) envelope atomically.
func WriteEnvelope(path string, env Envelope) error {
	return writeJSON(path, env)
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SiblingManifestPath derives the manifest path from an envelope path.
func SiblingManifestPath(envelopePath string) string {
	base := envelopePath
	for _, suf := range []string{".unsigned.json", ".signed.json", ".json"} {
		if strings.HasSuffix(base, suf) {
			return strings.TrimSuffix(base, suf) + ".manifest.json"
		}
	}
	return base + ".manifest.json"
}

// DigestConfig returns a SHA-256 of non-secret config bytes.
func DigestConfig(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// DecodeCBORHex decodes envelope CBOR.
func (e *Envelope) DecodeCBORHex() ([]byte, error) {
	return hex.DecodeString(e.CBORHex)
}
