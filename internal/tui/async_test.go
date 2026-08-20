package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
)

type fakeRunner struct {
	balanceErr error
}

func (f fakeRunner) Load(string) (*config.Effective, error) {
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		return nil, err
	}
	p := doc.Profiles["preprod"]
	for name, a := range p.Actors {
		a.Address = "addr_test1..." + name
		a.MnemonicEnv = ""
		a.SigningKeyFile = "k/" + name
		p.Actors[name] = a
	}
	return &config.Effective{Name: "preprod", Profile: p}, nil
}
func (f fakeRunner) Validate(context.Context, *config.Effective, bool) error { return nil }
func (f fakeRunner) WalletCreate(wallet.GenerateOptions) (*wallet.GeneratedWallet, error) {
	return &wallet.GeneratedWallet{Name: "x"}, nil
}
func (f fakeRunner) WalletFund(context.Context, *config.Effective, string, []txbuilder.FundAllocation, int64, string) (string, error) {
	return "fund.json", nil
}
func (f fakeRunner) WalletBalance(context.Context, *config.Effective, string) (int64, int, error) {
	if f.balanceErr != nil {
		return 0, 0, f.balanceErr
	}
	return 42, 1, nil
}
func (f fakeRunner) ProofGenerate(string, string, string) (string, error) {
	return "proof.json", nil
}
func (f fakeRunner) SystemPrepare(context.Context, system.PrepareOptions) (string, error) {
	return "deployment.json", nil
}
func (f fakeRunner) SystemInit(context.Context, *config.Effective, string, string, string) (string, string, error) {
	return "init.json", "hash", nil
}
func (f fakeRunner) SystemBind(system.BindOptions) (string, error) { return "bound.json", nil }
func (f fakeRunner) RegisterTLD(context.Context, *config.Effective, string, string, string) (string, error) {
	return "reg.json", nil
}
func (f fakeRunner) ActivateTLD(context.Context, *config.Effective, string, string, string) (string, error) {
	return "act.json", nil
}
func (f fakeRunner) MintSLD(context.Context, *config.Effective, string, string, string, string) (string, error) {
	return "mint.json", nil
}
func (f fakeRunner) UpdateSLD(context.Context, *config.Effective, string, string, string, string) (string, error) {
	return "upd.json", nil
}
func (f fakeRunner) TxInspect(string) (map[string]any, error) {
	return map[string]any{"bodyHash": "hh"}, nil
}
func (f fakeRunner) TxSign(*config.Effective, string, string, string, bool) error { return nil }
func (f fakeRunner) TxSubmit(context.Context, *config.Effective, string) (string, string, error) {
	return "txid", "http://x", nil
}
func (f fakeRunner) TxStatus(context.Context, *config.Effective, string, string, bool, time.Duration, logging.WaitReporter) (string, error) {
	return "confirmed", nil
}

func TestOpResultMsgUpdatesState(t *testing.T) {
	m := initialModel(DashboardOpts{
		ConfigPath: "x.json",
		Version:    VersionInfo{Version: "dev"},
		Runner:     fakeRunner{},
	})
	m.busy = true
	next, _ := m.Update(OpResultMsg{
		Action:   "wallet.balance",
		Message:  "ok",
		Balances: map[string]string{"bootstrap": "42"},
		BodyHash: "abc",
	})
	mm := next.(model)
	if mm.busy {
		t.Fatal("busy should clear")
	}
	if mm.status.Balances["bootstrap"] != "42" {
		t.Fatalf("balances: %+v", mm.status.Balances)
	}
}

func TestOpErrorMsgSetsLastError(t *testing.T) {
	m := initialModel(DashboardOpts{
		ConfigPath: "x.json",
		Version:    VersionInfo{Version: "dev"},
		Runner:     fakeRunner{balanceErr: errors.New("boom")},
	})
	next, _ := m.Update(OpResultMsg{Action: "wallet.balance", Err: errors.New("boom")})
	mm := next.(model)
	if mm.status.LastError == "" {
		t.Fatal("expected last error")
	}
}

func TestChecklistPatchApplied(t *testing.T) {
	m := initialModel(DashboardOpts{
		ConfigPath: "x.json",
		Version:    VersionInfo{Version: "dev"},
		Runner:     fakeRunner{},
	})
	tr := true
	next, _ := m.Update(OpResultMsg{
		Action:    "registrar.register",
		Message:   "ok",
		TLD:       "demo",
		Checklist: &checklistPatch{Register: &tr},
	})
	mm := next.(model)
	if !mm.status.Checklist.Register {
		t.Fatal("expected register checklist")
	}
	if mm.status.TLD != "demo" {
		t.Fatalf("tld=%q", mm.status.TLD)
	}
}
