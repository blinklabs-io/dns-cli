package demo

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/ops"
	"github.com/blinklabs-io/dns-cli/internal/prereq"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/report"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/tui"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Runner orchestrates the Preprod demo lifecycle in Go.
type Runner struct {
	opts   Options
	ops    *ops.Client
	prompt Prompter
	paths  Paths
	ctx    context.Context

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	mode     string
	provider string
	tld      string
	sld      string
	runID    string

	tldState *TLDState
	sldState *SLDState
}

// Run executes the demo per opts. Existing mode lists local runs and resumes one.
func Run(ctx context.Context, opts Options) error {
	if opts.DemoRoot == "" {
		return fmt.Errorf("demo root is required")
	}
	paths, err := resolvePaths(opts.DemoRoot, opts.RunsRoot)
	if err != nil {
		return err
	}
	r := &Runner{
		opts:   opts,
		ops:    ops.New(opts.ContractRev),
		paths:  paths,
		ctx:    ctx,
		stdin:  mustReader(opts.Stdin),
		stdout: mustWriter(opts.Stdout),
		stderr: mustWriter(opts.Stderr),
	}
	r.prompt = NewPrompter(r.stdin, r.stdout, opts.Yes || envTruthy("DEMO_ASSUME_YES"), !opts.NoColor)

	// Validate or repair the demo source tree before every run mode.
	if err := r.ensureDemoLayout(); err != nil {
		return err
	}
	if err := migrateLegacyRuntime(paths); err != nil {
		return err
	}
	if err := LoadEnvFile(paths.EnvFile); err != nil {
		return err
	}

	if err := r.resolveSettings(); err != nil {
		return err
	}
	if r.opts.ApplyVerbose != nil && strings.TrimSpace(r.opts.LogLevel) != "" {
		if err := r.opts.ApplyVerbose(VerboseFromLogLevel(r.opts.LogLevel)); err != nil {
			return err
		}
	}

	if r.mode == "existing" {
		return r.runExisting()
	}

	// Single brand splash after options are known (mode / provider / name).
	g := r.guide()
	fmt.Fprint(r.stdout, r.theme().Splash(
		"Handshake DNS on Cardano · Preprod demo",
		r.mode,
		r.provider,
		r.sld+"."+r.tld,
	))
	fmt.Fprintln(r.stdout, "")
	g.Step("Prerequisites",
		"Check demo layout, dns-contracts fixtures, and Aiken ≥ 1.1.19.")
	if err := r.ensureFreshPrereqs(); err != nil {
		return err
	}

	g.Step("Provider credentials",
		"Confirm endpoint + API credentials (defaults and existing values are shown; blank keeps them).")
	if err := r.ensureCredentials(); err != nil {
		return err
	}

	g.Step("Layout & wallets",
		"Create/reuse runs/shared wallets and write bootstrap.json for faucet funding.")
	if err := r.initLayout(); err != nil {
		return err
	}
	if err := r.ensureWallets(); err != nil {
		return err
	}
	if err := r.writeBootstrapConfig(); err != nil {
		return err
	}
	if err := r.checkProviderReadiness(r.paths.BootstrapConfig); err != nil {
		return err
	}

	g.Step("Faucet funding",
		"Bootstrap needs ≥ 150 ADA on Preprod before fund/deploy.",
		"dns-cli wallet wait-funds --config <bootstrap.json> --actor bootstrap --min-lovelace 150000000")
	if err := r.waitForFunds(); err != nil {
		return err
	}

	g.Step("Proof",
		"Generate the owner's Handshake proof keys.",
		"dns-cli proof generate --tld <tld> --out-dir <tld>/proofs")
	if err := r.ensureProof(); err != nil {
		return err
	}
	r.showPreflight()
	if !r.prompt.ConfirmProceed("Proceed with Preprod submissions?") {
		slog.Info("Aborted before submissions")
		g.Note("Aborted before on-chain submissions (no changes since preflight).")
		return nil
	}
	if err := r.freshSubmissions(); err != nil {
		return err
	}
	r.showSuccess()
	return nil
}

