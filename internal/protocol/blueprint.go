package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Blueprint is a minimal view of aiken plutus.json.
type Blueprint struct {
	Preamble   BlueprintPreamble    `json:"preamble"`
	Validators []BlueprintValidator `json:"validators"`
}

// BlueprintPreamble holds compiler metadata.
type BlueprintPreamble struct {
	Title         string `json:"title"`
	PlutusVersion string `json:"plutusVersion"`
	Compiler      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"compiler"`
}

// BlueprintValidator is one compiled validator entry.
type BlueprintValidator struct {
	Title        string `json:"title"`
	CompiledCode string `json:"compiledCode"`
	Hash         string `json:"hash"`
}

// LoadBlueprint loads and sanity-checks plutus.json.
func LoadBlueprint(path string) (*Blueprint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Blueprint
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parse blueprint: %w", err)
	}
	if !strings.EqualFold(b.Preamble.PlutusVersion, "v3") {
		return nil, fmt.Errorf("blueprint plutus version %q is not v3", b.Preamble.PlutusVersion)
	}
	return &b, nil
}

// FindValidator returns a validator by exact title.
func (b *Blueprint) FindValidator(title string) (*BlueprintValidator, error) {
	for i := range b.Validators {
		if b.Validators[i].Title == title {
			return &b.Validators[i], nil
		}
	}
	return nil, fmt.Errorf("validator %q not found in blueprint", title)
}

// Known validator titles from dns-contracts.
const (
	TitleTLDRegistrarMint   = "tld_registration/tld_registrar.tld_registrar.mint"
	TitleTLDRegistrarSpend  = "tld_registration/tld_registrar.tld_registrar.spend"
	TitleTLDReferenceMint   = "tld_registration/tld_reference.tld_reference.mint"
	TitleTLDReferenceSpend  = "tld_registration/tld_reference.tld_reference.spend"
	TitleSLDReferenceMint   = "tld_registration/sld_reference.sld_reference.mint"
	TitleSLDReferenceSpend  = "tld_registration/sld_reference.sld_reference.spend"
	TitleRegistrarTokenMint = "tld_registration/registrar_nft.registrar_token.mint"
)

// VerifyPolicyHash compares a configured policy id with a blueprint validator hash.
func VerifyPolicyHash(configured, blueprintHash string) error {
	if !strings.EqualFold(configured, blueprintHash) {
		return fmt.Errorf("policy id mismatch: config=%s blueprint=%s", configured, blueprintHash)
	}
	return nil
}
