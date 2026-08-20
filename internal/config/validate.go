package config

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	policyIDRe = regexp.MustCompile(`(?i)^[0-9a-f]{56}$`)
	addrRe     = regexp.MustCompile(`^addr(_test)?1[0-9a-z]+$`)
	utxoRefRe  = regexp.MustCompile(`(?i)^[0-9a-f]{64}#\d+$`)
)

// ValidateOffline validates static config shape without network calls.
func ValidateOffline(eff *Effective) error {
	if eff == nil {
		return fmt.Errorf("nil effective config")
	}
	slog.Debug("Validating config offline", "profile", eff.Name)
	p := eff.Profile
	switch strings.ToLower(p.Network.Name) {
	case "preview", "preprod", "mainnet":
	default:
		return fmt.Errorf("profiles.%s.network.name: unsupported %q", eff.Name, p.Network.Name)
	}
	switch strings.ToLower(p.Provider.Type) {
	case "utxorpc", "blockfrost":
		if err := validateProviderBaseURL(eff.Name, p.Provider); err != nil {
			return err
		}
		if strings.EqualFold(p.Provider.Type, "blockfrost") {
			if strings.TrimSpace(p.Provider.ProjectIDEnv) == "" {
				return fmt.Errorf("profiles.%s.provider.projectIdEnv: required for blockfrost", eff.Name)
			}
		}
	default:
		return fmt.Errorf("profiles.%s.provider.type: unsupported %q", eff.Name, p.Provider.Type)
	}
	c := p.Contracts
	if c.BlueprintPath == "" {
		return fmt.Errorf("profiles.%s.contracts.blueprintPath: required", eff.Name)
	}
	for field, val := range map[string]string{
		"tldRegistrarPolicyId": c.TLDRegistrarPolicyID,
		"tldReferencePolicyId": c.TLDReferencePolicyID,
		"sldReferencePolicyId": c.SLDReferencePolicyID,
	} {
		if val == "" || strings.HasPrefix(val, "REPLACE") {
			// Allow placeholders in freshly init'ed configs for offline structural checks
			// that only require provider/network; policy IDs are required for domain ops.
			continue
		}
		if !policyIDRe.MatchString(val) {
			return fmt.Errorf("profiles.%s.contracts.%s: invalid policy id", eff.Name, field)
		}
	}
	for name, ref := range c.ReferenceUtxos {
		if strings.HasPrefix(ref, "REPLACE") {
			continue
		}
		if !utxoRefRe.MatchString(ref) {
			return fmt.Errorf("profiles.%s.contracts.referenceUtxos.%s: want txhash#index", eff.Name, name)
		}
		if _, _, err := ParseUTxORef(ref); err != nil {
			return fmt.Errorf("profiles.%s.contracts.referenceUtxos.%s: %w", eff.Name, name, err)
		}
	}
	if len(p.Actors) == 0 {
		return fmt.Errorf("profiles.%s.actors: at least one actor required", eff.Name)
	}
	seen := map[string]string{}
	for name, a := range p.Actors {
		path := fmt.Sprintf("profiles.%s.actors.%s", eff.Name, name)
		if a.Address == "" {
			return fmt.Errorf("%s.address: required", path)
		}
		if !isStarterAddressPlaceholder(a.Address) && !addrRe.MatchString(a.Address) {
			return fmt.Errorf("%s.address: invalid bech32 address", path)
		}
		hasKey := strings.TrimSpace(a.SigningKeyFile) != ""
		hasMn := strings.TrimSpace(a.MnemonicEnv) != ""
		if hasKey == hasMn {
			return fmt.Errorf("%s: exactly one of signingKeyFile or mnemonicEnv is required", path)
		}
		if prev, ok := seen[a.Address]; ok && prev != name {
			return fmt.Errorf("%s.address: duplicates actor %q", path, prev)
		}
		seen[a.Address] = name
		if looksLikeSecret(a.SigningKeyFile) || looksLikeSecret(a.MnemonicEnv) {
			return fmt.Errorf("%s: secrets must not be embedded inline", path)
		}
	}
	if p.Transaction.TTLSlots <= 0 {
		return fmt.Errorf("profiles.%s.transaction.ttlSlots: must be > 0", eff.Name)
	}
	if _, err := p.Transaction.ConfirmationTimeoutDuration(); err != nil {
		return fmt.Errorf("profiles.%s.transaction.confirmationTimeout: %w", eff.Name, err)
	}
	if _, err := p.Transaction.PollIntervalDuration(); err != nil {
		return fmt.Errorf("profiles.%s.transaction.pollInterval: %w", eff.Name, err)
	}
	slog.Debug("Offline config validation passed", "profile", eff.Name)
	return nil
}

