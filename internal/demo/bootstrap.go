package demo

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func (r *Runner) writeBootstrapConfig() error {
	out := r.paths.BootstrapConfig
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	bootstrapAddr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, "bootstrap"))
	if err != nil {
		return err
	}
	registrarAddr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, "registrar"))
	if err != nil {
		return err
	}
	tldOwnerAddr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, "tld-owner"))
	if err != nil {
		return err
	}
	sldOwnerAddr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, "sld-owner"))
	if err != nil {
		return err
	}

	var provider any
	switch r.provider {
	case "blockfrost":
		provider = map[string]any{
			"type":         "blockfrost",
			"baseURL":      "https://cardano-preprod.blockfrost.io/api/v0",
			"projectIdEnv": "DNS_CLI_BLOCKFROST_PROJECT_ID",
		}
	case "utxorpc":
		provider = map[string]any{
			"type":       "utxorpc",
			"baseUrlEnv": "DNS_CLI_UTXORPC_URL",
			"headersEnv": "DNS_CLI_UTXORPC_HEADERS",
		}
	default:
		return fmt.Errorf("unsupported provider %q", r.provider)
	}

	doc := map[string]any{
		"version":        1,
		"defaultProfile": "preprod",
		"profiles": map[string]any{
			"preprod": map[string]any{
				"network": map[string]any{
					"name":          "preprod",
					"id":            0,
					"magic":         1,
					"explorerTxURL": "https://preprod.cexplorer.io/tx/{txId}",
				},
				"provider": provider,
				"contracts": map[string]any{
					"blueprintPath":        "../../../fixtures/contracts/plutus.json",
					"tldRegistrarAddress":  "addr_test1...",
					"tldReferenceAddress":  "addr_test1...",
					"sldReferenceAddress":  "addr_test1...",
					"tldRegistrarPolicyId": "REPLACE_ME",
					"tldReferencePolicyId": "REPLACE_ME",
					"sldReferencePolicyId": "REPLACE_ME",
					"referenceUtxos": map[string]any{
						"tldRegistrar": "REPLACE_ME_TXHASH#0",
						"tldReference": "REPLACE_ME_TXHASH#1",
						"sldReference": "REPLACE_ME_TXHASH#2",
					},
				},
				"actors": map[string]any{
					"bootstrap": map[string]any{
						"address":        bootstrapAddr,
						"signingKeyFile": "../../shared/wallets/bootstrap/payment.skey",
					},
					"registrar": map[string]any{
						"address":        registrarAddr,
						"signingKeyFile": "../../shared/wallets/registrar/payment.skey",
					},
					"tldOwner": map[string]any{
						"address":        tldOwnerAddr,
						"signingKeyFile": "../../shared/wallets/tld-owner/payment.skey",
					},
					"sldOwner": map[string]any{
						"address":        sldOwnerAddr,
						"signingKeyFile": "../../shared/wallets/sld-owner/payment.skey",
					},
				},
				"transaction": map[string]any{
					"ttlSlots":            300,
					"confirmationTimeout": "20m",
					"pollInterval":        "5s",
					"artifactDir":         "../artifacts",
					"maxDatumBytes":       4000,
				},
			},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	_ = os.Remove(out)
	if err := os.Rename(tmp, out); err != nil {
		return err
	}
	slog.Info("Wrote bootstrap config", "path", out)
	eff, err := r.loadConfig(out)
	if err != nil {
		return fmt.Errorf("bootstrap config load: %w", err)
	}
	if err := r.ops.ValidateConfig(r.ctx, eff, false); err != nil {
		return fmt.Errorf("bootstrap config validate: %w", err)
	}
	return nil
}
