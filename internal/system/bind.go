package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

// BindOptions configures BindConfig.
type BindOptions struct {
	BaseConfigPath string
	DeploymentPath string
	TxID           string
	ActorDir       string
	Provider       string
	OutPath        string
	Force          bool
}

// BindConfig merges deployment + tx id + actor wallets into a runnable dns-cli config.
func BindConfig(opts BindOptions) (*config.Document, error) {
	if err := validateBindOpts(opts); err != nil {
		return nil, err
	}
	if _, err := os.Stat(opts.OutPath); err == nil && !opts.Force {
		return nil, fmt.Errorf("output config already exists at %s (use --force)", opts.OutPath)
	}

	dep, err := LoadDeploymentJSON(opts.DeploymentPath)
	if err != nil {
		return nil, err
	}
	if err := dep.RequireRoles(); err != nil {
		return nil, err
	}
	txID := strings.ToLower(strings.TrimSpace(opts.TxID))
	if len(txID) != 64 {
		return nil, fmt.Errorf("--tx-id must be 64 hex characters")
	}
	for _, c := range txID {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return nil, fmt.Errorf("--tx-id must be hex")
		}
	}

	raw, err := os.ReadFile(opts.BaseConfigPath)
	if err != nil {
		return nil, err
	}
	var doc config.Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse base config: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(doc.DefaultProfile)) == "" {
		doc.DefaultProfile = "preprod"
	}
	prof, ok := doc.Profiles[doc.DefaultProfile]
	if !ok {
		return nil, fmt.Errorf("base config missing profile %q", doc.DefaultProfile)
	}
	netName := strings.ToLower(strings.TrimSpace(prof.Network.Name))
	if netName == "" {
		netName = strings.ToLower(strings.TrimSpace(doc.DefaultProfile))
	}
	net, err := config.NetworkDefaults(netName)
	if err != nil {
		return nil, fmt.Errorf("system bind: %w", err)
	}
	prof.Network.Name = net.Name
	if prof.Network.ID == 0 && net.ID != 0 {
		prof.Network.ID = net.ID
	}
	if prof.Network.Magic == 0 {
		prof.Network.Magic = net.Magic
	}
	if prof.Network.ExplorerTxURL == "" {
		prof.Network.ExplorerTxURL = net.ExplorerTxURL
	}

	provider := strings.ToLower(strings.TrimSpace(opts.Provider))
	prof.Provider.Type = provider
	switch provider {
	case "blockfrost":
		if prof.Provider.BaseURL == "" {
			defaults, err := config.ProviderDefaults("blockfrost", prof.Network.Name)
			if err != nil {
				return nil, err
			}
			prof.Provider.BaseURL = defaults.BaseURL
		}
		if prof.Provider.ProjectIDEnv == "" {
			prof.Provider.ProjectIDEnv = "DNS_CLI_BLOCKFROST_PROJECT_ID"
		}
	case "utxorpc":
		if prof.Provider.HeadersEnv == "" {
			prof.Provider.HeadersEnv = "DNS_CLI_UTXORPC_HEADERS"
		}
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}

	reg := dep.Validators[RoleTLDRegistrar]
	tld := dep.Validators[RoleTLDReference]
	sld := dep.Validators[RoleSLDReference]
	prof.Contracts.TLDRegistrarAddress = reg.Address
	prof.Contracts.TLDReferenceAddress = tld.Address
	prof.Contracts.SLDReferenceAddress = sld.Address
	prof.Contracts.TLDRegistrarPolicyID = reg.PolicyID
	prof.Contracts.TLDReferencePolicyID = tld.PolicyID
	prof.Contracts.SLDReferencePolicyID = sld.PolicyID
	// registrarToken is optional here: mint-registrar-token may run before or
	// after bind, since it doesn't depend on the reference-script tx id.
	if token, ok := dep.Validators[RoleRegistrarToken]; ok {
		prof.Contracts.RegistrarTokenPolicyID = token.PolicyID
		prof.Contracts.RegistrarTokenAssetName = token.AssetNameHex
	}
	prof.Contracts.ReferenceUtxos = map[string]string{
		RoleTLDRegistrar: fmt.Sprintf("%s#%d", txID, 0),
		RoleTLDReference: fmt.Sprintf("%s#%d", txID, 1),
		RoleSLDReference: fmt.Sprintf("%s#%d", txID, 2),
	}
	// Prefer the prepared deployment blueprint. Template paths like
	// ../runtime/contracts/plutus.json are relative to demo/config/ and break
	// when the bound config is written under runtime/config/.
	if dep.BlueprintPath != "" {
		prof.Contracts.BlueprintPath = dep.BlueprintPath
	} else if strings.TrimSpace(prof.Contracts.BlueprintPath) == "" || strings.Contains(prof.Contracts.BlueprintPath, "GENERATED") {
		return nil, fmt.Errorf("deployment missing blueprintPath and base config has no usable blueprintPath")
	}

	if prof.Actors == nil {
		prof.Actors = map[string]config.ActorConfig{}
	}
	for _, name := range []string{"bootstrap", "registrar", "tldOwner", "sldOwner"} {
		actorDir := filepath.Join(opts.ActorDir, actorDirName(name))
		addr, keyPath, err := loadActorArtifacts(actorDir)
		if err != nil {
			return nil, fmt.Errorf("actor %s: %w", name, err)
		}
		existing := prof.Actors[name]
		existing.Address = addr
		existing.SigningKeyFile = keyPath
		existing.MnemonicEnv = ""
		prof.Actors[name] = existing
	}

	doc.Profiles[doc.DefaultProfile] = prof

	if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o755); err != nil && filepath.Dir(opts.OutPath) != "." {
		return nil, err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, '\n')
	tmp := opts.OutPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, opts.OutPath); err != nil {
		return nil, err
	}
	return &doc, nil
}

func actorDirName(name string) string {
	switch name {
	case "tldOwner":
		return "tld-owner"
	case "sldOwner":
		return "sld-owner"
	default:
		return name
	}
}

func loadActorArtifacts(dir string) (address, signingKey string, err error) {
	addrPath := filepath.Join(dir, "payment.addr")
	addrRaw, err := os.ReadFile(addrPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", addrPath, err)
	}
	address = strings.TrimSpace(string(addrRaw))
	if address == "" {
		return "", "", fmt.Errorf("empty address in %s", addrPath)
	}
	signingKey = filepath.Join(dir, "payment.skey")
	if _, err := os.Stat(signingKey); err != nil {
		return "", "", fmt.Errorf("missing payment.skey in %s", dir)
	}
	return address, signingKey, nil
}

func validateBindOpts(opts BindOptions) error {
	if opts.BaseConfigPath == "" {
		return fmt.Errorf("--base-config is required")
	}
	if opts.DeploymentPath == "" {
		return fmt.Errorf("--deployment is required")
	}
	if opts.TxID == "" {
		return fmt.Errorf("--tx-id is required")
	}
	if opts.ActorDir == "" {
		return fmt.Errorf("--actor-dir is required")
	}
	if opts.OutPath == "" {
		return fmt.Errorf("--out is required")
	}
	p := strings.ToLower(strings.TrimSpace(opts.Provider))
	if p != "blockfrost" && p != "utxorpc" {
		return fmt.Errorf("--provider must be blockfrost or utxorpc")
	}
	return nil
}
