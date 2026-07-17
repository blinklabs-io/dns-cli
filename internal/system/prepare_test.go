package system_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// FakeRunner records Aiken CLI calls and synthesizes applied blueprints.
type FakeRunner struct {
	Calls   []string
	BuildN  int
	ApplyN  int
	hashes  map[string]string
	scripts map[string]string // validator -> compiledCode hex
}

func NewFakeRunner() *FakeRunner {
	return &FakeRunner{
		hashes:  map[string]string{},
		scripts: map[string]string{},
	}
}

func (f *FakeRunner) Version(context.Context) (string, error) {
	f.Calls = append(f.Calls, "version")
	return "aiken fake", nil
}

func (f *FakeRunner) Build(_ context.Context, workdir string) error {
	f.BuildN++
	f.Calls = append(f.Calls, "build:"+workdir)
	return nil
}

func (f *FakeRunner) Apply(_ context.Context, workdir, inBlueprint, outBlueprint, module, validator, cborHexParam string) error {
	f.ApplyN++
	f.Calls = append(f.Calls, fmt.Sprintf("apply:%s:%s:%s:%s", module, validator, inBlueprint, cborHexParam[:min(8, len(cborHexParam))]))
	inPath := inBlueprint
	if !filepath.IsAbs(inPath) {
		inPath = filepath.Join(workdir, inBlueprint)
	}
	outPath := outBlueprint
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(workdir, outBlueprint)
	}
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	var bp map[string]any
	if err := json.Unmarshal(raw, &bp); err != nil {
		return err
	}
	// Assign a deterministic fake compiled code / hash per validator once fully applied.
	key := validator
	if _, ok := f.scripts[key]; !ok {
		code := make([]byte, 40)
		copy(code, []byte{0x01, 0x01, 0x00})
		for i := 3; i < len(code); i++ {
			code[i] = byte(len(f.scripts) + i)
		}
		wrapped, _ := cbor.Encode(code)
		f.scripts[key] = hex.EncodeToString(wrapped)
		h := common.PlutusV3Script(wrapped).Hash()
		f.hashes[key] = hex.EncodeToString(h[:])
	}
	validators, _ := bp["validators"].([]any)
	for i, v := range validators {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		title, _ := vm["title"].(string)
		if strings.Contains(title, validator) {
			vm["compiledCode"] = f.scripts[key]
			vm["hash"] = f.hashes[key]
			validators[i] = vm
		}
	}
	bp["validators"] = validators
	out, err := json.MarshalIndent(bp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, out, 0o644)
}

func (f *FakeRunner) Convert(_ context.Context, workdir, blueprint, module, validator string) ([]byte, error) {
	f.Calls = append(f.Calls, "convert:"+validator)
	codeHex := f.scripts[validator]
	if codeHex == "" {
		return nil, fmt.Errorf("no script for %s", validator)
	}
	compiled, _ := hex.DecodeString(codeHex)
	env, err := protocol.WrapCompiledCodeEnvelope(compiled, "fake")
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(env, "", "  ")
}

func (f *FakeRunner) Hash(_ context.Context, _, _, _, validator string) (string, error) {
	h, ok := f.hashes[validator]
	if !ok {
		return "", fmt.Errorf("no hash for %s", validator)
	}
	return h, nil
}

