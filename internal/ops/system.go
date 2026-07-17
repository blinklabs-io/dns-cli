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