func (r *Runner) prereqOptions() prereq.Options {
	return prereq.Options{
		DemoRoot:    r.paths.DemoRoot,
		StartDir:    r.paths.DemoRoot,
		SkipInstall: r.opts.SkipInstall,
		AssumeYes:   r.opts.Yes || envTruthy("DEMO_ASSUME_YES"),
		NoColor:     r.opts.NoColor,
		ConfirmYes:  r.prompt.ConfirmYes,
		Stdout:      r.stdout,
		Stderr:      r.stderr,
	}
}

func (r *Runner) ensureDemoLayout() error {
	return prereq.EnsureDemoLayout(r.prereqOptions())
}

func (r *Runner) ensureFreshPrereqs() error {
	toolsDir := filepath.Join(r.paths.SharedDir, "tools")
	_, err := prereq.EnsureAiken(r.prereqOptions(), toolsDir)
	return err
}

func (r *Runner) loadConfig(path string) (*config.Effective, error) {
	return config.Load(path, config.Overrides{Provider: r.provider})
}

func (r *Runner) waitForFunds() error {
	eff, err := r.loadConfig(r.paths.BootstrapConfig)
	if err != nil {
		return err
	}
	addr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, "bootstrap"))
	if err == nil {
		th := r.theme()
		fmt.Fprintln(r.stdout, "")
		fmt.Fprint(r.stdout, th.Panel("Faucet funding", []report.KV{
			{Key: "faucet", Value: FaucetURL},
			{Key: "address", Value: addr},
			{Key: "min ADA", Value: "150"},
		}))
		if !r.opts.NoClipboard {
			if err := prereq.CopyToClipboard(addr); err != nil {
				slog.Warn("Could not copy address to clipboard", "error", err)
			} else {
				fmt.Fprint(r.stdout, th.Dim("(copied to clipboard)"))
				fmt.Fprintln(r.stdout, "")
			}
		}
		fmt.Fprintln(r.stdout, "")
	}
	_, err = r.ops.WalletWaitFunds(r.ctx, eff, "bootstrap", MinBootstrapLovelace, PollSeconds*time.Second, 0)
	return err
}

// ensureProof generates the owner's Handshake proof bundle if missing or
// stale. Minting the registrar NFT and running system prepare both need an
// on-chain submission (the registrar NFT's policy id parameterizes
// tld_registrar), so they run later in freshSubmissions, gated behind the
// user's confirmation prompt.
func (r *Runner) ensureProof() error {
	proofsDir := filepath.Join(r.paths.TldDir, "proofs")
	if err := os.MkdirAll(proofsDir, 0o755); err != nil {
		return err
	}

	needProof := true
	if _, err := os.Stat(r.paths.ProofBundle); err == nil {
		if proofTLDMatches(r.paths.ProofBundle, r.tld) {
			needProof = false
			slog.Info("Proof bundle matches tld", "tld", r.tld)
		} else {
			slog.Info("Proof bundle tld mismatch; regenerating", "tld", r.tld)
		}
	}
	ownerKeyPath := filepath.Join(proofsDir, "owner.hns")
	if !needProof {
		if _, err := os.Stat(ownerKeyPath); err != nil {
			return fmt.Errorf("proof bundle matches %q but owner.hns is missing at %s (delete %s to regenerate): %w", r.tld, ownerKeyPath, r.paths.ProofBundle, err)
		}
		return nil
	}
	reuseKey := ownerKeyPath
	if _, err := os.Stat(reuseKey); err != nil {
		reuseKey = ""
	}
	slog.Info("Generating proof bundle", "tld", r.tld)
	if _, err := r.ops.ProofGenerate(r.tld, proofsDir, reuseKey); err != nil {
		return fmt.Errorf("proof generate: %w", err)
	}
	return nil
}

