// Package config loads and validates dns-cli JSON profiles.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

// Document is the on-disk JSON configuration root.
type Document struct {
	Version        int                `json:"version"`
	DefaultProfile string             `json:"defaultProfile"`
	Profiles       map[string]Profile `json:"profiles"`
}

// Profile is a named network/provider deployment profile.
type Profile struct {
	Network     NetworkConfig          `json:"network"`
	Provider    ProviderConfig         `json:"provider"`
	Contracts   ContractsConfig        `json:"contracts"`
	Actors      map[string]ActorConfig `json:"actors"`
	Transaction TransactionConfig      `json:"transaction"`
}

// NetworkConfig describes Cardano network parameters.
type NetworkConfig struct {
	Name          string `json:"name"`
	ID            uint8  `json:"id"`
	Magic         int    `json:"magic"`
	ExplorerTxURL string `json:"explorerTxURL"`
}

// ProviderConfig selects and configures a chain backend.
type ProviderConfig struct {
	Type         string `json:"type"`
	BaseURL      string `json:"baseURL,omitempty"`
	BaseURLEnv   string `json:"baseUrlEnv,omitempty"`
	HeadersEnv   string `json:"headersEnv,omitempty"`
	ProjectIDEnv string `json:"projectIdEnv,omitempty"`
}

// ContractsConfig points at blueprint and deployed script references.
type ContractsConfig struct {
	BlueprintPath        string            `json:"blueprintPath"`
	TLDRegistrarAddress  string            `json:"tldRegistrarAddress"`
	TLDReferenceAddress  string            `json:"tldReferenceAddress"`
	SLDReferenceAddress  string            `json:"sldReferenceAddress"`
	TLDRegistrarPolicyID string            `json:"tldRegistrarPolicyId"`
	TLDReferencePolicyID string            `json:"tldReferencePolicyId"`
	SLDReferencePolicyID string            `json:"sldReferencePolicyId"`
	ReferenceUtxos       map[string]string `json:"referenceUtxos"`
}

// ActorConfig describes a signing actor. Exactly one of SigningKeyFile or MnemonicEnv may be set.
type ActorConfig struct {
	Address        string `json:"address"`
	SigningKeyFile string `json:"signingKeyFile,omitempty"`
	MnemonicEnv    string `json:"mnemonicEnv,omitempty"`
	AccountID      uint32 `json:"accountId,omitempty"`
	AddressID      uint32 `json:"addressId,omitempty"`
}

// TransactionConfig controls TTL and confirmation polling.
type TransactionConfig struct {
	TTLSlots            int64  `json:"ttlSlots"`
	ConfirmationTimeout string `json:"confirmationTimeout"`
	PollInterval        string `json:"pollInterval"`
	ArtifactDir         string `json:"artifactDir"`
	MaxDatumBytes       int    `json:"maxDatumBytes,omitempty"`
}

// Overrides are non-secret CLI flag overrides.
type Overrides struct {
	Network     string
	Provider    string
	ArtifactDir string
}

// Effective is a resolved profile ready for commands.
type Effective struct {
	Path    string
	Profile Profile
	Name    string
}

// Load reads JSON config and applies overrides.
func Load(path string, o Overrides) (*Effective, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var doc Document
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if doc.Version != SchemaVersion {
		return nil, fmt.Errorf("profiles.%s: unsupported config version %d (want %d)", doc.DefaultProfile, doc.Version, SchemaVersion)
	}
	name := doc.DefaultProfile
	if o.Network != "" {
		name = o.Network
	}
	if name == "" {
		return nil, fmt.Errorf("defaultProfile is required")
	}
	prof, ok := doc.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	if o.Provider != "" {
		prof.Provider.Type = o.Provider
	}
	if o.ArtifactDir != "" {
		prof.Transaction.ArtifactDir = o.ArtifactDir
	}
	if prof.Network.Name == "" {
		prof.Network.Name = name
	}
	eff := &Effective{Path: path, Profile: prof, Name: name}
	if err := ResolveRelativePaths(eff); err != nil {
		return nil, err
	}
	if err := ValidateOffline(eff); err != nil {
		return nil, err
	}
	return eff, nil
}

// ResolveRelativePaths resolves blueprint, signing-key, and artifact paths
// against the config file directory (not the process working directory).
func ResolveRelativePaths(eff *Effective) error {
	if eff == nil {
		return fmt.Errorf("nil effective config")
	}
	absPath, err := filepath.Abs(eff.Path)
	if err != nil {
		return fmt.Errorf("resolve config path %s: %w", eff.Path, err)
	}
	eff.Path = absPath
	baseDir := filepath.Dir(absPath)

	c := &eff.Profile.Contracts
	c.BlueprintPath = resolveAgainst(baseDir, c.BlueprintPath)

	for name, a := range eff.Profile.Actors {
		a.SigningKeyFile = resolveAgainst(baseDir, a.SigningKeyFile)
		eff.Profile.Actors[name] = a
	}

	t := &eff.Profile.Transaction
	t.ArtifactDir = resolveAgainst(baseDir, t.ArtifactDir)
	return nil
}

func resolveAgainst(baseDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Clean(filepath.Join(baseDir, p))
}

