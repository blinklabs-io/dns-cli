package ops

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

func TestWalletBalanceUnknownActor(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	_, _, err = c.WalletBalance(t.Context(), eff, "does-not-exist")
	if err == nil {
		t.Fatal("expected unknown actor error")
	}
	if !strings.Contains(err.Error(), "unknown actor") {
		t.Fatalf("got %v", err)
	}
}
