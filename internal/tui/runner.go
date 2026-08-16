package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/ops"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/tui/forms"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
)

// OpResultMsg is delivered after an async ops call completes.
type OpResultMsg struct {
	Action    string
	Message   string
	Artifact  string
	TxID      string
	BodyHash  string
	Explorer  string
	TLD       string
	SLD       string
	Balances  map[string]string
	Checklist *checklistPatch
	Err       error
}

type checklistPatch struct {
	Prepare  *bool
	InitBind *bool
	Register *bool
	Activate *bool
	Mint     *bool
	Update   *bool
}

// Runner executes ops for the dashboard. Tests may substitute a fake.
type Runner interface {
	Load(path string) (*config.Effective, error)
	Validate(ctx context.Context, eff *config.Effective, online bool) error
	WalletCreate(opts wallet.GenerateOptions) (*wallet.GeneratedWallet, error)
	WalletFund(ctx context.Context, eff *config.Effective, from string, alloc []txbuilder.FundAllocation, collateral int64, out string) (string, error)
	WalletBalance(ctx context.Context, eff *config.Effective, actor string) (int64, int, error)
	ProofGenerate(tld, outDir, ownerKey string) (string, error)
	SystemPrepare(ctx context.Context, opts system.PrepareOptions) (string, error)
	SystemInit(ctx context.Context, eff *config.Effective, deployment, actor, out string) (string, string, error)
	SystemBind(opts system.BindOptions) (string, error)
	RegisterTLD(ctx context.Context, eff *config.Effective, tld, proof, out string) (string, error)
	ActivateTLD(ctx context.Context, eff *config.Effective, tld, ownerKey, out string) (string, error)
	MintSLD(ctx context.Context, eff *config.Effective, tld, sld, sldOwner, out string) (string, error)
	UpdateSLD(ctx context.Context, eff *config.Effective, tld, sld, records, out string) (string, error)
	TxInspect(path string) (map[string]any, error)
	TxSign(eff *config.Effective, txPath, actor, out string, allowExtra bool) error
	TxSubmit(ctx context.Context, eff *config.Effective, txPath string) (string, string, error)
	TxStatus(ctx context.Context, eff *config.Effective, txID, manifest string, wait bool, timeout time.Duration, reporter logging.WaitReporter) (string, error)
}

type opsRunner struct {
	client *ops.Client
}

func newOpsRunner(contractRev string) Runner {
	return &opsRunner{client: ops.New(contractRev)}
}

