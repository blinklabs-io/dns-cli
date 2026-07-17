package ops

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

func TestIsWaitFundsFatal(t *testing.T) {
	if !isWaitFundsFatal(errors.New("provider authentication failed (check project id)")) {
		t.Fatal("expected auth failure to be fatal")
	}
	if !isWaitFundsFatal(errors.New("environment variable DNS_CLI_BLOCKFROST_PROJECT_ID is required")) {
		t.Fatal("expected missing env to be fatal")
	}
	if isWaitFundsFatal(errors.New("temporary network blip")) {
		t.Fatal("expected transient error")
	}
}

func TestWalletWaitFundsUnknownActor(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	_, err = c.WalletWaitFunds(t.Context(), eff, "does-not-exist", 1, time.Second, 0)
	if err == nil {
		t.Fatal("expected unknown actor error")
	}
	if !strings.Contains(err.Error(), "unknown actor") {
		t.Fatalf("got %v", err)
	}
}

func TestWalletWaitFundsRejectsBadPoll(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	eff.Profile.Actors["bootstrap"] = config.ActorConfig{
		Address: "addr_test1qz2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer3jcu5d8ps7zex2k2xt3uqxgjqnnj83ws8lhrn648jjxtwq2ytjqp",
	}
	_, err = c.WalletWaitFunds(t.Context(), eff, "bootstrap", 1, 0, 0)
	if err == nil || !strings.Contains(err.Error(), "poll interval") {
		t.Fatalf("expected poll interval error, got %v", err)
	}
	_, err = c.WalletWaitFunds(t.Context(), eff, "bootstrap", 0, time.Second, 0)
	if err == nil || !strings.Contains(err.Error(), "min lovelace") {
		t.Fatalf("expected min lovelace error, got %v", err)
	}
}

func TestTxApplyMissingPaths(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	_, err = c.TxApply(t.Context(), eff, "missing-unsigned.json", "bootstrap", "signed.json", "manifest.json", false, time.Minute, nil)
	if err == nil {
		t.Fatal("expected missing file error")
	}
}
