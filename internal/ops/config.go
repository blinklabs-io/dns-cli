package ops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/provider"
)

// LoadConfig loads and applies overrides (same as CLI loadEffective).
func LoadConfig(path string, o config.Overrides) (*config.Effective, error) {
	if strings.TrimSpace(path) == "" {
		path = config.DefaultConfigPath
	}
	return config.Load(path, o)
}

// ValidateConfig runs offline validation, and when online is true also
// provider health and reference UTxO checks.
func (c *Client) ValidateConfig(ctx context.Context, eff *config.Effective, online bool) error {
	if err := config.ValidateOffline(eff); err != nil {
		return err
	}
	if !online {
		return nil
	}
	return c.validateOnline(ctx, eff)
}

func (c *Client) validateOnline(ctx context.Context, eff *config.Effective) error {
	if err := config.ValidateOnline(eff); err != nil {
		return err
	}
	prov, err := provider.New(eff)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	if err := prov.Health(ctx); err != nil {
		return fmt.Errorf("provider health (%s): %w", prov.Name(), err)
	}
	for name, ref := range eff.Profile.Contracts.ReferenceUtxos {
		path := fmt.Sprintf("profiles.%s.contracts.referenceUtxos.%s", eff.Name, name)
		txHash, index, err := config.ParseUTxORef(ref)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hash, err := TxIDToBlake(txHash)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		utxo, err := prov.UtxoByRef(hash, index)
		if err != nil {
			return fmt.Errorf("%s: resolve %s: %w", path, ref, err)
		}
		if utxo == nil {
			return fmt.Errorf("%s: utxo %s not found", path, ref)
		}
		if utxo.Output.ScriptRef() == nil {
			return fmt.Errorf("%s: utxo %s exists but has no reference script", path, ref)
		}
		slog.Debug("Reference UTxO verified", "key", name, "ref", ref)
	}
	slog.Info("Online config validation passed", "profile", eff.Name, "provider", prov.Name())
	return nil
}

// RedactedConfig returns a display-safe config view.
func RedactedConfig(eff *config.Effective, redact bool) map[string]any {
	return config.RedactedView(eff, redact)
}
