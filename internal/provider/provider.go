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
	AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32, explorerURL string, reporter logging.WaitReporter) error
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

func (w *wrapped) AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32, explorerURL string, reporter logging.WaitReporter) error {
	if len(indexes) == 0 {
		return fmt.Errorf("no output indexes to await")
	}
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	txHex := hex.EncodeToString(txID.Bytes())
	slog.Debug("Awaiting transaction outputs", "txId", txHex, "indexes", indexes)

	started := time.Now()
	var deadline time.Time
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	progress := logging.WaitProgress{
		Stage:       "tx.confirm",
		Process:     "waiting for outputs",
		TxID:        txHex,
		ExplorerURL: explorerURL,
		Indexes:     indexes,
		StartedAt:   started,
		Deadline:    deadline,
	}

	poll := 0
	check := func() bool {
		for _, idx := range indexes {
			utxo, err := w.UtxoByRef(txID, idx)
			if err != nil || utxo == nil {
				return false
			}
		}
		return true
	}

	for {
		poll++
		progress.Poll = poll
		if reporter != nil {
			reporter.Tick(progress)
		}
		if check() {
			slog.Info("Transaction outputs confirmed", "txId", txHex)
			if reporter != nil {
				reporter.Done(progress, nil)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			err := fmt.Errorf("confirmation timeout or canceled: %w", context.Cause(ctx))
			if reporter != nil {
				reporter.Done(progress, err)
			}
			return err
		case <-ticker.C:
			// Avoid TRACE spam over the wait TUI; log occasionally at debug.
			if poll%12 == 0 {
				slog.Debug("Still waiting for transaction outputs", "txId", txHex, "poll", poll)
			}
		}
	}
}

const dmtrAPIKeyEnv = "DMTR_API_KEY"
const dmtrAPIKeyHeader = "dmtr-api-key"

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

// resolveUtxoRPCHeaders loads optional headersEnv values and, matching the
// utxorpc go-sdk Demeter example, applies DMTR_API_KEY as dmtr-api-key when
// that header is not already set explicitly.
func resolveUtxoRPCHeaders(headersEnv string) (map[string]string, error) {
	headers, err := loadHeaders(headersEnv)
	if err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(os.Getenv(dmtrAPIKeyEnv)); key != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		if _, exists := headers[dmtrAPIKeyHeader]; !exists {
			headers[dmtrAPIKeyHeader] = key
		}
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

func requireEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return v, nil
}
