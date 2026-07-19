package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

type fakeHealth struct {
	name string
	err  error
}

func (f *fakeHealth) Name() string { return f.name }
func (f *fakeHealth) Health(context.Context) error {
	return f.err
}

func blockfrostEff(baseURL, baseURLEnv, projectEnv string) *config.Effective {
	return &config.Effective{
		Name: "preprod",
		Profile: config.Profile{
			Network: config.NetworkConfig{Name: "preprod"},
			Provider: config.ProviderConfig{
				Type:         "blockfrost",
				BaseURL:      baseURL,
				BaseURLEnv:   baseURLEnv,
				ProjectIDEnv: projectEnv,
			},
		},
	}
}

func utxoEff(baseURL, baseURLEnv, headersEnv string) *config.Effective {
	return &config.Effective{
		Name: "preprod",
		Profile: config.Profile{
			Network: config.NetworkConfig{Name: "preprod"},
			Provider: config.ProviderConfig{
				Type:       "utxorpc",
				BaseURL:    baseURL,
				BaseURLEnv: baseURLEnv,
				HeadersEnv: headersEnv,
			},
		},
	}
}

func healthyFactory(name string) func(*config.Effective) (healthChecker, error) {
	return func(*config.Effective) (healthChecker, error) {
		return &fakeHealth{name: name}, nil
	}
}

func TestCheckReadinessBlockfrostLiteral(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "preprodSECRETVALUE")
	checker := ReadinessChecker{NewProvider: healthyFactory("blockfrost")}
	r, err := checker.Check(context.Background(), blockfrostEff(
		"https://cardano-preprod.blockfrost.io/api/v0", "", "DNS_CLI_BLOCKFROST_PROJECT_ID",
	))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Healthy || r.EndpointHost != "cardano-preprod.blockfrost.io" || r.EndpointSource != "baseURL" {
		t.Fatalf("unexpected readiness: %#v", r)
	}
	assertNoSecrets(t, r, "preprodSECRETVALUE")
}

func TestCheckReadinessBlockfrostURLEnv(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_URL", "https://cardano-preprod.blockfrost.io/api/v0")
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "preprodOK")
	checker := ReadinessChecker{NewProvider: healthyFactory("blockfrost")}
	r, err := checker.Check(context.Background(), blockfrostEff(
		"", "DNS_CLI_BLOCKFROST_URL", "DNS_CLI_BLOCKFROST_PROJECT_ID",
	))
	if err != nil {
		t.Fatal(err)
	}
	if r.EndpointSource != "DNS_CLI_BLOCKFROST_URL" || r.EndpointHost != "cardano-preprod.blockfrost.io" {
		t.Fatalf("unexpected: %#v", r)
	}
}

func TestCheckReadinessBlockfrostMissingProjectID(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "")
	checker := ReadinessChecker{
		NewProvider: func(*config.Effective) (healthChecker, error) {
			t.Fatal("must not construct provider when credentials missing")
			return nil, nil
		},
	}
	r, err := checker.Check(context.Background(), blockfrostEff(
		"https://cardano-preprod.blockfrost.io/api/v0", "", "DNS_CLI_BLOCKFROST_PROJECT_ID",
	))
	if err == nil || !IsConfigError(err) {
		t.Fatalf("want config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "DNS_CLI_BLOCKFROST_PROJECT_ID") {
		t.Fatalf("error should name env: %v", err)
	}
	if r.Healthy {
		t.Fatal("Healthy should be false")
	}
}

func TestCheckReadinessUtxoRPCURLEnv(t *testing.T) {
	t.Setenv("DNS_CLI_UTXORPC_URL", "https://example.invalid:4433")
	t.Setenv("DNS_CLI_UTXORPC_HEADERS", "")
	t.Setenv("DMTR_API_KEY", "")
	checker := ReadinessChecker{NewProvider: healthyFactory("utxorpc")}
	r, err := checker.Check(context.Background(), utxoEff("", "DNS_CLI_UTXORPC_URL", ""))
	if err != nil {
		t.Fatal(err)
	}
	if r.EndpointHost != "example.invalid:4433" || r.EndpointSource != "DNS_CLI_UTXORPC_URL" {
		t.Fatalf("unexpected endpoint: %#v", r)
	}
}

