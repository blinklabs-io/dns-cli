package system

import (
	"encoding/json"
	"fmt"
	"os"
)

// Role names stored in DeploymentJSON.Validators.
const (
	RoleTLDRegistrar   = "tldRegistrar"
	RoleTLDReference   = "tldReference"
	RoleSLDReference   = "sldReference"
	RoleRegistrarToken = "registrarToken"
)

// Validator names / modules within the Aiken project.
const (
	ModuleTLDRegistrar   = "tld_registration/tld_registrar"
	ModuleTLDReference   = "tld_registration/tld_reference"
	ModuleSLDReference   = "tld_registration/sld_reference"
	ModuleRegistrarToken = "tld_registration/registrar_nft"

	ValidatorTLDRegistrar   = "tld_registrar"
	ValidatorTLDReference   = "tld_reference"
	ValidatorSLDReference   = "sld_reference"
	ValidatorRegistrarToken = "registrar_token"
)

// RegistrarTokenAssetName is the fixed asset name minted for the registrar
// NFT. tld_registrar only checks the policy id is present in outputs
// (policy_id_present_in_outputs), not any specific asset name, so this is
// a dns-cli convention rather than an on-chain requirement.
const RegistrarTokenAssetName = "registrar"

// DeploymentJSON is written by system prepare and consumed by init/bind.
// mint-registrar-token runs before prepare (tld_registrar is parameterized
// by the registrar NFT's policy id, which only exists once minted) and
// bootstraps this same file with just the registrarToken validator entry;
// prepare then loads and extends it.
type DeploymentJSON struct {
	Version       int                          `json:"version"`
	Network       string                       `json:"network"`
	NetworkID     uint8                        `json:"networkId"`
	Magic         int                          `json:"magic"`
	StakeKeyHash  string                       `json:"stakeKeyHash,omitempty"`
	BlueprintPath string                       `json:"blueprintPath,omitempty"`
	OutDir        string                       `json:"outDir,omitempty"`
	Validators    map[string]ValidatorArtifact `json:"validators"`
}

// ValidatorArtifact describes one applied validator deployment unit.
type ValidatorArtifact struct {
	Role          string `json:"role"`
	Module        string `json:"module"`
	Validator     string `json:"validator"`
	PolicyID      string `json:"policyId"`
	ScriptHash    string `json:"scriptHash"`
	Address       string `json:"address"`
	PlutusFile    string `json:"plutusFile"`
	BlueprintFile string `json:"blueprintFile,omitempty"`
	// AssetNameHex is set for one-shot minting policies (e.g. registrarToken)
	// where the artifact represents a specific minted token, not a spend address.
	AssetNameHex string `json:"assetNameHex,omitempty"`
	// RegistrarTokenPolicyID is set on the tldRegistrar entry to the
	// registrar_token policy id it was parameterized against, so callers can
	// detect a deployment.json prepared before a registrar-token rotation
	// (or before this scheme existed at all) instead of trusting presence
	// alone.
	RegistrarTokenPolicyID string `json:"registrarTokenPolicyId,omitempty"`
}

// LoadDeploymentJSON reads deployment.json from path.
func LoadDeploymentJSON(path string) (*DeploymentJSON, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d DeploymentJSON
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse deployment.json: %w", err)
	}
	if d.Validators == nil {
		d.Validators = map[string]ValidatorArtifact{}
	}
	return &d, nil
}

// LoadOrInitDeploymentJSON loads deployment.json at path if present,
// otherwise initializes a fresh preprod deployment anchored to blueprint/outDir.
func LoadOrInitDeploymentJSON(path, blueprint, outDir string) (*DeploymentJSON, error) {
	if _, err := os.Stat(path); err == nil {
		return LoadDeploymentJSON(path)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &DeploymentJSON{
		Version:       1,
		Network:       "preprod",
		NetworkID:     0,
		Magic:         1,
		BlueprintPath: blueprint,
		OutDir:        outDir,
		Validators:    map[string]ValidatorArtifact{},
	}, nil
}

// SaveDeploymentJSON writes deployment.json.
func SaveDeploymentJSON(path string, d *DeploymentJSON) error {
	if d.Version == 0 {
		d.Version = 1
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RequireRoles ensures the three system validators are present.
func (d *DeploymentJSON) RequireRoles() error {
	for _, role := range []string{RoleTLDRegistrar, RoleTLDReference, RoleSLDReference} {
		v, ok := d.Validators[role]
		if !ok {
			return fmt.Errorf("deployment missing validator role %q", role)
		}
		if v.PolicyID == "" || v.Address == "" || v.PlutusFile == "" {
			return fmt.Errorf("deployment role %q incomplete", role)
		}
	}
	return nil
}

// RequireRegistrarToken ensures the registrar NFT has been minted and recorded.
func (d *DeploymentJSON) RequireRegistrarToken() error {
	v, ok := d.Validators[RoleRegistrarToken]
	if !ok {
		return fmt.Errorf("deployment missing validator role %q", RoleRegistrarToken)
	}
	if v.PolicyID == "" || v.AssetNameHex == "" {
		return fmt.Errorf("deployment role %q incomplete", RoleRegistrarToken)
	}
	return nil
}