func TestPrepareDeploymentWithFakeAiken(t *testing.T) {
	tmp := t.TempDir()
	blueprintDir := filepath.Join(tmp, "contracts")
	if err := os.MkdirAll(blueprintDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal plutus.json matching validator titles used by prepare.
	bp := map[string]any{
		"preamble": map[string]any{"plutusVersion": "v3", "title": "test"},
		"validators": []any{
			map[string]any{
				"title":        "tld_registration/tld_registrar.tld_registrar.spend",
				"compiledCode": "010100",
				"hash":         strings.Repeat("aa", 28),
			},
			map[string]any{
				"title":        "tld_registration/tld_reference.tld_reference.spend",
				"compiledCode": "010100",
				"hash":         strings.Repeat("bb", 28),
			},
			map[string]any{
				"title":        "tld_registration/sld_reference.sld_reference.spend",
				"compiledCode": "010100",
				"hash":         strings.Repeat("cc", 28),
			},
		},
	}
	raw, _ := json.MarshalIndent(bp, "", "  ")
	if err := os.WriteFile(filepath.Join(blueprintDir, "plutus.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, hns, err := domain.GenerateHNSKey()
	if err != nil {
		t.Fatal(err)
	}
	hnsPath := filepath.Join(tmp, "registrar.hns")
	hnsRaw, _ := json.MarshalIndent(hns, "", "  ")
	if err := os.WriteFile(hnsPath, hnsRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	stakePub := make([]byte, 32)
	for i := range stakePub {
		stakePub[i] = byte(i + 1)
	}
	stakeCBOR, err := cbor.Encode(stakePub)
	if err != nil {
		t.Fatal(err)
	}
	stakeEnv := wallet.KeyEnvelope{
		Type:        wallet.TypeStakeVKey,
		Description: "test stake vkey",
		CBORHex:     hex.EncodeToString(stakeCBOR),
	}
	stakePath := filepath.Join(tmp, "stake.vkey")
	if err := wallet.WriteKeyEnvelope(stakePath, stakeEnv); err != nil {
		t.Fatal(err)
	}

	fake := NewFakeRunner()
	outDir := filepath.Join(tmp, "out")
	result, err := system.PrepareDeployment(context.Background(), system.PrepareOptions{
		BlueprintDir:    blueprintDir,
		RegistrarHNSKey: hnsPath,
		StakeKeyPath:    stakePath,
		Network:         "preprod",
		OutDir:          outDir,
		Force:           true,
		Runner:          fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.BuildN != 1 {
		t.Fatalf("build calls %d", fake.BuildN)
	}
	if fake.ApplyN != 6 {
		t.Fatalf("apply calls %d want 6", fake.ApplyN)
	}
	if err := result.Deployment.RequireRoles(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{system.RoleTLDRegistrar, system.RoleTLDReference, system.RoleSLDReference} {
		v := result.Deployment.Validators[role]
		if v.PolicyID == "" || v.Address == "" {
			t.Fatalf("incomplete %s: %+v", role, v)
		}
		if _, err := os.Stat(v.PlutusFile); err != nil {
			t.Fatalf("missing plutus %s: %v", v.PlutusFile, err)
		}
		if !strings.HasPrefix(v.Address, "addr_test1") {
			t.Fatalf("bad address %s", v.Address)
		}
	}
	wantStake := common.Blake2b224Hash(stakePub)
	if result.Deployment.StakeKeyHash != hex.EncodeToString(wantStake[:]) {
		t.Fatalf("stake hash %s vs %s", result.Deployment.StakeKeyHash, hex.EncodeToString(wantStake[:]))
	}
}

func TestBindConfig(t *testing.T) {
	tmp := t.TempDir()
	depBlueprint := filepath.Join(tmp, "contracts", "plutus.base.json")
	dep := &system.DeploymentJSON{
		Version:       1,
		Network:       "preprod",
		NetworkID:     0,
		Magic:         1,
		StakeKeyHash:  strings.Repeat("11", 28),
		BlueprintPath: depBlueprint,
		Validators: map[string]system.ValidatorArtifact{
			system.RoleTLDRegistrar: {Role: system.RoleTLDRegistrar, PolicyID: strings.Repeat("aa", 28), Address: "addr_test1qpaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PlutusFile: "a.plutus"},
			system.RoleTLDReference: {Role: system.RoleTLDReference, PolicyID: strings.Repeat("bb", 28), Address: "addr_test1qpbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PlutusFile: "b.plutus"},
			system.RoleSLDReference: {Role: system.RoleSLDReference, PolicyID: strings.Repeat("cc", 28), Address: "addr_test1qpcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", PlutusFile: "c.plutus"},
		},
	}
	// Use real-looking test addresses from generate instead of fake bech32.
	wallets := map[string]string{}
	for _, name := range []string{"bootstrap", "registrar", "tld-owner", "sld-owner"} {
		dir := filepath.Join(tmp, "actors", name)
		generated, err := wallet.GenerateWallet(wallet.GenerateOptions{
			Name: name, Network: "preprod", Format: wallet.FormatKeyEnvelope, OutDir: dir, Force: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		wallets[name] = generated.Address
	}
	dep.Validators[system.RoleTLDRegistrar] = system.ValidatorArtifact{
		Role: system.RoleTLDRegistrar, PolicyID: strings.Repeat("a1", 28),
		Address: wallets["bootstrap"], PlutusFile: "tld_registrar.plutus",
	}
	dep.Validators[system.RoleTLDReference] = system.ValidatorArtifact{
		Role: system.RoleTLDReference, PolicyID: strings.Repeat("b2", 28),
		Address: wallets["registrar"], PlutusFile: "tld_reference.plutus",
	}
	dep.Validators[system.RoleSLDReference] = system.ValidatorArtifact{
		Role: system.RoleSLDReference, PolicyID: strings.Repeat("c3", 28),
		Address: wallets["tld-owner"], PlutusFile: "sld_reference.plutus",
	}
	depPath := filepath.Join(tmp, "deployment.json")
	if err := system.SaveDeploymentJSON(depPath, dep); err != nil {
		t.Fatal(err)
	}

	base := map[string]any{
		"version":        1,
		"defaultProfile": "preprod",
		"profiles": map[string]any{
			"preprod": map[string]any{
				"network": map[string]any{"name": "preprod", "id": 0, "magic": 1},
				"provider": map[string]any{
					"type": "blockfrost", "baseURL": "https://cardano-preprod.blockfrost.io/api/v0",
					"projectIdEnv": "DNS_CLI_BLOCKFROST_PROJECT_ID",
				},
				"contracts": map[string]any{
					"blueprintPath": "plutus.json",
					"referenceUtxos": map[string]string{
						"tldRegistrar": "GENERATED#0",
					},
				},
				"actors": map[string]any{
					"bootstrap": map[string]any{"address": "addr_test1...", "signingKeyFile": "x"},
				},
				"transaction": map[string]any{
					"ttlSlots": 300, "confirmationTimeout": "10m", "pollInterval": "5s", "artifactDir": "a",
				},
			},
		},
	}
	basePath := filepath.Join(tmp, "base.json")
	baseRaw, _ := json.MarshalIndent(base, "", "  ")
	if err := os.WriteFile(basePath, baseRaw, 0o644); err != nil {
		t.Fatal(err)
	}

	txID := strings.Repeat("ab", 32)
	outPath := filepath.Join(tmp, "bound.json")
	doc, err := system.BindConfig(system.BindOptions{
		BaseConfigPath: basePath,
		DeploymentPath: depPath,
		TxID:           txID,
		ActorDir:       filepath.Join(tmp, "actors"),
		Provider:       "blockfrost",
		OutPath:        outPath,
		Force:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	prof := doc.Profiles["preprod"]
	if prof.Contracts.TLDRegistrarPolicyID != strings.Repeat("a1", 28) {
		t.Fatalf("policy %s", prof.Contracts.TLDRegistrarPolicyID)
	}
	wantRef := txID + "#0"
	if prof.Contracts.ReferenceUtxos[system.RoleTLDRegistrar] != wantRef {
		t.Fatalf("ref %s", prof.Contracts.ReferenceUtxos[system.RoleTLDRegistrar])
	}
	if prof.Actors["bootstrap"].Address != wallets["bootstrap"] {
		t.Fatalf("bootstrap addr")
	}
	if prof.Contracts.BlueprintPath != depBlueprint {
		t.Fatalf("blueprintPath: got %q want deployment path %q", prof.Contracts.BlueprintPath, depBlueprint)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