func (r *Runner) showPreflight() {
	th := r.theme()
	w := r.stdout
	fmt.Fprintln(w, "")
	fmt.Fprint(w, th.Panel(fmt.Sprintf("Preflight · %s.%s", r.sld, r.tld), []report.KV{
		{Key: "mode", Value: r.mode},
		{Key: "provider", Value: r.provider},
		{Key: "network", Value: NetworkName},
		{Key: "runId", Value: r.runID},
		{Key: "DEMO_ROOT", Value: r.paths.DemoRoot},
		{Key: "shared", Value: r.paths.SharedDir},
		{Key: "tld root", Value: r.paths.TldDir},
		{Key: "sld run", Value: r.paths.SldRunDir},
		{Key: "bootstrap cfg", Value: r.paths.BootstrapConfig},
		{Key: "deployment", Value: r.paths.DeploymentJSON},
		{Key: "proof", Value: r.paths.ProofBundle},
		{Key: "records", Value: r.paths.RecordsFile},
		{Key: "tld state", Value: r.paths.TldStateFile},
		{Key: "sld state", Value: r.paths.SldStateFile},
		{Key: "funding", Value: "registrar=30 ADA, tldOwner=50 ADA, sldOwner=30 ADA (+5 ADA collateral each)"},
	}))
	fmt.Fprintln(w, "")
	fmt.Fprint(w, th.Roadmap(
		"Submission roadmap",
		"skips steps already confirmed in state.json",
		r.preflightRoadmap(),
	))
	fmt.Fprintln(w, "")
}

func (r *Runner) preflightRoadmap() []report.RoadmapItem {
	steps := []struct {
		key    string
		label  string
		detail string
		scope  string
	}{
		{"mintRegistrarToken", "mint-registrar-token", "mint registrar NFT + parameterize validators", "tld"},
		{"fund", "fund", "split ADA to registrar / tldOwner / sldOwner", "tld"},
		{"deploy", "deploy", "publish reference scripts (system init)", "tld"},
		{"bind", "bind", "write provider config with ref UTxOs", "bind"},
		{"register", "register", "registrar register-tld", "tld"},
		{"activate", "activate", "owner activate-tld", "tld"},
		{"mintSld", "mint-sld", "owner mint-sld", "sld"},
		{"updateSld", "update-sld", "publish DNS records from records.json", "sld"},
	}
	items := make([]report.RoadmapItem, 0, len(steps))
	currentSet := false
	for _, s := range steps {
		done := false
		switch s.scope {
		case "tld":
			done = r.tldState != nil && r.tldState.stepTxID(s.key) != ""
		case "sld":
			done = r.sldState != nil && r.sldState.stepTxID(s.key) != ""
		case "bind":
			if _, err := os.Stat(r.paths.BoundConfig); err == nil {
				done = true
			}
		}
		st := report.StatusPending
		detail := s.detail
		switch {
		case done:
			st = report.StatusDone
			detail = "done"
		case !currentSet:
			st = report.StatusCurrent
			detail = "← next"
			currentSet = true
		}
		items = append(items, report.RoadmapItem{Label: s.label, Detail: detail, Status: st})
	}
	return items
}

func (r *Runner) showSuccess() {
	r.guide().Done(CompletionReport{
		TLD:         r.tld,
		SLD:         r.sld,
		RunID:       r.runID,
		Provider:    r.provider,
		RecordsPath: r.paths.RecordsFile,
		BoundConfig: r.paths.BoundConfig,
		TLDState:    r.paths.TldStateFile,
		SLDRunDir:   r.paths.SldRunDir,
		Explorer:    ExplorerURLPrefix,
		Steps: []ReportStep{
			{Label: "mint-registrar-token", TxID: r.tldState.stepTxID("mintRegistrarToken")},
			{Label: "fund", TxID: r.tldState.stepTxID("fund")},
			{Label: "deploy", TxID: r.tldState.stepTxID("deploy")},
			{Label: "register", TxID: r.tldState.stepTxID("register")},
			{Label: "activate", TxID: r.tldState.stepTxID("activate")},
			{Label: "mint-sld", TxID: r.sldState.stepTxID("mintSld")},
			{Label: "update-sld", TxID: r.sldState.stepTxID("updateSld")},
		},
	})
}