// DefaultDocument builds a starter config for a network/provider pair.
func DefaultDocument(network, provider string) (*Document, error) {
	net, err := networkDefaults(network)
	if err != nil {
		return nil, err
	}
	prov, err := providerDefaults(provider, network)
	if err != nil {
		return nil, err
	}
	profile := Profile{
		Network:  net,
		Provider: prov,
		Contracts: ContractsConfig{
			BlueprintPath: "../dns-contracts/onchain/plutus.json",
			ReferenceUtxos: map[string]string{
				"tldRegistrar": "REPLACE_ME_TXHASH#0",
				"tldReference": "REPLACE_ME_TXHASH#1",
				"sldReference": "REPLACE_ME_TXHASH#2",
			},
		},
		Actors: map[string]ActorConfig{
			"registrar": {Address: "addr_test1...", SigningKeyFile: "keys/registrar.skey"},
			"tldOwner":  {Address: "addr_test1...", MnemonicEnv: "DNS_CLI_TLD_OWNER_MNEMONIC"},
			"sldOwner":  {Address: "addr_test1...", SigningKeyFile: "keys/sld-owner.skey"},
		},
		Transaction: TransactionConfig{
			TTLSlots:            300,
			ConfirmationTimeout: "20m",
			PollInterval:        "5s",
			ArtifactDir:         "artifacts",
			MaxDatumBytes:       4000,
		},
	}
	return &Document{
		Version:        SchemaVersion,
		DefaultProfile: network,
		Profiles:       map[string]Profile{network: profile},
	}, nil
}

func networkDefaults(name string) (NetworkConfig, error) {
	switch strings.ToLower(name) {
	case "preview":
		return NetworkConfig{
			Name:          "preview",
			ID:            0,
			Magic:         2,
			ExplorerTxURL: "https://preview.cexplorer.io/tx/{txId}",
		}, nil
	case "preprod":
		return NetworkConfig{
			Name:          "preprod",
			ID:            0,
			Magic:         1,
			ExplorerTxURL: "https://preprod.cexplorer.io/tx/{txId}",
		}, nil
	default:
		return NetworkConfig{}, fmt.Errorf("unsupported network %q (want preview|preprod)", name)
	}
}

func providerDefaults(provider, network string) (ProviderConfig, error) {
	switch strings.ToLower(provider) {
	case "utxorpc":
		return ProviderConfig{
			Type:       "utxorpc",
			BaseURL:    "https://example.invalid",
			HeadersEnv: "DNS_CLI_UTXORPC_HEADERS",
		}, nil
	case "blockfrost":
		base := "https://cardano-preview.blockfrost.io/api/v0"
		if network == "preprod" {
			base = "https://cardano-preprod.blockfrost.io/api/v0"
		}
		return ProviderConfig{
			Type:         "blockfrost",
			BaseURL:      base,
			ProjectIDEnv: "DNS_CLI_BLOCKFROST_PROJECT_ID",
		}, nil
	default:
		return ProviderConfig{}, fmt.Errorf("unsupported provider %q (want utxorpc|blockfrost)", provider)
	}
}

// ConfirmationTimeout parses the profile timeout.
func (t TransactionConfig) ConfirmationTimeoutDuration() (time.Duration, error) {
	if t.ConfirmationTimeout == "" {
		return 20 * time.Minute, nil
	}
	return time.ParseDuration(t.ConfirmationTimeout)
}

// PollIntervalDuration parses the profile poll interval.
func (t TransactionConfig) PollIntervalDuration() (time.Duration, error) {
	if t.PollInterval == "" {
		return 5 * time.Second, nil
	}
	return time.ParseDuration(t.PollInterval)
}

// RedactedView returns a map safe for display/logging.
func RedactedView(eff *Effective, redact bool) map[string]any {
	p := eff.Profile
	actors := map[string]any{}
	for name, a := range p.Actors {
		entry := map[string]any{"address": a.Address}
		if a.SigningKeyFile != "" {
			entry["signingKeyFile"] = a.SigningKeyFile
		}
		if a.MnemonicEnv != "" {
			if redact {
				entry["mnemonicEnv"] = a.MnemonicEnv
				entry["mnemonic"] = "[redacted]"
			} else {
				entry["mnemonicEnv"] = a.MnemonicEnv
			}
		}
		actors[name] = entry
	}
	provider := map[string]any{
		"type": p.Provider.Type,
	}
	if p.Provider.BaseURLEnv != "" {
		provider["baseUrlEnv"] = p.Provider.BaseURLEnv
		if redact {
			provider["baseURL"] = "[redacted]"
		} else if p.Provider.BaseURL != "" {
			provider["baseURL"] = p.Provider.BaseURL
		}
	} else if p.Provider.BaseURL != "" {
		provider["baseURL"] = p.Provider.BaseURL
	}
	if p.Provider.HeadersEnv != "" {
		provider["headersEnv"] = p.Provider.HeadersEnv
		if redact {
			provider["headers"] = "[redacted]"
		}
	}
	if p.Provider.ProjectIDEnv != "" {
		provider["projectIdEnv"] = p.Provider.ProjectIDEnv
		if redact {
			provider["projectId"] = "[redacted]"
		}
	}
	return map[string]any{
		"path":        eff.Path,
		"profile":     eff.Name,
		"network":     p.Network,
		"provider":    provider,
		"contracts":   p.Contracts,
		"actors":      actors,
		"transaction": p.Transaction,
	}
}
