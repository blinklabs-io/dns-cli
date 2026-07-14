package system

import (
	"encoding/json"
	"fmt"
	"os"
)

// Role names stored in DeploymentJSON.Validators.
const (
	RoleTLDRegistrar = "tldRegistrar"
	RoleTLDReference = "tldReference"
	RoleSLDReference = "sldReference"
)

// Validator names / modules within the Aiken project.
const (
	ModuleTLDRegistrar = "tld_registration/tld_registrar"
	ModuleTLDReference = "tld_registration/tld_reference"
	ModuleSLDReference = "tld_registration/sld_reference"

	ValidatorTLDRegistrar = "tld_registrar"
	ValidatorTLDReference = "tld_reference"
	ValidatorSLDReference = "sld_reference"
)

// DeploymentJSON is written by system prepare and consumed by init/bind.
type DeploymentJSON struct {
	Version         int                          `json:"version"`
	Network         string                       `json:"network"`
	NetworkID       uint8                        `json:"networkId"`
	Magic           int                          `json:"magic"`
	StakeKeyHash    string                       `json:"stakeKeyHash"`
	RegistrarHNSKey string                       `json:"registrarHnsKey"`
	BlueprintPath   string                       `json:"blueprintPath,omitempty"`
	OutDir          string                       `json:"outDir,omitempty"`
	Validators      map[string]ValidatorArtifact `json:"validators"`
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
