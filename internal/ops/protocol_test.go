package ops

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

func TestRegisterTLDInvalidLabel(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	_, err = c.RegisterTLD(t.Context(), eff, "BAD_LABEL!", "missing-proof.json", "out")
	if err == nil {
		t.Fatal("expected invalid label error")
	}
	// Must fail before network / proof file load for clearly invalid labels.
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "invalid") && !strings.Contains(msg, "label") && !strings.Contains(msg, "empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
