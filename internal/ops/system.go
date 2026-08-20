package ops

import (
	"context"
	"path/filepath"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
)

// SystemPrepare builds parameterized validators into deployment.json.
func (c *Client) SystemPrepare(ctx context.Context, opts system.PrepareOptions) (*system.PrepareResult, error) {
	return system.PrepareDeployment(ctx, opts)
}

// MintRegistrarTokenResult carries the built tx plus the minted policy id
// needed to parameterize tld_registrar in a subsequent SystemPrepare call.
type MintRegistrarTokenResult struct {
	Build          txbuilder.BuildOutput
	DeploymentPath string
	PolicyID       string
	AssetNameHex   string
}

// MintRegistrarToken builds an unsigned tx minting the one-shot registrar
// NFT and bootstraps (or extends) deployment.json with its validator entry.
// Runs before SystemPrepare, since tld_registrar's registrar_nft_policy_id
// parameter depends on this policy id existing first.
func (c *Client) MintRegistrarToken(ctx context.Context, eff *config.Effective, blueprint, outDir, fundingActor, destActor, out string) (MintRegistrarTokenResult, error) {
	depPath := filepath.Join(outDir, "deployment.json")
	dep, err := system.LoadOrInitDeploymentJSON(depPath, blueprint, outDir)
	if err != nil {
		return MintRegistrarTokenResult{}, err
	}
	bctx, err := txbuilder.NewFundingContext(ctx, eff)
	if err != nil {
		return MintRegistrarTokenResult{}, err
	}
	build, token, err := txbuilder.MintRegistrarToken(ctx, bctx, txbuilder.MintRegistrarTokenOptions{
		Blueprint:        blueprint,
		OutDir:           outDir,
		FundingActor:     fundingActor,
		DestinationActor: destActor,
		OutPrefix:        out,
		ContractRevision: c.ContractRevision,
	})
	if err != nil {
		return MintRegistrarTokenResult{}, err
	}
	dep.Validators[system.RoleRegistrarToken] = system.ValidatorArtifact{
		Role:          system.RoleRegistrarToken,
		Module:        system.ModuleRegistrarToken,
		Validator:     system.ValidatorRegistrarToken,
		PolicyID:      token.PolicyID,
		ScriptHash:    token.PolicyID,
		Address:       eff.Profile.Actors[destActor].Address,
		PlutusFile:    token.PlutusFile,
		BlueprintFile: token.BlueprintFile,
		AssetNameHex:  token.AssetNameHex,
	}
	if err := system.SaveDeploymentJSON(depPath, dep); err != nil {
		return MintRegistrarTokenResult{}, err
	}
	return MintRegistrarTokenResult{
		Build:          build,
		DeploymentPath: depPath,
		PolicyID:       token.PolicyID,
		AssetNameHex:   token.AssetNameHex,
	}, nil
}

// SystemInit builds an unsigned system init transaction.
func (c *Client) SystemInit(ctx context.Context, eff *config.Effective, deploymentPath, actor, out string) (txbuilder.BuildOutput, error) {
	dep, err := system.LoadDeploymentJSON(deploymentPath)
	if err != nil {
		return txbuilder.BuildOutput{}, err
	}
	bctx, err := txbuilder.NewFundingContext(ctx, eff)
	if err != nil {
		return txbuilder.BuildOutput{}, err
	}
	return txbuilder.SystemInit(ctx, bctx, txbuilder.SystemInitOptions{
		Deployment:       dep,
		DeploymentDir:    filepath.Dir(deploymentPath),
		BootstrapActor:   actor,
		OutPrefix:        out,
		ContractRevision: c.ContractRevision,
	})
}

// SystemBind merges deployment + init tx into a runnable config document.
func (c *Client) SystemBind(opts system.BindOptions) (*config.Document, error) {
	return system.BindConfig(opts)
}
