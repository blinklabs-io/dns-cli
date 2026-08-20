package txbuilder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// SystemInitOptions configures the reference-script publish transaction.
type SystemInitOptions struct {
	Deployment       *system.DeploymentJSON
	DeploymentDir    string // resolves relative plutus paths
	BootstrapActor   string
	OutPrefix        string
	ContractRevision string
}

// SystemInit builds an unsigned tx publishing tldRegistrar, tldReference, sldReference
// reference scripts at output indexes 0, 1, 2.
func SystemInit(ctx context.Context, bctx *Context, opts SystemInitOptions) (BuildOutput, error) {
	slog.Info("Building system init transaction")
	if bctx == nil || bctx.Eff == nil {
		return BuildOutput{}, fmt.Errorf("nil builder context")
	}
	network := strings.ToLower(strings.TrimSpace(bctx.Eff.Profile.Network.Name))
	if _, err := config.NetworkDefaults(network); err != nil {
		return BuildOutput{}, fmt.Errorf("system init: %w", err)
	}
	dep := opts.Deployment
	if dep == nil {
		return BuildOutput{}, fmt.Errorf("deployment is required")
	}
	if err := dep.RequireRoles(); err != nil {
		return BuildOutput{}, err
	}
	actor := opts.BootstrapActor
	if actor == "" {
		actor = "bootstrap"
	}
	bootstrap, err := actorAddress(bctx.Eff, actor)
	if err != nil {
		return BuildOutput{}, err
	}
	pkhHex, err := loadActorKeyHash(bctx.Eff, actor)
	if err != nil {
		return BuildOutput{}, err
	}
	// After wallet fund, bootstrap change can lag on the address API.
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, bootstrap, chainquery.MinActorFundingLovelace); err != nil {
		return BuildOutput{}, fmt.Errorf("bootstrap funding: %w", err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, bootstrap)
	if err != nil {
		return BuildOutput{}, err
	}

	baseDir := opts.DeploymentDir
	scripts := make([]common.PlutusV3Script, 3)
	roles := []string{system.RoleTLDRegistrar, system.RoleTLDReference, system.RoleSLDReference}
	for i, role := range roles {
		v := dep.Validators[role]
		path := resolvePlutusPath(baseDir, v.PlutusFile)
		script, loadErr := protocol.LoadPlutusV3Script(path)
		if loadErr != nil {
			return BuildOutput{}, fmt.Errorf("load %s plutus: %w", role, loadErr)
		}
		scripts[i] = script
	}

	a := bctx.newApollo(bootstrap)
	a.AddLoadedUTxOs(funding...)
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(pkhHex)
	if err != nil {
		return BuildOutput{}, err
	}
	a.AddRequiredSigner(pkh)

	// Publish reference scripts to the bootstrap address (matches cardano-cli init pattern).
	for i, script := range scripts {
		var payErr error
		a, payErr = a.PayToAddressWithV3ReferenceScript(bootstrap, 0, script)
		if payErr != nil {
			return BuildOutput{}, fmt.Errorf("attach reference script %d: %w", i, payErr)
		}
	}

	extra := map[string]string{
		"tldRegistrarPolicyId": dep.Validators[system.RoleTLDRegistrar].PolicyID,
		"tldReferencePolicyId": dep.Validators[system.RoleTLDReference].PolicyID,
		"sldReferencePolicyId": dep.Validators[system.RoleSLDReference].PolicyID,
	}
	outputs := []artifact.ExpectedOutput{
		{Role: system.RoleTLDRegistrar, Index: 0},
		{Role: system.RoleTLDReference, Index: 1},
		{Role: system.RoleSLDReference, Index: 2},
	}
	return bctx.finalize(
		a,
		"system-init",
		"dns-cli system init: publish reference scripts",
		opts.OutPrefix,
		opts.ContractRevision,
		[]string{pkhHex},
		outputs,
		extra,
	)
}

func resolvePlutusPath(baseDir, plutusFile string) string {
	if plutusFile == "" {
		return plutusFile
	}
	if filepath.IsAbs(plutusFile) {
		return plutusFile
	}
	if baseDir == "" {
		return plutusFile
	}
	// Prefer basename under deployment dir (prepare writes absolute or out-dir paths).
	candidate := filepath.Join(baseDir, filepath.Base(plutusFile))
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	joined := filepath.Join(baseDir, plutusFile)
	if _, err := os.Stat(joined); err == nil {
		return joined
	}
	return plutusFile
}
