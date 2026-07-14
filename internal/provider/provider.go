// Package provider wraps Apollo chain backends for dns-cli.
package provider

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Salvionied/apollo/v2/backend"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Provider is the application chain interface.
type Provider interface {
	backend.ChainContext
	Name() string
	Health(ctx context.Context) error
	AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32) error
}

// New constructs a provider from an effective profile.
func New(eff *config.Effective) (Provider, error) {
	p := eff.Profile.Provider
	slog.Debug("Creating chain provider", "type", p.Type, "network", eff.Profile.Network.Name)
	switch strings.ToLower(p.Type) {
	case "utxorpc":
		return newUtxoRPC(eff)
	case "blockfrost":
		return newBlockfrost(eff)
	default:
		return nil, fmt.Errorf("unsupported provider type %q", p.Type)
	}
}

type wrapped struct {
	backend.ChainContext
	name         string
	pollInterval time.Duration
}

func (w *wrapped) Name() string { return w.name }

func (w *wrapped) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := w.Tip()
	if err != nil {
		slog.Warn("Provider health check failed", "provider", w.name, "error", err)
		return err
	}
	slog.Debug("Provider healthy", "provider", w.name)
	return nil
}

func (w *wrapped) AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32) error {
	if len(indexes) == 0 {
		return fmt.Errorf("no output indexes to await")
	}
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	txHex := hex.EncodeToString(txID.Bytes())
	slog.Debug("Awaiting transaction outputs", "txId", txHex, "indexes", indexes)
	for {
		allFound := true
		for _, idx := range indexes {
			utxo, err := w.UtxoByRef(txID, idx)
			if err != nil || utxo == nil {
				allFound = false
				break
			}
		}
		if allFound {
			slog.Info("Transaction outputs confirmed", "txId", txHex)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("confirmation timeout or canceled: %w", ctx.Err())
		case <-ticker.C:
			slog.Log(ctx, logging.LevelTrace, "Polling for transaction outputs", "txId", txHex)
		}
	}
}

func loadHeaders(envName string) (map[string]string, error) {
	if envName == "" {
		return nil, nil
	}
	raw := os.Getenv(envName)
	if raw == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid header %q in %s (want Key=Value,...)", part, envName)
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out, nil
}

func requireEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return v, nil
}
