package txbuilder

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/system"
)

// MintRegistrarTokenOptions configures MintRegistrarToken.
type MintRegistrarTokenOptions struct {
	Blueprint        string // base plutus.json, e.g. deployment.json's BlueprintPath
	OutDir           string // where to write the parameterized blueprint/plutus artifacts
	AikenBin         string
	FundingActor     string // defaults to "bootstrap"
	DestinationActor string // defaults to "registrar"
	OutPrefix        string
	ContractRevision string
}

// MintRegistrarToken builds an unsigned tx that mints the one-shot
// registrar NFT. registrar_token's policy is parameterized by whichever
// UTxO gets spent in the same transaction, so the funding actor's own
// UTxO doubles as both the genesis reference and a real transaction
// input — there's no separately pre-chosen "genesis UTxO" to configure.
func MintRegistrarToken(ctx context.Context, bctx *Context, opts MintRegistrarTokenOptions) (BuildOutput, *system.RegistrarTokenResult, error) {
	slog.Info("Building mint-registrar-token transaction")
	if bctx == nil || bctx.Eff == nil {
		return BuildOutput{}, nil, fmt.Errorf("nil builder context")
	}
	fundingActor := opts.FundingActor
	if fundingActor == "" {
		fundingActor = "bootstrap"
	}
	destActor := opts.DestinationActor
	if destActor == "" {
		destActor = "registrar"
	}

	funder, err := actorAddress(bctx.Eff, fundingActor)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	funderPKH, err := loadActorKeyHash(bctx.Eff, fundingActor)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	dest, err := actorAddress(bctx.Eff, destActor)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, funder, chainquery.MinActorFundingLovelace); err != nil {
		return BuildOutput{}, nil, fmt.Errorf("%s funding: %w", fundingActor, err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, funder)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	if len(funding) == 0 {
		return BuildOutput{}, nil, fmt.Errorf("no funding utxos for %s", fundingActor)
	}
	genesis := funding[0]
	genesisTxHash := genesis.Id.Id().Bytes()

	token, err := system.PrepareRegistrarToken(ctx, system.RegistrarTokenOptions{
		Blueprint: opts.Blueprint,
		TxHash:    genesisTxHash,
		TxIndex:   uint32(genesis.Id.Index()),
		OutDir:    opts.OutDir,
		AikenBin:  opts.AikenBin,
	})
	if err != nil {
		return BuildOutput{}, nil, fmt.Errorf("prepare registrar_token: %w", err)
	}
	script, err := protocol.LoadPlutusV3Script(token.PlutusFile)
	if err != nil {
		return BuildOutput{}, nil, fmt.Errorf("load registrar_token plutus: %w", err)
	}

	assetName := []byte(system.RegistrarTokenAssetName)
	tokenUnit := unit(token.PolicyID, token.AssetNameHex, 1)
	mintRed := plutusRedeemer(protocol.RegistrarTokenMintRedeemer(assetName))

	a := bctx.newApollo(funder)
	a.AddInput(genesis)
	a.AddLoadedUTxOs(funding[1:]...)
	a.AttachScript(script)
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, nil, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(funderPKH)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	a.AddRequiredSigner(pkh)
	a.Mint(tokenUnit, &mintRed, nil)
	a.PayToAddress(dest, 0, tokenUnit)

	out, err := bctx.finalize(
		a,
		"mint-registrar-token",
		"dns-cli system mint-registrar-token: mint registrar NFT",
		opts.OutPrefix,
		opts.ContractRevision,
		[]string{funderPKH},
		[]artifact.ExpectedOutput{{Role: system.RoleRegistrarToken, Index: 0}},
		map[string]string{
			"registrarTokenPolicyId":  token.PolicyID,
			"registrarTokenAssetName": token.AssetNameHex,
			"destination":             destActor,
		},
	)
	if err != nil {
		return BuildOutput{}, nil, err
	}
	return out, token, nil
}