func TestCheckReadinessDemeterRequiresAuth(t *testing.T) {
	t.Setenv("DMTR_API_KEY", "")
	t.Setenv("DNS_CLI_UTXORPC_HEADERS", "")
	checker := ReadinessChecker{
		NewProvider: func(*config.Effective) (healthChecker, error) {
			t.Fatal("must not construct provider")
			return nil, nil
		},
	}
	_, err := checker.Check(context.Background(), utxoEff(
		"https://preprod.utxorpc-v0.demeter.run", "", "DNS_CLI_UTXORPC_HEADERS",
	))
	if err == nil || !IsConfigError(err) {
		t.Fatalf("want config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "DMTR_API_KEY") {
		t.Fatalf("want auth env named: %v", err)
	}
}

func TestCheckReadinessDemeterWithDMTRKey(t *testing.T) {
	t.Setenv("DMTR_API_KEY", "dmtr_secret_key_value")
	t.Setenv("DNS_CLI_UTXORPC_HEADERS", "")
	checker := ReadinessChecker{NewProvider: healthyFactory("utxorpc")}
	r, err := checker.Check(context.Background(), utxoEff(
		"https://preprod.utxorpc-v0.demeter.run", "", "DNS_CLI_UTXORPC_HEADERS",
	))
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, r, "dmtr_secret_key_value")
}

func TestCheckReadinessHeadersSatisfyAuth(t *testing.T) {
	t.Setenv("DNS_CLI_UTXORPC_HEADERS", "dmtr-api-key=headerSECRET")
	t.Setenv("DMTR_API_KEY", "")
	checker := ReadinessChecker{NewProvider: healthyFactory("utxorpc")}
	r, err := checker.Check(context.Background(), utxoEff(
		"https://preprod.utxorpc-v0.demeter.run", "", "DNS_CLI_UTXORPC_HEADERS",
	))
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, r, "headerSECRET")
}

func TestCheckReadinessGenericUtxoNoAuth(t *testing.T) {
	t.Setenv("DMTR_API_KEY", "")
	checker := ReadinessChecker{NewProvider: healthyFactory("utxorpc")}
	r, err := checker.Check(context.Background(), utxoEff("https://rpc.example.invalid", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Healthy || r.EndpointHost != "rpc.example.invalid" {
		t.Fatalf("unexpected: %#v", r)
	}
}

func TestCheckReadinessInvalidURL(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "x")
	checker := ReadinessChecker{NewProvider: healthyFactory("blockfrost")}
	_, err := checker.Check(context.Background(), blockfrostEff("not a url", "", "DNS_CLI_BLOCKFROST_PROJECT_ID"))
	if err == nil || !IsConfigError(err) {
		t.Fatalf("want config error, got %v", err)
	}
}

func TestCheckReadinessHealthFailure(t *testing.T) {
	t.Setenv("DNS_CLI_BLOCKFROST_PROJECT_ID", "preprodOK")
	checker := ReadinessChecker{
		NewProvider: func(*config.Effective) (healthChecker, error) {
			return &fakeHealth{name: "blockfrost", err: errors.New("tip unavailable")}, nil
		},
	}
	r, err := checker.Check(context.Background(), blockfrostEff(
		"https://cardano-preprod.blockfrost.io/api/v0", "", "DNS_CLI_BLOCKFROST_PROJECT_ID",
	))
	if err == nil || !IsHealthError(err) {
		t.Fatalf("want health error, got %v", err)
	}
	if r.Healthy {
		t.Fatal("Healthy should be false")
	}
	if !strings.Contains(err.Error(), "blockfrost") {
		t.Fatalf("want provider type in error: %v", err)
	}
}

func TestCheckReadinessURLQueryNotLeaked(t *testing.T) {
	t.Setenv("DNS_CLI_UTXORPC_URL", "https://rpc.example.invalid/path?api_key=querySECRET")
	checker := ReadinessChecker{NewProvider: healthyFactory("utxorpc")}
	r, err := checker.Check(context.Background(), utxoEff("", "DNS_CLI_UTXORPC_URL", ""))
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, r, "querySECRET", "api_key=")
}

func assertNoSecrets(t *testing.T, r Readiness, secrets ...string) {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw) + fmt.Sprintf("%#v", r)
	for _, secret := range secrets {
		if secret != "" && strings.Contains(s, secret) {
			t.Fatalf("readiness leaked %q in %s", secret, s)
		}
	}
}