func (r *opsRunner) Load(path string) (*config.Effective, error) {
	return ops.LoadConfig(path, config.Overrides{})
}
func (r *opsRunner) Validate(ctx context.Context, eff *config.Effective, online bool) error {
	return r.client.ValidateConfig(ctx, eff, online)
}
func (r *opsRunner) WalletCreate(opts wallet.GenerateOptions) (*wallet.GeneratedWallet, error) {
	return r.client.WalletCreate(opts)
}
func (r *opsRunner) WalletFund(ctx context.Context, eff *config.Effective, from string, alloc []txbuilder.FundAllocation, collateral int64, out string) (string, error) {
	return r.client.WalletFund(ctx, eff, from, alloc, collateral, out)
}
func (r *opsRunner) WalletBalance(ctx context.Context, eff *config.Effective, actor string) (int64, int, error) {
	return r.client.WalletBalance(ctx, eff, actor)
}
func (r *opsRunner) ProofGenerate(tld, outDir, ownerKey string) (string, error) {
	out, err := r.client.ProofGenerate(tld, outDir, ownerKey)
	if err != nil {
		return "", err
	}
	return out.ProofBundlePath, nil
}
func (r *opsRunner) SystemPrepare(ctx context.Context, opts system.PrepareOptions) (string, error) {
	res, err := r.client.SystemPrepare(ctx, opts)
	if err != nil {
		return "", err
	}
	return res.DeploymentPath, nil
}
func (r *opsRunner) SystemInit(ctx context.Context, eff *config.Effective, deployment, actor, out string) (string, string, error) {
	res, err := r.client.SystemInit(ctx, eff, deployment, actor, out)
	if err != nil {
		return "", "", err
	}
	return res.EnvelopePath, res.BodyHash, nil
}
func (r *opsRunner) SystemBind(opts system.BindOptions) (string, error) {
	_, err := r.client.SystemBind(opts)
	if err != nil {
		return "", err
	}
	return opts.OutPath, nil
}
func (r *opsRunner) RegisterTLD(ctx context.Context, eff *config.Effective, tld, proof, out string) (string, error) {
	return r.client.RegisterTLD(ctx, eff, tld, proof, out)
}
func (r *opsRunner) ActivateTLD(ctx context.Context, eff *config.Effective, tld, ownerKey, out string) (string, error) {
	return r.client.ActivateTLD(ctx, eff, tld, ownerKey, out)
}
func (r *opsRunner) MintSLD(ctx context.Context, eff *config.Effective, tld, sld, sldOwner, out string) (string, error) {
	return r.client.MintSLD(ctx, eff, tld, sld, sldOwner, out)
}
func (r *opsRunner) UpdateSLD(ctx context.Context, eff *config.Effective, tld, sld, records, out string) (string, error) {
	return r.client.UpdateSLD(ctx, eff, tld, sld, records, out)
}
func (r *opsRunner) TxInspect(path string) (map[string]any, error) { return r.client.TxInspect(path) }
func (r *opsRunner) TxSign(eff *config.Effective, txPath, actor, out string, allowExtra bool) error {
	return r.client.TxSign(eff, txPath, actor, out, allowExtra)
}
func (r *opsRunner) TxSubmit(ctx context.Context, eff *config.Effective, txPath string) (string, string, error) {
	return r.client.TxSubmit(ctx, eff, txPath)
}
func (r *opsRunner) TxStatus(ctx context.Context, eff *config.Effective, txID, manifest string, wait bool, timeout time.Duration, reporter logging.WaitReporter) (string, error) {
	return r.client.TxStatus(ctx, eff, txID, manifest, wait, timeout, reporter)
}

