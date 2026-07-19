package cli

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/provider"
)

func TestFormatReadinessHumanNoSecrets(t *testing.T) {
	r := provider.Readiness{
		Provider:       "blockfrost",
		Network:        "preprod",
		EndpointHost:   "cardano-preprod.blockfrost.io",
		EndpointSource: "baseURL",
		Credentials: []provider.CredentialReadiness{
			{Name: "DNS_CLI_BLOCKFROST_PROJECT_ID", Required: true, Present: true},
		},
		Healthy: true,
	}
	out := formatReadinessHuman(r, false)
	for _, want := range []string{"Provider readiness", "blockfrost", "cardano-preprod.blockfrost.io", "DNS_CLI_BLOCKFROST_PROJECT_ID", "present", "ready"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, bad := range []string{"preprodSECRET", "http://", "https://"} {
		if strings.Contains(out, bad) {
			t.Fatalf("leaked %q in:\n%s", bad, out)
		}
	}
}

func TestFormatReadinessHumanMissing(t *testing.T) {
	r := provider.Readiness{
		Provider: "utxorpc",
		Network:  "preprod",
		Credentials: []provider.CredentialReadiness{
			{Name: "DMTR_API_KEY", Required: true, Present: false},
		},
		Healthy: false,
	}
	out := formatReadinessHuman(r, false)
	if !strings.Contains(out, "missing") || !strings.Contains(out, "failed") {
		t.Fatalf("unexpected:\n%s", out)
	}
}
