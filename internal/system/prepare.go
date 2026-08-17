package system

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// PrepareOptions configures PrepareDeployment.
type PrepareOptions struct {
	Blueprint              string
	RegistrarTokenPolicyID string
	StakeKeyPath           string
	Network                string
	OutDir                 string
	AikenBin               string
	Force                  bool
	Runner                 Runner
}

// PrepareResult summarizes prepare outputs.
type PrepareResult struct {
	DeploymentPath string
	Deployment     *DeploymentJSON
}

// PrepareDeployment parameterizes and packages system validators from a
// pre-built blueprint.
func PrepareDeployment(ctx context.Context, opts PrepareOptions) (*PrepareResult, error) {
	if err := validatePrepareOpts(opts); err != nil {
		return nil, err
	}
	runner := opts.Runner
	if runner == nil {
		runner = NewCLIRunner(opts.AikenBin)
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("create out-dir: %w", err)
	}
	depPath := filepath.Join(opts.OutDir, "deployment.json")
	existing, err := LoadOrInitDeploymentJSON(depPath, opts.Blueprint, opts.OutDir)
	if err != nil {
		return nil, err
	}
	if _, alreadyPrepared := existing.Validators[RoleTLDRegistrar]; alreadyPrepared && !opts.Force {
		return nil, fmt.Errorf("deployment already exists at %s (use --force to overwrite)", depPath)
	}

	registrarTokenPolicy, err := protocol.HexPolicyID(opts.RegistrarTokenPolicyID)
	if err != nil {
		return nil, fmt.Errorf("registrar token policy id: %w", err)
	}
	stakeHash, err := LoadStakeKeyHash(opts.StakeKeyPath)
	if err != nil {
		return nil, err
	}
	registrarTokenPolicyCBOR, err := protocol.EncodePolicyIDCBORHex(registrarTokenPolicy)
	if err != nil {
		return nil, fmt.Errorf("encode registrar token policy id: %w", err)
	}
	stakeCBOR, err := protocol.EncodeStakeKeyHashCredentialCBORHex(stakeHash)
	if err != nil {
		return nil, fmt.Errorf("encode stake credential: %w", err)
	}

	srcBlueprint := opts.Blueprint
	if _, err := os.Stat(srcBlueprint); err != nil {
		return nil, fmt.Errorf("blueprint not found: %w", err)
	}

	baseBP, err := copyFile(srcBlueprint, filepath.Join(opts.OutDir, "plutus.base.json"))
	if err != nil {
		return nil, err
	}

	// 1) Apply registrar_nft_policy_id + stake_cred to tld_registrar
	regBP := filepath.Join(opts.OutDir, "plutus.tld_registrar.json")
	if err := copyFileRaw(baseBP, regBP); err != nil {
		return nil, err
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(regBP), filepath.Base(regBP), ModuleTLDRegistrar, ValidatorTLDRegistrar, registrarTokenPolicyCBOR); err != nil {
		return nil, fmt.Errorf("apply registrar nft policy id: %w", err)
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(regBP), filepath.Base(regBP), ModuleTLDRegistrar, ValidatorTLDRegistrar, stakeCBOR); err != nil {
		return nil, fmt.Errorf("apply registrar stake cred: %w", err)
	}
	regPolicy, regPlutus, err := materializeValidator(ctx, runner, opts.OutDir, regBP, ModuleTLDRegistrar, ValidatorTLDRegistrar, "tld_registrar.plutus")
	if err != nil {
		return nil, fmt.Errorf("materialize tld_registrar: %w", err)
	}

	// 2) Apply tld_registrar_policy_id + stake_cred to tld_reference
	regPolicyBytes, err := protocol.HexPolicyID(regPolicy)
	if err != nil {
		return nil, err
	}
	regPolicyCBOR, err := protocol.EncodePolicyIDCBORHex(regPolicyBytes)
	if err != nil {
		return nil, err
	}
	tldBP := filepath.Join(opts.OutDir, "plutus.tld_reference.json")
	if err := copyFileRaw(baseBP, tldBP); err != nil {
		return nil, err
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(tldBP), filepath.Base(tldBP), ModuleTLDReference, ValidatorTLDReference, regPolicyCBOR); err != nil {
		return nil, fmt.Errorf("apply tld_reference policy: %w", err)
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(tldBP), filepath.Base(tldBP), ModuleTLDReference, ValidatorTLDReference, stakeCBOR); err != nil {
		return nil, fmt.Errorf("apply tld_reference stake: %w", err)
	}
	tldPolicy, tldPlutus, err := materializeValidator(ctx, runner, opts.OutDir, tldBP, ModuleTLDReference, ValidatorTLDReference, "tld_reference.plutus")
	if err != nil {
		return nil, fmt.Errorf("materialize tld_reference: %w", err)
	}

	// 3) Apply tld_reference_policy_id + stake_cred to sld_reference
	tldPolicyBytes, err := protocol.HexPolicyID(tldPolicy)
	if err != nil {
		return nil, err
	}
	tldPolicyCBOR, err := protocol.EncodePolicyIDCBORHex(tldPolicyBytes)
	if err != nil {
		return nil, err
	}
	sldBP := filepath.Join(opts.OutDir, "plutus.sld_reference.json")
	if err := copyFileRaw(baseBP, sldBP); err != nil {
		return nil, err
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(sldBP), filepath.Base(sldBP), ModuleSLDReference, ValidatorSLDReference, tldPolicyCBOR); err != nil {
		return nil, fmt.Errorf("apply sld_reference policy: %w", err)
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(sldBP), filepath.Base(sldBP), ModuleSLDReference, ValidatorSLDReference, stakeCBOR); err != nil {
		return nil, fmt.Errorf("apply sld_reference stake: %w", err)
	}
	sldPolicy, sldPlutus, err := materializeValidator(ctx, runner, opts.OutDir, sldBP, ModuleSLDReference, ValidatorSLDReference, "sld_reference.plutus")
	if err != nil {
		return nil, fmt.Errorf("materialize sld_reference: %w", err)
	}

	networkID := protocol.PreprodNetworkID
	addrs := map[string]string{}
	for role, policy := range map[string]string{
		RoleTLDRegistrar: regPolicy,
		RoleTLDReference: tldPolicy,
		RoleSLDReference: sldPolicy,
	} {
		addr, err := protocol.ScriptBaseAddressHex(networkID, policy, hex.EncodeToString(stakeHash))
		if err != nil {
			return nil, fmt.Errorf("address for %s: %w", role, err)
		}
		addrs[role] = addr.String()
		addrPath := filepath.Join(opts.OutDir, role+".addr")
		if err := os.WriteFile(addrPath, []byte(addr.String()+"\n"), 0o644); err != nil {
			return nil, err
		}
	}

	dep := existing
	dep.Version = 1
	dep.Network = "preprod"
	dep.NetworkID = networkID
	dep.Magic = 1
	dep.StakeKeyHash = hex.EncodeToString(stakeHash)
	dep.BlueprintPath = baseBP
	dep.OutDir = opts.OutDir
	if dep.Validators == nil {
		dep.Validators = map[string]ValidatorArtifact{}
	}
	dep.Validators[RoleTLDRegistrar] = ValidatorArtifact{
		Role:          RoleTLDRegistrar,
		Module:        ModuleTLDRegistrar,
		Validator:     ValidatorTLDRegistrar,
		PolicyID:      regPolicy,
		ScriptHash:    regPolicy,
		Address:       addrs[RoleTLDRegistrar],
		PlutusFile:    regPlutus,
		BlueprintFile: regBP,
		// Canonical lowercase hex, not opts.RegistrarTokenPolicyID verbatim,
		// so callers comparing against materializeValidator's own
		// (lowercase) policy ids don't get a spurious mismatch from casing.
		RegistrarTokenPolicyID: hex.EncodeToString(registrarTokenPolicy),
	}
	dep.Validators[RoleTLDReference] = ValidatorArtifact{
		Role:          RoleTLDReference,
		Module:        ModuleTLDReference,
		Validator:     ValidatorTLDReference,
		PolicyID:      tldPolicy,
		ScriptHash:    tldPolicy,
		Address:       addrs[RoleTLDReference],
		PlutusFile:    tldPlutus,
		BlueprintFile: tldBP,
	}
	dep.Validators[RoleSLDReference] = ValidatorArtifact{
		Role:          RoleSLDReference,
		Module:        ModuleSLDReference,
		Validator:     ValidatorSLDReference,
		PolicyID:      sldPolicy,
		ScriptHash:    sldPolicy,
		Address:       addrs[RoleSLDReference],
		PlutusFile:    sldPlutus,
		BlueprintFile: sldBP,
	}
	if err := SaveDeploymentJSON(depPath, dep); err != nil {
		return nil, err
	}
	return &PrepareResult{DeploymentPath: depPath, Deployment: dep}, nil
}