func (r *Runner) applyStep(scope, key, actor, unsignedPath, configPath string) error {
	prefix := strings.TrimSuffix(unsignedPath, ".unsigned.json")
	signed := prefix + ".signed.json"
	manifest := prefix + ".manifest.json"
	eff, err := r.loadConfig(configPath)
	if err != nil {
		return err
	}
	reporter := tui.NewWaitReporter(tui.WaitOptions{ForcePlain: false, Color: !r.opts.NoColor})
	slog.Info("Applying step", "step", key, "actor", actor)
	res, err := r.ops.TxApply(r.ctx, eff, unsignedPath, actor, signed, manifest, false, 20*time.Minute, reporter)
	if err != nil {
		return fmt.Errorf("%s apply: %w", key, err)
	}
	step := StepResult{TxID: res.TxID, Manifest: manifest}
	switch scope {
	case "tld":
		r.tldState.setStep(key, step)
		if err := writeJSONAtomic(r.paths.TldStateFile, r.tldState); err != nil {
			return err
		}
	case "sld":
		r.sldState.setStep(key, step)
		if err := writeJSONAtomic(r.paths.SldStateFile, r.sldState); err != nil {
			return err
		}
	}
	slog.Info("Step confirmed", "step", key, "txId", res.TxID)
	r.guide().Note(fmt.Sprintf("Confirmed %s — tx %s%s", key, ExplorerURLPrefix, res.TxID))
	return nil
}

