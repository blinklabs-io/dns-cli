package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.json"), config.Overrides{})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func uniquifyTestActors(p config.Profile) config.Profile {
	for name, a := range p.Actors {
		a.Address = "addr_test1..." + name
		a.MnemonicEnv = ""
		if a.SigningKeyFile == "" {
			a.SigningKeyFile = "keys/" + name + ".skey"
		}
		p.Actors[name] = a
	}
	return p
}

func TestLoadConfigValidFixture(t *testing.T) {
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	doc.Profiles["preprod"] = uniquifyTestActors(doc.Profiles["preprod"])

	dir := t.TempDir()
	path := filepath.Join(dir, "dns-cli.json")
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	eff, err := LoadConfig(path, config.Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if eff.Name != "preprod" {
		t.Fatalf("profile name: got %q", eff.Name)
	}
	if eff.Profile.Provider.Type != "blockfrost" {
		t.Fatalf("provider: got %q", eff.Profile.Provider.Type)
	}
}

func TestValidateConfigOffline(t *testing.T) {
	c := New("test")
	doc, err := config.DefaultDocument("preprod", "blockfrost")
	if err != nil {
		t.Fatal(err)
	}
	eff := &config.Effective{Name: "preprod", Profile: uniquifyTestActors(doc.Profiles["preprod"])}
	if err := c.ValidateConfig(t.Context(), eff, false); err != nil {
		t.Fatalf("offline validate: %v", err)
	}
}
