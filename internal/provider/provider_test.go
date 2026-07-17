package provider

import (
	"os"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

func TestNewRejectsUnknown(t *testing.T) {
	doc, _ := config.DefaultDocument("preview", "utxorpc")
	p := doc.Profiles["preview"]
	p.Provider.Type = "ogmios"
	_, err := New(&config.Effective{Name: "preview", Profile: p})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUtxoRPC(t *testing.T) {
	doc, _ := config.DefaultDocument("preview", "utxorpc")
	p := doc.Profiles["preview"]
	prov, err := New(&config.Effective{Name: "preview", Profile: p})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "utxorpc" {
		t.Fatalf("got %s", prov.Name())
	}
}

func TestResolveBaseURLFromLiteral(t *testing.T) {
	got, err := resolveBaseURL(config.ProviderConfig{
		BaseURL: "https://example.invalid/rpc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.invalid/rpc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveBaseURLFromEnv(t *testing.T) {
	const envName = "DNS_CLI_TEST_UTXORPC_URL"
	t.Setenv(envName, "https://from-env.example/rpc")
	got, err := resolveBaseURL(config.ProviderConfig{BaseURLEnv: envName})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://from-env.example/rpc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveBaseURLPrefersLiteral(t *testing.T) {
	const envName = "DNS_CLI_TEST_UTXORPC_URL_PREF"
	t.Setenv(envName, "https://from-env.example/rpc")
	got, err := resolveBaseURL(config.ProviderConfig{
		BaseURL:    "https://literal.example/rpc",
		BaseURLEnv: envName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://literal.example/rpc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveBaseURLMissing(t *testing.T) {
	if _, err := resolveBaseURL(config.ProviderConfig{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveBaseURLMissingEnv(t *testing.T) {
	const envName = "DNS_CLI_TEST_UTXORPC_URL_MISSING"
	_ = os.Unsetenv(envName)
	if _, err := resolveBaseURL(config.ProviderConfig{BaseURLEnv: envName}); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestNewUtxoRPCFromEnv(t *testing.T) {
	doc, _ := config.DefaultDocument("preview", "utxorpc")
	p := doc.Profiles["preview"]
	p.Provider.BaseURL = ""
	p.Provider.BaseURLEnv = "DNS_CLI_TEST_NEW_UTXORPC_URL"
	t.Setenv(p.Provider.BaseURLEnv, "https://example.invalid")
	prov, err := New(&config.Effective{Name: "preview", Profile: p})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "utxorpc" {
		t.Fatalf("got %s", prov.Name())
	}
}

func TestResolveUtxoRPCHeadersFromDMTRAPIKey(t *testing.T) {
	t.Setenv("DMTR_API_KEY", "utxorpc1testkey")
	got, err := resolveUtxoRPCHeaders("")
	if err != nil {
		t.Fatal(err)
	}
	if got["dmtr-api-key"] != "utxorpc1testkey" {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveUtxoRPCHeadersMergesDMTRAPIKey(t *testing.T) {
	const envName = "DNS_CLI_TEST_UTXORPC_HEADERS_MERGE"
	t.Setenv(envName, "X-Custom=one")
	t.Setenv("DMTR_API_KEY", "utxorpc1merge")
	got, err := resolveUtxoRPCHeaders(envName)
	if err != nil {
		t.Fatal(err)
	}
	if got["dmtr-api-key"] != "utxorpc1merge" {
		t.Fatalf("missing dmtr-api-key: %#v", got)
	}
	if got["X-Custom"] != "one" {
		t.Fatalf("missing custom header: %#v", got)
	}
}

func TestResolveUtxoRPCHeadersExplicitDmtrWins(t *testing.T) {
	const envName = "DNS_CLI_TEST_UTXORPC_HEADERS_EXPLICIT"
	t.Setenv(envName, "dmtr-api-key=from-headers")
	t.Setenv("DMTR_API_KEY", "from-dmtr-env")
	got, err := resolveUtxoRPCHeaders(envName)
	if err != nil {
		t.Fatal(err)
	}
	if got["dmtr-api-key"] != "from-headers" {
		t.Fatalf("explicit headersEnv should win: %#v", got)
	}
}

func TestResolveUtxoRPCHeadersEmpty(t *testing.T) {
	t.Setenv("DMTR_API_KEY", "")
	got, err := resolveUtxoRPCHeaders("")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil headers, got %#v", got)
	}
}

func TestNewBlockfrost(t *testing.T) {
	doc, _ := config.DefaultDocument("preprod", "blockfrost")
	p := doc.Profiles["preprod"]
	t.Setenv(p.Provider.ProjectIDEnv, "preprodFakeProjectIdValue0123456789")
	prov, err := New(&config.Effective{Name: "preprod", Profile: p})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "blockfrost" {
		t.Fatalf("got %s", prov.Name())
	}
}

func TestNewBlockfrostFromBaseURLEnv(t *testing.T) {
	doc, _ := config.DefaultDocument("preprod", "blockfrost")
	p := doc.Profiles["preprod"]
	p.Provider.BaseURL = ""
	p.Provider.BaseURLEnv = "DNS_CLI_TEST_BF_URL"
	t.Setenv(p.Provider.BaseURLEnv, "https://cardano-preprod.blockfrost.io/api/v0")
	t.Setenv(p.Provider.ProjectIDEnv, "preprodFakeProjectIdValue0123456789")
	prov, err := New(&config.Effective{Name: "preprod", Profile: p})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "blockfrost" {
		t.Fatalf("got %s", prov.Name())
	}
}