func (r *Runner) freshSubmissions() error {
	if err := os.MkdirAll(r.paths.TldArtifacts, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(r.paths.SldArtifacts, 0o755); err != nil {
		return err
	}
	bootstrapCfg := r.paths.BootstrapConfig
	g := r.guide()
	boundHint := "<bound.json>"
	contractsDir := filepath.Join(r.paths.TldDir, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		return err
	}
	blueprint := filepath.Join(r.paths.DemoRoot, "fixtures", "contracts", "plutus.json")

	// mint-registrar-token + prepare
	g.Step("1/8 Mint registrar NFT + prepare validators",
		"Mint the one-shot registrar NFT (registrar authority proof), then parameterize validators against its policy id.",
		"dns-cli system mint-registrar-token --config "+quotePath(bootstrapCfg)+" --blueprint "+blueprint+" --out-dir "+quotePath(contractsDir)+" --funding-actor bootstrap --destination-actor registrar --out "+quotePath(filepath.Join(r.paths.TldArtifacts, "00-mint-registrar-token")),
		"dns-cli tx apply --config "+quotePath(bootstrapCfg)+" --tx …/00-mint-registrar-token.unsigned.json --actor bootstrap --signed …/00-mint-registrar-token.signed.json --manifest …/00-mint-registrar-token.manifest.json",
		"dns-cli system prepare --blueprint "+blueprint+" --registrar-token-policy-id <…> --stake-key <…>/stake.vkey --out-dir "+quotePath(contractsDir))
	if r.tldState.stepTxID("mintRegistrarToken") == "" {
		eff, err := r.loadConfig(bootstrapCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.TldArtifacts, "00-mint-registrar-token")
		result, err := r.ops.MintRegistrarToken(r.ctx, eff, blueprint, contractsDir, "bootstrap", "registrar", out)
		if err != nil {
			return fmt.Errorf("mint-registrar-token: %w", err)
		}
		if err := r.applyStep("tld", "mintRegistrarToken", "bootstrap", result.Build.EnvelopePath, bootstrapCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip mint-registrar-token — already confirmed in TLD state.")
	}
	dep, err := system.LoadDeploymentJSON(r.paths.DeploymentJSON)
	if err != nil {
		return fmt.Errorf("load deployment.json after mint-registrar-token: %w", err)
	}
	token, ok := dep.Validators[system.RoleRegistrarToken]
	if !ok {
		return fmt.Errorf("deployment.json missing registrarToken entry")
	}
	// A tldRegistrar entry from before this token scheme (or from a prior
	// registrar-token rotation) still satisfies a presence-only check but
	// enforces a different registrar_token policy id on-chain, so registrar
	// authority checks would fail at register-tld. Compare the policy id it
	// was actually parameterized against, not just whether it exists.
	tldRegistrar, alreadyPrepared := dep.Validators[system.RoleTLDRegistrar]
	if alreadyPrepared && tldRegistrar.RegistrarTokenPolicyID != token.PolicyID {
		slog.Warn("tld_registrar was parameterized against a different registrar_token policy; re-preparing",
			"have", tldRegistrar.RegistrarTokenPolicyID, "want", token.PolicyID)
		alreadyPrepared = false
	}
	if !alreadyPrepared {
		slog.Info("Running system prepare (parameterize)")
		stakeKey := filepath.Join(r.paths.WalletsDir, "bootstrap", "stake.vkey")
		if _, err := r.ops.SystemPrepare(r.ctx, system.PrepareOptions{
			Blueprint:              blueprint,
			RegistrarTokenPolicyID: token.PolicyID,
			StakeKeyPath:           stakeKey,
			Network:                NetworkName,
			OutDir:                 contractsDir,
		}); err != nil {
			return fmt.Errorf("system prepare: %w", err)
		}
	} else {
		g.Note("Skip system prepare — already parameterized in deployment.json.")
	}

	// fund
	if r.tldState.stepTxID("fund") == "" {
		g.Step("2/8 Fund actors",
			"Move ADA (+collateral) from bootstrap to registrar, tldOwner, and sldOwner.",
			"dns-cli wallet fund --config "+quotePath(bootstrapCfg)+" --from-actor bootstrap --allocation registrar=30000000 --allocation tldOwner=50000000 --allocation sldOwner=30000000 --collateral 5000000 --out "+quotePath(filepath.Join(r.paths.TldArtifacts, "00-fund")),
			"dns-cli tx apply --config "+quotePath(bootstrapCfg)+" --tx …/00-fund.unsigned.json --actor bootstrap --signed …/00-fund.signed.json --manifest …/00-fund.manifest.json")
		eff, err := r.loadConfig(bootstrapCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.TldArtifacts, "00-fund")
		allocations := []txbuilder.FundAllocation{
			{Actor: "registrar", Lovelace: 30000000},
			{Actor: "tldOwner", Lovelace: 50000000},
			{Actor: "sldOwner", Lovelace: 30000000},
		}
		unsigned, err := r.ops.WalletFund(r.ctx, eff, "bootstrap", allocations, 5000000, out)
		if err != nil {
			return fmt.Errorf("wallet fund: %w", err)
		}
		if err := r.applyStep("tld", "fund", "bootstrap", unsigned, bootstrapCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip fund — already confirmed in TLD state.")
	}

	// deploy
	if r.tldState.stepTxID("deploy") == "" {
		g.Step("3/8 Deploy reference scripts",
			"Publish tldRegistrar / tldReference / sldReference scripts on-chain (system init).",
			"dns-cli system init --config "+quotePath(bootstrapCfg)+" --deployment "+quotePath(r.paths.DeploymentJSON)+" --actor bootstrap --out "+quotePath(filepath.Join(r.paths.TldArtifacts, "01-deploy")),
			"dns-cli tx apply --config "+quotePath(bootstrapCfg)+" --tx …/01-deploy.unsigned.json --actor bootstrap --signed …/01-deploy.signed.json --manifest …/01-deploy.manifest.json")
		eff, err := r.loadConfig(bootstrapCfg)
		if err != nil {
			return err
		}
		// Fund confirmation (tx outputs) can race Blockfrost address UTxOs —
		// deploy previously rebuilt against the spent fund input. Wait first.
		if err := r.waitBootstrapFundingReady(eff); err != nil {
			return err
		}
		out := filepath.Join(r.paths.TldArtifacts, "01-deploy")
		build, err := r.ops.SystemInit(r.ctx, eff, r.paths.DeploymentJSON, "bootstrap", out)
		if err != nil {
			return fmt.Errorf("system init: %w", err)
		}
		if err := r.applyStep("tld", "deploy", "bootstrap", build.EnvelopePath, bootstrapCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip deploy — already confirmed in TLD state.")
	}

	// bind (always after deploy confirms)
	g.Step("4/8 Bind provider config",
		"Merge deploy tx + wallets into a runnable config (reference UTxO ids).",
		"dns-cli system bind --base-config <template.json> --deployment "+quotePath(r.paths.DeploymentJSON)+" --tx-id <deployTx> --actor-dir "+quotePath(r.paths.WalletsDir)+" --provider "+r.provider+" --out "+quotePath(r.paths.BoundConfig))
	if err := r.ensureBoundConfig(); err != nil {
		return err
	}
	boundCfg := r.paths.BoundConfig
	boundHint = quotePath(boundCfg)

	// Deploy tx-output confirmation can race Blockfrost address UTxOs used to
	// resolve reference scripts during register/activate/mint builds.
	if r.needsProtocolSteps() {
		g.Note("Waiting for reference UTxOs to appear on the provider index (can lag after deploy).")
		eff, err := r.loadConfig(boundCfg)
		if err != nil {
			return err
		}
		if err := r.waitReferenceUTxOsReady(eff); err != nil {
			return err
		}
	}

	// register
	if r.tldState.stepTxID("register") == "" {
		g.Step("5/8 Register TLD",
			fmt.Sprintf("Registrar claims %q on Preprod using the Handshake proof.", r.tld),
			"dns-cli registrar register-tld --config "+boundHint+" --tld "+r.tld+" --proof "+quotePath(r.paths.ProofBundle)+" --out "+quotePath(filepath.Join(r.paths.TldArtifacts, "02-register")),
			"dns-cli tx apply --config "+boundHint+" --tx …/02-register.unsigned.json --actor registrar --signed …/02-register.signed.json --manifest …/02-register.manifest.json")
		eff, err := r.loadConfig(boundCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.TldArtifacts, "02-register")
		unsigned, err := r.ops.RegisterTLD(r.ctx, eff, r.tld, r.paths.ProofBundle, out)
		if err != nil {
			return fmt.Errorf("register-tld: %w", err)
		}
		if err := r.applyStep("tld", "register", "registrar", unsigned, boundCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip register — already confirmed in TLD state.")
	}

	// activate
	if r.tldState.stepTxID("activate") == "" {
		g.Step("6/8 Activate TLD",
			"TLD owner activates the registration (first OwnerAction).",
			"dns-cli owner activate-tld --config "+boundHint+" --tld "+r.tld+" --owner-key "+quotePath(r.paths.OwnerHNSKey)+" --out "+quotePath(filepath.Join(r.paths.TldArtifacts, "03-activate")),
			"dns-cli tx apply --config "+boundHint+" --tx …/03-activate.unsigned.json --actor tldOwner --signed …/03-activate.signed.json --manifest …/03-activate.manifest.json")
		g.Note("Waiting for registration UTxO on the address API (Blockfrost lag).")
		eff, err := r.loadConfig(boundCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.TldArtifacts, "03-activate")
		unsigned, err := r.ops.ActivateTLD(r.ctx, eff, r.tld, r.paths.OwnerHNSKey, out)
		if err != nil {
			return fmt.Errorf("activate-tld: %w", err)
		}
		if err := r.applyStep("tld", "activate", "tldOwner", unsigned, boundCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip activate — already confirmed in TLD state.")
	}

	// mint
	if r.sldState.stepTxID("mintSld") == "" {
		g.Step("7/8 Mint SLD",
			fmt.Sprintf("Mint second-level name %q under %q; token goes to sldOwner.", r.sld, r.tld),
			"dns-cli owner mint-sld --config "+boundHint+" --tld "+r.tld+" --sld "+r.sld+" --sld-owner sldOwner --out "+quotePath(filepath.Join(r.paths.SldArtifacts, "04-mint-sld")),
			"dns-cli tx apply --config "+boundHint+" --tx …/04-mint-sld.unsigned.json --actor tldOwner --signed …/04-mint-sld.signed.json --manifest …/04-mint-sld.manifest.json")
		eff, err := r.loadConfig(boundCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.SldArtifacts, "04-mint-sld")
		unsigned, err := r.ops.MintSLD(r.ctx, eff, r.tld, r.sld, "sldOwner", out)
		if err != nil {
			return fmt.Errorf("mint-sld: %w", err)
		}
		if err := r.applyStep("sld", "mintSld", "tldOwner", unsigned, boundCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip mint-sld — already confirmed in SLD state.")
	}

	// update (DNS records — final protocol step)
	if r.sldState.stepTxID("updateSld") == "" {
		g.Step("8/8 Update DNS records (SLD)",
			"Publish the records.json set on-chain for this SLD (A/IN example by default). Edit the run’s records.json before this step to customize.",
			"dns-cli owner update-sld --config "+boundHint+" --tld "+r.tld+" --sld "+r.sld+" --records "+quotePath(r.paths.RecordsFile)+" --out "+quotePath(filepath.Join(r.paths.SldArtifacts, "05-update-sld")),
			"dns-cli tx apply --config "+boundHint+" --tx …/05-update-sld.unsigned.json --actor sldOwner --signed …/05-update-sld.signed.json --manifest …/05-update-sld.manifest.json")
		eff, err := r.loadConfig(boundCfg)
		if err != nil {
			return err
		}
		out := filepath.Join(r.paths.SldArtifacts, "05-update-sld")
		unsigned, err := r.ops.UpdateSLD(r.ctx, eff, r.tld, r.sld, r.paths.RecordsFile, out)
		if err != nil {
			return fmt.Errorf("update-sld: %w", err)
		}
		if err := r.applyStep("sld", "updateSld", "sldOwner", unsigned, boundCfg); err != nil {
			return err
		}
	} else {
		g.Note("Skip update-sld — already confirmed in SLD state.")
	}
	return nil
}

func (r *Runner) ensureBoundConfig() error {
	deployTx := r.tldState.stepTxID("deploy")
	if deployTx == "" {
		return fmt.Errorf("cannot bind: deploy not confirmed")
	}
	if _, err := os.Stat(r.paths.BoundConfig); err == nil {
		return nil
	}
	base := filepath.Join(r.paths.DemoRoot, "config", r.provider+".template.json")
	if _, err := os.Stat(base); err != nil {
		return fmt.Errorf("missing base template: %s", base)
	}
	slog.Info("Binding provider config", "out", r.paths.BoundConfig)
	_, err := r.ops.SystemBind(system.BindOptions{
		BaseConfigPath: base,
		DeploymentPath: r.paths.DeploymentJSON,
		TxID:           deployTx,
		ActorDir:       r.paths.WalletsDir,
		Provider:       r.provider,
		OutPath:        r.paths.BoundConfig,
		Force:          true,
	})
	if err != nil {
		return fmt.Errorf("system bind: %w", err)
	}
	return nil
}

// waitBootstrapFundingReady ensures Blockfrost (or other providers) no longer
// list spent fund inputs before system-init selects funding UTxOs.
func (r *Runner) waitBootstrapFundingReady(eff *config.Effective) error {
	actor, ok := eff.Profile.Actors["bootstrap"]
	if !ok || strings.TrimSpace(actor.Address) == "" {
		return fmt.Errorf("bootstrap actor address missing from config")
	}
	addr, err := common.NewAddress(actor.Address)
	if err != nil {
		return fmt.Errorf("bootstrap address: %w", err)
	}
	prov, err := provider.New(eff)
	if err != nil {
		return err
	}

	var exclude []string
	for _, name := range []string{"00-fund.signed.json", "00-fund.unsigned.json"} {
		path := filepath.Join(r.paths.TldArtifacts, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		refs, err := artifact.TxInputRefs(path)
		if err != nil {
			slog.Warn("Could not read fund inputs for UTxO sync wait", "path", path, "error", err)
			continue
		}
		exclude = refs
		break
	}
	if len(exclude) == 0 {
		slog.Debug("No fund artifact inputs found; skipping spent-input UTxO sync wait")
		return nil
	}

	const minDeployLovelace int64 = 10_000_000 // enough for ref-script deposits + fees
	return chainquery.WaitFundingUTxOsSynced(r.ctx, prov, addr, chainquery.WaitFundingOpts{
		ExcludeRefs: exclude,
		MinLovelace: minDeployLovelace,
		Poll:        2 * time.Second,
		Timeout:     3 * time.Minute,
	})
}

func (r *Runner) needsProtocolSteps() bool {
	if r.tldState.stepTxID("register") == "" || r.tldState.stepTxID("activate") == "" {
		return true
	}
	if r.sldState.stepTxID("mintSld") == "" || r.sldState.stepTxID("updateSld") == "" {
		return true
	}
	return false
}

func (r *Runner) waitReferenceUTxOsReady(eff *config.Effective) error {
	refs := eff.Profile.Contracts.ReferenceUtxos
	if len(refs) == 0 {
		return fmt.Errorf("bound config missing referenceUtxos")
	}
	prov, err := provider.New(eff)
	if err != nil {
		return err
	}
	return chainquery.WaitReferenceUTxOs(r.ctx, prov, refs, chainquery.WaitReferenceOpts{
		Poll:          2 * time.Second,
		Timeout:       3 * time.Minute,
		RequireScript: true,
	})
}

// VerboseFromLogLevel maps demo log level to a dns-cli verbosity value.
func VerboseFromLogLevel(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "quiet", "q", "0":
		return 1
	case "extensive", "debug", "verbose", "v", "ext", "4":
		return 4
	default:
		return 2
	}
}
