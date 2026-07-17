package chainquery

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// WaitFundingOpts controls WaitFundingUTxOsSynced.
type WaitFundingOpts struct {
	// ExcludeRefs are "txidhex#index" inputs that must no longer appear.
	ExcludeRefs []string
	// MinLovelace is the minimum total lovelace required (0 = any UTxO).
	MinLovelace int64
	// Poll interval (default 2s).
	Poll time.Duration
	// Timeout (default 2m). Zero uses default; negative means no deadline beyond ctx.
	Timeout time.Duration
}

// WaitFundingUTxOsSynced polls address UTxOs until spent inputs disappear and
// enough lovelace is visible. Needed after a confirmed spend because providers
// (notably Blockfrost) can briefly keep returning spent UTxOs from the address
// endpoint while tx-output confirmation already succeeded.
func WaitFundingUTxOsSynced(ctx context.Context, p provider.Provider, addr common.Address, opts WaitFundingOpts) error {
	if p == nil {
		return fmt.Errorf("nil provider")
	}
	poll := opts.Poll
	if poll <= 0 {
		poll = 2 * time.Second
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	exclude := normalizeRefs(opts.ExcludeRefs)
	slog.Info("Waiting for funding UTxOs to refresh",
		"address", addr.String(),
		"exclude", len(exclude),
		"minLovelace", opts.MinLovelace,
	)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	attempt := 0
	for {
		attempt++
		utxos, err := p.Utxos(addr)
		if err != nil {
			slog.Warn("Funding UTxO query failed; retrying", "error", err, "attempt", attempt)
		} else {
			stale, total := fundingSyncState(utxos, exclude)
			if len(stale) == 0 && total >= opts.MinLovelace && (opts.MinLovelace > 0 || len(utxos) > 0) {
				slog.Info("Funding UTxOs ready", "utxos", len(utxos), "lovelace", total, "attempts", attempt)
				return nil
			}
			slog.Debug("Funding UTxOs not ready yet",
				"attempt", attempt,
				"utxos", len(utxos),
				"lovelace", total,
				"stale", stale,
			)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for funding UTxOs at %s: %w", addr.String(), ctx.Err())
		case <-ticker.C:
		}
	}
}

func fundingSyncState(utxos []common.Utxo, exclude map[string]struct{}) (stale []string, total int64) {
	for _, u := range utxos {
		ref := fmt.Sprintf("%s#%d", hex.EncodeToString(u.Id.Id().Bytes()), u.Id.Index())
		if _, bad := exclude[ref]; bad {
			stale = append(stale, ref)
		}
		total += u.Output.Amount().Int64()
	}
	return stale, total
}

func normalizeRefs(refs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" {
			continue
		}
		out[r] = struct{}{}
	}
	return out
}