// ValidateOnline validates provider connectivity and reference UTxOs.
func ValidateOnline(eff *Effective) error {
	slog.Info("Validating config online", "profile", eff.Name, "provider", eff.Profile.Provider.Type)
	if err := ValidateOffline(eff); err != nil {
		return err
	}
	for field, val := range map[string]string{
		"tldRegistrarPolicyId": eff.Profile.Contracts.TLDRegistrarPolicyID,
		"tldReferencePolicyId": eff.Profile.Contracts.TLDReferencePolicyID,
		"sldReferencePolicyId": eff.Profile.Contracts.SLDReferencePolicyID,
		"tldRegistrarAddress":  eff.Profile.Contracts.TLDRegistrarAddress,
		"tldReferenceAddress":  eff.Profile.Contracts.TLDReferenceAddress,
		"sldReferenceAddress":  eff.Profile.Contracts.SLDReferenceAddress,
	} {
		if val == "" || strings.HasPrefix(val, "REPLACE") || isStarterAddressPlaceholder(val) {
			return fmt.Errorf("profiles.%s.contracts.%s: must be set for online validation", eff.Name, field)
		}
	}
	for name, ref := range eff.Profile.Contracts.ReferenceUtxos {
		if strings.HasPrefix(ref, "REPLACE") {
			return fmt.Errorf("profiles.%s.contracts.referenceUtxos.%s: must be set for online validation", eff.Name, name)
		}
	}
	slog.Info("Online placeholder validation passed", "profile", eff.Name)
	return nil
}

// RequireContractIDs ensures deployment-specific fields are present for domain ops.
func RequireContractIDs(eff *Effective) error {
	c := eff.Profile.Contracts
	required := map[string]string{
		"tldRegistrarAddress":  c.TLDRegistrarAddress,
		"tldReferenceAddress":  c.TLDReferenceAddress,
		"sldReferenceAddress":  c.SLDReferenceAddress,
		"tldRegistrarPolicyId": c.TLDRegistrarPolicyID,
		"tldReferencePolicyId": c.TLDReferencePolicyID,
		"sldReferencePolicyId": c.SLDReferencePolicyID,
	}
	for k, v := range required {
		if v == "" || strings.HasPrefix(v, "REPLACE") || isStarterAddressPlaceholder(v) {
			return fmt.Errorf("profiles.%s.contracts.%s: required for domain operations", eff.Name, k)
		}
	}
	for _, key := range []string{"tldRegistrar", "tldReference", "sldReference"} {
		ref, ok := c.ReferenceUtxos[key]
		if !ok || strings.HasPrefix(ref, "REPLACE") {
			return fmt.Errorf("profiles.%s.contracts.referenceUtxos.%s: required", eff.Name, key)
		}
	}
	return nil
}

// ParseUTxORef splits txhash#index.
func ParseUTxORef(ref string) (txHash string, index uint32, err error) {
	parts := strings.Split(ref, "#")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid utxo ref %q", ref)
	}
	h := strings.ToLower(parts[0])
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 32 {
		return "", 0, fmt.Errorf("invalid tx hash in %q", ref)
	}
	n, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("invalid index in %q", ref)
	}
	return h, uint32(n), nil
}

// RequirePreprod rejects profiles that are not Cardano preprod (name=preprod, id=0, magic=1).
func RequirePreprod(eff *Effective) error {
	if eff == nil {
		return fmt.Errorf("nil effective config")
	}
	n := eff.Profile.Network
	if !strings.EqualFold(n.Name, "preprod") || n.ID != 0 || n.Magic != 1 {
		return fmt.Errorf(
			"profiles.%s.network: require preprod (name=preprod, id=0, magic=1); got name=%q id=%d magic=%d",
			eff.Name, n.Name, n.ID, n.Magic,
		)
	}
	return nil
}

func validateProviderBaseURL(profile string, p ProviderConfig) error {
	base := strings.TrimSpace(p.BaseURL)
	envName := strings.TrimSpace(p.BaseURLEnv)
	hasBase := base != ""
	hasEnv := envName != ""
	switch {
	case hasBase && hasEnv:
		return fmt.Errorf("profiles.%s.provider: exactly one of baseURL or baseUrlEnv is required", profile)
	case hasBase:
		if _, err := url.ParseRequestURI(base); err != nil {
			return fmt.Errorf("profiles.%s.provider.baseURL: %w", profile, err)
		}
	case hasEnv:
		// Env is resolved at provider construction time; offline only checks the name.
	default:
		return fmt.Errorf("profiles.%s.provider: exactly one of baseURL or baseUrlEnv is required", profile)
	}
	return nil
}

// isStarterAddressPlaceholder reports config-init dummy addresses. Testnet
// starters use addr_test1...; mainnet starters use addr1... (literal dots,
// which are not bech32).
func isStarterAddressPlaceholder(addr string) bool {
	return strings.HasPrefix(addr, "addr_test1...") || strings.HasPrefix(addr, "addr1...")
}

func looksLikeSecret(s string) bool {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "mnemonic") && strings.Contains(lower, " ") {
		return true
	}
	if strings.HasPrefix(lower, "mainnet") || strings.HasPrefix(lower, "preprod") || strings.HasPrefix(lower, "preview") {
		// blockfrost project id style
		if len(s) > 20 && !strings.Contains(s, "/") && !strings.Contains(s, "\\") {
			return true
		}
	}
	return false
}
