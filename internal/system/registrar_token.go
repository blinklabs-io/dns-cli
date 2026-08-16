package system

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/protocol"
)

// RegistrarTokenOptions configures PrepareRegistrarToken.
type RegistrarTokenOptions struct {
	Blueprint string // base plutus.json (e.g. deployment.json's BlueprintPath)
	TxHash    []byte // 32-byte id of the UTxO that will be spent in the mint tx
	TxIndex   uint32
	OutDir    string
	AikenBin  string
	Runner    Runner
}

// RegistrarTokenResult summarizes the parameterized one-shot minting policy.
type RegistrarTokenResult struct {
	PolicyID      string
	AssetNameHex  string
	PlutusFile    string
	BlueprintFile string
}

// PrepareRegistrarToken parameterizes registrar_token with the exact UTxO it
// will be spent against and materializes its compiled script. registrar_token
// is a one-shot minting policy (aiken/cardano/transaction.OutputReference),
// so its policy id only exists once a specific genesis UTxO is chosen —
// unlike the other three validators, it can't be parameterized ahead of
// time by system prepare. The caller is responsible for actually spending
// that UTxO in the same transaction that mints the token; see
// txbuilder.MintRegistrarToken.
func PrepareRegistrarToken(ctx context.Context, opts RegistrarTokenOptions) (*RegistrarTokenResult, error) {
	if err := validateRegistrarTokenOpts(opts); err != nil {
		return nil, err
	}
	runner := opts.Runner
	if runner == nil {
		runner = NewCLIRunner(opts.AikenBin)
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("create out-dir: %w", err)
	}

	outputRefCBOR, err := protocol.EncodeOutputRefCBORHex(opts.TxHash, opts.TxIndex)
	if err != nil {
		return nil, fmt.Errorf("encode output reference: %w", err)
	}

	bp := filepath.Join(opts.OutDir, "plutus.registrar_token.json")
	if err := copyFileRaw(opts.Blueprint, bp); err != nil {
		return nil, err
	}
	if err := runner.Apply(ctx, opts.OutDir, filepath.Base(bp), filepath.Base(bp), ModuleRegistrarToken, ValidatorRegistrarToken, outputRefCBOR); err != nil {
		return nil, fmt.Errorf("apply registrar_token output reference: %w", err)
	}
	policyID, plutusPath, err := materializeValidator(ctx, runner, opts.OutDir, bp, ModuleRegistrarToken, ValidatorRegistrarToken, "registrar_token.plutus")
	if err != nil {
		return nil, fmt.Errorf("materialize registrar_token: %w", err)
	}

	return &RegistrarTokenResult{
		PolicyID:      policyID,
		AssetNameHex:  hex.EncodeToString([]byte(RegistrarTokenAssetName)),
		PlutusFile:    plutusPath,
		BlueprintFile: bp,
	}, nil
}

func validateRegistrarTokenOpts(opts RegistrarTokenOptions) error {
	if strings.TrimSpace(opts.Blueprint) == "" {
		return fmt.Errorf("blueprint is required")
	}
	if len(opts.TxHash) != 32 {
		return fmt.Errorf("tx hash must be 32 bytes, got %d", len(opts.TxHash))
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return fmt.Errorf("out-dir is required")
	}
	return nil
}