func runRefreshCmd(runner Runner, path string) tea.Cmd {
	return func() tea.Msg {
		eff, err := runner.Load(path)
		if err != nil {
			return OpResultMsg{Action: "refresh", Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		onlineErr := runner.Validate(ctx, eff, true)
		msg := OpResultMsg{Action: "refresh", Message: fmt.Sprintf("config ok profile=%s", eff.Name)}
		if onlineErr != nil {
			msg.Message = fmt.Sprintf("offline ok; online: %v", onlineErr)
		}
		return msg
	}
}

func runActionCmd(runner Runner, eff *config.Effective, action string, v forms.ActionValues, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		msg := OpResultMsg{Action: action, TLD: v.TLD, SLD: v.SLD}
		switch action {
		case "wallet.create":
			outDir := v.OutDir
			if outDir == "" {
				outDir = filepath.Join("wallets", v.Name)
			}
			gen, err := runner.WalletCreate(wallet.GenerateOptions{
				Name: v.Name, Network: v.Network, Format: wallet.WalletFormat(v.Format), OutDir: outDir, Force: v.Force,
			})
			msg.Err = err
			if err == nil {
				msg.Message = fmt.Sprintf("created wallet %q at %s", gen.Name, outDir)
				msg.Artifact = outDir
			}
		case "wallet.fund":
			var allocs []txbuilder.FundAllocation
			for _, raw := range forms.ParseAllocations(v.Allocation) {
				a, err := txbuilder.ParseFundAllocation(raw)
				if err != nil {
					msg.Err = err
					return msg
				}
				allocs = append(allocs, a)
			}
			col, err := forms.ParseInt64(v.Collateral, 5_000_000)
			if err != nil {
				msg.Err = err
				return msg
			}
			path, err := runner.WalletFund(ctx, eff, v.FromActor, allocs, col, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "built fund tx " + path
			}
		case "wallet.balance":
			total, count, err := runner.WalletBalance(ctx, eff, v.Actor)
			msg.Err = err
			if err == nil {
				msg.Message = fmt.Sprintf("%s: %d lovelace (%d utxos)", v.Actor, total, count)
				msg.Balances = map[string]string{v.Actor: fmt.Sprintf("%d lovelace (%d utxos)", total, count)}
			}
		case "proof.generate":
			path, err := runner.ProofGenerate(v.TLD, v.OutDir, v.OwnerKey)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "wrote proof bundle " + path
				msg.TLD = v.TLD
			}
		case "system.prepare":
			path, err := runner.SystemPrepare(ctx, system.PrepareOptions{
				Blueprint: v.Blueprint, RegistrarTokenPolicyID: v.RegistrarTokenPolicy, StakeKeyPath: v.StakeKey,
				Network: v.Network, OutDir: v.OutDir, AikenBin: v.Aiken, Force: v.Force,
			})
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "prepared deployment " + path
				t := true
				msg.Checklist = &checklistPatch{Prepare: &t}
			}
		case "system.init":
			path, hash, err := runner.SystemInit(ctx, eff, v.Deployment, v.Actor, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.BodyHash = hash
				msg.Message = "built system init " + path
			}
		case "system.bind":
			path, err := runner.SystemBind(system.BindOptions{
				BaseConfigPath: v.BaseConfig, DeploymentPath: v.Deployment, TxID: v.TxID,
				ActorDir: v.ActorDir, Provider: v.Provider, OutPath: v.Out, Force: v.Force,
			})
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "wrote bound config " + path
				msg.TxID = strings.ToLower(strings.TrimSpace(v.TxID))
				t := true
				msg.Checklist = &checklistPatch{InitBind: &t}
			}
		case "registrar.register":
			path, err := runner.RegisterTLD(ctx, eff, v.TLD, v.Proof, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "built register-tld " + path
				msg.TLD = v.TLD
				t := true
				msg.Checklist = &checklistPatch{Register: &t}
			}
		case "owner.activate":
			path, err := runner.ActivateTLD(ctx, eff, v.TLD, v.OwnerKey, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "built activate-tld " + path
				msg.TLD = v.TLD
				t := true
				msg.Checklist = &checklistPatch{Activate: &t}
			}
		case "owner.mint":
			path, err := runner.MintSLD(ctx, eff, v.TLD, v.SLD, v.SLDOwner, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "built mint-sld " + path
				msg.TLD, msg.SLD = v.TLD, v.SLD
				t := true
				msg.Checklist = &checklistPatch{Mint: &t}
			}
		case "owner.update":
			path, err := runner.UpdateSLD(ctx, eff, v.TLD, v.SLD, v.Records, v.Out)
			msg.Err = err
			if err == nil {
				msg.Artifact = path
				msg.Message = "built update-sld " + path
				msg.TLD, msg.SLD = v.TLD, v.SLD
				t := true
				msg.Checklist = &checklistPatch{Update: &t}
			}
		case "tx.inspect":
			data, err := runner.TxInspect(v.TxPath)
			msg.Err = err
			msg.Artifact = v.TxPath
			if err == nil {
				if h, ok := data["bodyHash"].(string); ok {
					msg.BodyHash = h
				}
				msg.Message = "inspected " + v.TxPath
			}
		case "tx.sign":
			err := runner.TxSign(eff, v.TxPath, v.Actor, v.Out, v.AllowExtra)
			msg.Err = err
			if err == nil {
				msg.Artifact = v.Out
				msg.Message = "signed → " + v.Out
			}
		case "tx.submit":
			txID, explorer, err := runner.TxSubmit(ctx, eff, v.TxPath)
			msg.Err = err
			if err == nil {
				msg.TxID, msg.Explorer = txID, explorer
				msg.Message = "submitted " + txID
			}
		case "tx.status":
			reporter := NewWaitReporter(WaitOptions{ForcePlain: false, Color: true})
			status, err := runner.TxStatus(ctx, eff, v.TxID, v.Manifest, v.Wait, timeout, reporter)
			msg.Err = err
			msg.TxID = v.TxID
			if err == nil {
				msg.Message = "status: " + status
			}
		default:
			msg.Err = fmt.Errorf("unknown action %q", action)
		}
		return msg
	}
}
