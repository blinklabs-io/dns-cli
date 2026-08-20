package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDocument(t *testing.T) {
	doc, err := DefaultDocument("preview", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != SchemaVersion {
		t.Fatalf("version: got %d", doc.Version)
	}
	if _, ok := doc.Profiles["preview"]; !ok {
		t.Fatal("missing preview profile")
	}
}

func TestNetworkDefaultsMainnet(t *testing.T) {
	net, err := NetworkDefaults("mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if net.Name != "mainnet" || net.ID != 1 || net.Magic != 764824073 {
		t.Fatalf("mainnet defaults: %+v", net)
	}
	if net.ExplorerTxURL != "https://cexplorer.io/tx/{txId}" {
		t.Fatalf("explorer: %s", net.ExplorerTxURL)
	}
	prov, err := ProviderDefaults("blockfrost", "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if prov.BaseURL != "https://cardano-mainnet.blockfrost.io/api/v0" {
		t.Fatalf("blockfrost mainnet url: %s", prov.BaseURL)
	}
	doc, err := DefaultDocument("mainnet", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Profiles["mainnet"].Actors["registrar"].Address; !strings.HasPrefix(got, "addr1...") {
		t.Fatalf("mainnet starter address: %s", got)
	}
	if err := ValidateOffline(&Effective{Name: "mainnet", Profile: doc.Profiles["mainnet"]}); err != nil {
		t.Fatalf("mainnet ValidateOffline: %v", err)
	}
}

func TestValidateOfflineRejectsBadProvider(t *testing.T) {
	doc, err := DefaultDocument("preview", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	p := doc.Profiles["preview"]
	p.Provider.Type = "magic"
	eff := &Effective{Name: "preview", Profile: p}
	if err := ValidateOffline(eff); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUTxORef(t *testing.T) {
	h, idx, err := ParseUTxORef("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa#3")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 3 || len(h) != 64 {
		t.Fatalf("got %s #%d", h, idx)
	}
}

func TestActorRequiresExclusiveCredential(t *testing.T) {
	doc, _ := DefaultDocument("preprod", "blockfrost")
	p := doc.Profiles["preprod"]
	a := p.Actors["registrar"]
	a.MnemonicEnv = "DNS_CLI_X"
	p.Actors["registrar"] = a
	eff := &Effective{Name: "preprod", Profile: p}
	if err := ValidateOffline(eff); err == nil {
		t.Fatal("expected both key+mnemonic rejection")
	}
}

func TestValidateOfflineBaseURLEnv(t *testing.T) {
	doc, err := DefaultDocument("preview", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	p := uniquifyActors(doc.Profiles["preview"])
	p.Provider.BaseURL = ""
	p.Provider.BaseURLEnv = "DNS_CLI_UTXORPC_URL"
	eff := &Effective{Name: "preview", Profile: p}
	if err := ValidateOffline(eff); err != nil {
		t.Fatalf("baseUrlEnv alone should pass: %v", err)
	}

	p.Provider.BaseURL = "https://example.invalid"
	eff.Profile = p
	if err := ValidateOffline(eff); err == nil {
		t.Fatal("expected rejection when both baseURL and baseUrlEnv are set")
	}

	p.Provider.BaseURL = ""
	p.Provider.BaseURLEnv = ""
	eff.Profile = p
	if err := ValidateOffline(eff); err == nil {
		t.Fatal("expected rejection when neither baseURL nor baseUrlEnv is set")
	}
}

func TestRequirePreprod(t *testing.T) {
	doc, err := DefaultDocument("preprod", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	eff := &Effective{Name: "preprod", Profile: doc.Profiles["preprod"]}
	if err := RequirePreprod(eff); err != nil {
		t.Fatal(err)
	}

	preview, err := DefaultDocument("preview", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	bad := &Effective{Name: "preview", Profile: preview.Profiles["preview"]}
	if err := RequirePreprod(bad); err == nil {
		t.Fatal("expected preview rejection")
	}

	eff.Profile.Network.Magic = 2
	if err := RequirePreprod(eff); err == nil {
		t.Fatal("expected magic mismatch rejection")
	}

	mainnet, err := DefaultDocument("mainnet", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	if err := RequirePreprod(&Effective{Name: "mainnet", Profile: mainnet.Profiles["mainnet"]}); err == nil {
		t.Fatal("expected mainnet rejection")
	}
}

func TestResolveRelativePaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "dns-cli.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	doc, err := DefaultDocument("preview", "utxorpc")
	if err != nil {
		t.Fatal(err)
	}
	p := uniquifyActors(doc.Profiles["preview"])
	p.Contracts.BlueprintPath = "../contracts/plutus.json"
	p.Transaction.ArtifactDir = "artifacts"
	reg := p.Actors["registrar"]
	reg.SigningKeyFile = "keys/registrar.skey"
	p.Actors["registrar"] = reg
	doc.Profiles["preview"] = p

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	eff, err := Load(cfgPath, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	wantBlueprint := filepath.Clean(filepath.Join(dir, "contracts", "plutus.json"))
	if eff.Profile.Contracts.BlueprintPath != wantBlueprint {
		t.Fatalf("blueprint: got %q want %q", eff.Profile.Contracts.BlueprintPath, wantBlueprint)
	}
	wantArtifact := filepath.Clean(filepath.Join(dir, "nested", "artifacts"))
	if eff.Profile.Transaction.ArtifactDir != wantArtifact {
		t.Fatalf("artifactDir: got %q want %q", eff.Profile.Transaction.ArtifactDir, wantArtifact)
	}
	wantKey := filepath.Clean(filepath.Join(dir, "nested", "keys", "registrar.skey"))
	if eff.Profile.Actors["registrar"].SigningKeyFile != wantKey {
		t.Fatalf("signingKeyFile: got %q want %q", eff.Profile.Actors["registrar"].SigningKeyFile, wantKey)
	}
}

func TestLoadAcceptsBaseURLEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "dns-cli.json")
	content := `{
  "version": 1,
  "defaultProfile": "preprod",
  "profiles": {
    "preprod": {
      "network": {
        "name": "preprod",
        "id": 0,
        "magic": 1,
        "explorerTxURL": "https://preprod.cexplorer.io/tx/{txId}"
      },
      "provider": {
        "type": "utxorpc",
        "baseUrlEnv": "DNS_CLI_UTXORPC_URL",
        "headersEnv": "DNS_CLI_UTXORPC_HEADERS"
      },
      "contracts": {
        "blueprintPath": "plutus.json",
        "referenceUtxos": {}
      },
      "actors": {
        "registrar": {
          "address": "addr_test1qrhfz4pzfeqcju3ewcktvgx42dj0kylgle229tx5nly6patln900p4grrhygewdttzjxlrudsen83k4d7vgg9c8w4yxsdvgq2y",
          "signingKeyFile": "keys/registrar.skey"
        }
      },
      "transaction": {
        "ttlSlots": 300,
        "confirmationTimeout": "10m",
        "pollInterval": "5s",
        "artifactDir": "artifacts"
      }
    }
  }
}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	eff, err := Load(cfgPath, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if eff.Profile.Provider.BaseURLEnv != "DNS_CLI_UTXORPC_URL" {
		t.Fatalf("baseUrlEnv: %q", eff.Profile.Provider.BaseURLEnv)
	}
	if strings.TrimSpace(eff.Profile.Provider.BaseURL) != "" {
		t.Fatalf("expected empty baseURL, got %q", eff.Profile.Provider.BaseURL)
	}
}

func TestRedactedViewBaseURLEnv(t *testing.T) {
	doc, _ := DefaultDocument("preview", "utxorpc")
	p := doc.Profiles["preview"]
	p.Provider.BaseURL = ""
	p.Provider.BaseURLEnv = "DNS_CLI_UTXORPC_URL"
	eff := &Effective{Name: "preview", Path: "x.json", Profile: p}
	view := RedactedView(eff, true)
	prov := view["provider"].(map[string]any)
	if prov["baseUrlEnv"] != "DNS_CLI_UTXORPC_URL" {
		t.Fatalf("missing baseUrlEnv: %#v", prov)
	}
	if prov["baseURL"] != "[redacted]" {
		t.Fatalf("expected redacted baseURL, got %#v", prov["baseURL"])
	}
}

func uniquifyActors(p Profile) Profile {
	cloned := p
	cloned.Actors = make(map[string]ActorConfig, len(p.Actors))
	for name, a := range p.Actors {
		a.Address = "addr_test1..." + name
		cloned.Actors[name] = a
	}
	return cloned
}