func materializeValidator(ctx context.Context, runner Runner, outDir, blueprint, module, validator, plutusName string) (policyID, plutusPath string, err error) {
	plutusPath = filepath.Join(outDir, plutusName)
	bpPath := blueprint
	if !filepath.IsAbs(bpPath) {
		bpPath = filepath.Join(outDir, blueprint)
	}
	bp, err := protocol.LoadBlueprint(bpPath)
	if err != nil {
		return "", "", fmt.Errorf("load applied blueprint: %w", err)
	}
	titleSpend := fmt.Sprintf("%s.%s.spend", module, validator)
	v, findErr := bp.FindValidator(titleSpend)
	if findErr != nil {
		v, findErr = findValidatorByName(bp, validator)
		if findErr != nil {
			return "", "", findErr
		}
	}
	script, err := protocol.ScriptFromCompiledCodeHex(v.CompiledCode)
	if err != nil {
		return "", "", err
	}
	policyID = protocol.ScriptHashHex(script)
	if v.Hash != "" && len(v.Hash) == 56 {
		policyID = strings.ToLower(v.Hash)
	}

	// Prefer aiken convert output when available (cardano-cli text envelope).
	if converted, convErr := runner.Convert(ctx, outDir, blueprint, module, validator); convErr == nil && len(converted) > 0 {
		if writeErr := os.WriteFile(plutusPath, converted, 0o644); writeErr != nil {
			return "", "", writeErr
		}
	} else {
		env, wrapErr := protocol.WrapCompiledCodeEnvelope([]byte(script), "Generated by dns-cli / Aiken")
		if wrapErr != nil {
			return "", "", wrapErr
		}
		if saveErr := protocol.SavePlutusEnvelope(plutusPath, env); saveErr != nil {
			return "", "", saveErr
		}
	}
	return policyID, plutusPath, nil
}

