package prereq

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// RequiredDemoRelPaths are tracked assets that must exist for a fresh demo run.
var RequiredDemoRelPaths = []string{
	"config/records.json",
	"config/blockfrost.template.json",
	"config/utxorpc.template.json",
	"fixtures/contracts/aiken.toml",
	"fixtures/contracts/validators",
	"fixtures/contracts/lib",
}

// MissingDemoAssets returns relative paths under demoRoot that are missing.
func MissingDemoAssets(demoRoot string) []string {
	var missing []string
	if demoRoot == "" {
		return []string{"(demo-root empty)"}
	}
	if st, err := os.Stat(demoRoot); err != nil || !st.IsDir() {
		return append([]string{"(demo directory)"}, RequiredDemoRelPaths...)
	}
	for _, rel := range RequiredDemoRelPaths {
		p := filepath.Join(demoRoot, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}

// EnsureDemoLayout verifies demo/ contents and offers to create/pull missing pieces.
func EnsureDemoLayout(opts Options) error {
	demoRoot := opts.DemoRoot
	if demoRoot == "" {
		return fmt.Errorf("demo root is required")
	}
	abs, err := filepath.Abs(demoRoot)
	if err != nil {
		return err
	}
	opts.DemoRoot = abs
	opts.StartDir = abs

	missing := MissingDemoAssets(abs)
	if len(missing) == 0 {
		return nil
	}

	fmt.Fprint(opts.out(), opts.theme().Panel("Demo assets incomplete", append([]report.KV{
		{Key: "root", Value: abs},
	}, missingRows(missing)...)))

	if opts.SkipInstall {
		printDemoGuide(opts)
		return fmt.Errorf("demo assets missing and --skip-install was set")
	}
	if !opts.askYes("Create/pull missing demo files and contracts now?") {
		printDemoGuide(opts)
		return fmt.Errorf("demo assets are required before fresh run")
	}

	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	for _, d := range []string{"config", "fixtures/contracts", "runs"} {
		if err := os.MkdirAll(filepath.Join(abs, filepath.FromSlash(d)), 0o755); err != nil {
			return err
		}
	}

	// Contracts first (general prereq).
	onchain, err := EnsureDNSContracts(opts)
	if err != nil {
		return err
	}
	fixtures := filepath.Join(abs, "fixtures", "contracts")
	if !ContractsOK(fixtures) {
		if err := SyncDemoContracts(onchain, fixtures); err != nil {
			return err
		}
	}

	if err := writeMissingConfigFiles(abs); err != nil {
		return err
	}

	still := MissingDemoAssets(abs)
	if len(still) > 0 {
		return fmt.Errorf("demo assets still missing after repair: %s", strings.Join(still, ", "))
	}
	fmt.Fprintln(opts.out(), opts.theme().Dim("Demo layout ready."))
	return nil
}

func writeMissingConfigFiles(demoRoot string) error {
	type file struct {
		rel  string
		body string
	}
	files := []file{
		{rel: "config/records.json", body: defaultRecordsJSON},
		{rel: "config/blockfrost.template.json", body: defaultBlockfrostTemplate},
		{rel: "config/utxorpc.template.json", body: defaultUtxorpcTemplate},
	}
	for _, f := range files {
		path := filepath.Join(demoRoot, filepath.FromSlash(f.rel))
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(f.body), 0o644); err != nil {
			return err
		}
	}
	// Keep runs/.gitkeep so the tree is recognizable.
	gitkeep := filepath.Join(demoRoot, "runs", ".gitkeep")
	if _, err := os.Stat(gitkeep); err != nil {
		_ = os.WriteFile(gitkeep, []byte{}, 0o644)
	}
	return nil
}

func printDemoGuide(opts Options) {
	fmt.Fprint(opts.out(), opts.theme().Guide(
		"Self-serve: restore the dns-cli demo tree",
		"1. Use a full dns-cli checkout that includes demo/",
		"2. Clone contracts: git clone "+DNSContractsRepoURL,
		"3. Sync onchain → demo/fixtures/contracts (aiken.toml, validators, lib, …)",
		"4. Ensure demo/config/{records.json,blockfrost.template.json,utxorpc.template.json}",
	))
}

func missingRows(missing []string) []report.KV {
	rows := make([]report.KV, 0, len(missing))
	for i, m := range missing {
		rows = append(rows, report.KV{Key: fmt.Sprintf("missing.%d", i+1), Value: m})
	}
	return rows
}

const defaultRecordsJSON = `{
  "records": [
    {
      "lhs": { "encoding": "text", "value": "www" },
      "ttl": 300,
      "class": { "encoding": "text", "value": "IN" },
      "rtype": { "encoding": "text", "value": "A" },
      "rdata": { "encoding": "hex", "value": "c0a80101" }
    }
  ]
}
`

const defaultBlockfrostTemplate = `{
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
        "type": "blockfrost",
        "baseURL": "https://cardano-preprod.blockfrost.io/api/v0",
        "projectIdEnv": "DNS_CLI_BLOCKFROST_PROJECT_ID"
      },
      "contracts": {
        "blueprintPath": "GENERATED_BY_SYSTEM_BIND",
        "tldRegistrarAddress": "GENERATED_BY_SYSTEM_BIND",
        "tldReferenceAddress": "GENERATED_BY_SYSTEM_BIND",
        "sldReferenceAddress": "GENERATED_BY_SYSTEM_BIND",
        "tldRegistrarPolicyId": "GENERATED_BY_SYSTEM_BIND",
        "tldReferencePolicyId": "GENERATED_BY_SYSTEM_BIND",
        "sldReferencePolicyId": "GENERATED_BY_SYSTEM_BIND",
        "referenceUtxos": {
          "tldRegistrar": "GENERATED_BY_SYSTEM_BIND#0",
          "tldReference": "GENERATED_BY_SYSTEM_BIND#1",
          "sldReference": "GENERATED_BY_SYSTEM_BIND#2"
        }
      },
      "actors": {
        "bootstrap": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/bootstrap/payment.skey"
        },
        "registrar": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/registrar/payment.skey"
        },
        "tldOwner": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/tld-owner/payment.skey"
        },
        "sldOwner": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/sld-owner/payment.skey"
        }
      },
      "transaction": {
        "ttlSlots": 900,
        "confirmationTimeout": "20m",
        "pollInterval": "5s",
        "artifactDir": "../artifacts",
        "maxDatumBytes": 4000
      }
    }
  }
}
`

const defaultUtxorpcTemplate = `{
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
        "blueprintPath": "GENERATED_BY_SYSTEM_BIND",
        "tldRegistrarAddress": "GENERATED_BY_SYSTEM_BIND",
        "tldReferenceAddress": "GENERATED_BY_SYSTEM_BIND",
        "sldReferenceAddress": "GENERATED_BY_SYSTEM_BIND",
        "tldRegistrarPolicyId": "GENERATED_BY_SYSTEM_BIND",
        "tldReferencePolicyId": "GENERATED_BY_SYSTEM_BIND",
        "sldReferencePolicyId": "GENERATED_BY_SYSTEM_BIND",
        "referenceUtxos": {
          "tldRegistrar": "GENERATED_BY_SYSTEM_BIND#0",
          "tldReference": "GENERATED_BY_SYSTEM_BIND#1",
          "sldReference": "GENERATED_BY_SYSTEM_BIND#2"
        }
      },
      "actors": {
        "bootstrap": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/bootstrap/payment.skey"
        },
        "registrar": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/registrar/payment.skey"
        },
        "tldOwner": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/tld-owner/payment.skey"
        },
        "sldOwner": {
          "address": "GENERATED_BY_WALLET_CREATE",
          "signingKeyFile": "../wallets/sld-owner/payment.skey"
        }
      },
      "transaction": {
        "ttlSlots": 900,
        "confirmationTimeout": "20m",
        "pollInterval": "5s",
        "artifactDir": "../artifacts",
        "maxDatumBytes": 4000
      }
    }
  }
}
`