func findValidatorByName(bp *protocol.Blueprint, name string) (*protocol.BlueprintValidator, error) {
	suffix := "." + name + ".spend"
	for i := range bp.Validators {
		if strings.HasSuffix(bp.Validators[i].Title, suffix) {
			return &bp.Validators[i], nil
		}
	}
	for i := range bp.Validators {
		if strings.Contains(bp.Validators[i].Title, name) {
			return &bp.Validators[i], nil
		}
	}
	return nil, fmt.Errorf("validator %q not found", name)
}

func validatePrepareOpts(opts PrepareOptions) error {
	if strings.TrimSpace(opts.Blueprint) == "" {
		return fmt.Errorf("--blueprint is required")
	}
	if strings.TrimSpace(opts.RegistrarTokenPolicyID) == "" {
		return fmt.Errorf("--registrar-token-policy-id is required")
	}
	if strings.TrimSpace(opts.StakeKeyPath) == "" {
		return fmt.Errorf("--stake-key is required")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	net := strings.ToLower(strings.TrimSpace(opts.Network))
	if net == "" {
		net = "preprod"
	}
	if net != "preprod" {
		return fmt.Errorf("system prepare supports preprod only (got %q)", opts.Network)
	}
	return nil
}

// LoadStakeKeyHash loads blake2b-224 of a Cardano stake verification key.
// Path may be a stake.vkey envelope or a wallet directory containing stake.vkey.
func LoadStakeKeyHash(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	vkeyPath := path
	if info.IsDir() {
		vkeyPath = filepath.Join(path, "stake.vkey")
	}
	env, err := wallet.ReadKeyEnvelope(vkeyPath)
	if err != nil {
		return nil, fmt.Errorf("read stake key: %w", err)
	}
	cborBytes, err := wallet.DecodeKeyEnvelopeCBOR(env)
	if err != nil {
		return nil, err
	}
	keyBytes, err := wallet.ExtractKeyBytes(cborBytes)
	if err != nil {
		return nil, fmt.Errorf("stake key bytes: %w", err)
	}
	// Payment/stake vkeys are 32-byte ed25519 public keys (or 64-byte extended).
	pub := keyBytes
	switch len(keyBytes) {
	case 32:
		pub = keyBytes
	case 64:
		pub = keyBytes[32:]
	default:
		return nil, fmt.Errorf("unexpected stake verification key length %d", len(keyBytes))
	}
	hash := common.Blake2b224Hash(pub)
	return hash[:], nil
}

func copyFile(src, dst string) (string, error) {
	if err := copyFileRaw(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func copyFileRaw(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
