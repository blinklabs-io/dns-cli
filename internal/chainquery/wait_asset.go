package chainquery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// WaitByAssetOpts controls WaitByAsset.
type WaitByAssetOpts struct {
	// Poll interval (default 2s).
	Poll time.Duration
	// Timeout (default 3m). Zero uses default; negative means no deadline beyond ctx.
	Timeout time.Duration
}

// WaitByAsset polls address UTxOs until the asset is visible.
// Needed after tx confirmation: Blockfrost tx-output APIs can succeed before
// the address UTxO index used by FindByAsset catches up.
func WaitByAsset(ctx context.Context, p provider.Provider, addr common.Address, asset AssetID, opts WaitByAssetOpts) error {
	if p == nil {
		return fmt.Errorf("nil provider")
	}
	poll := opts.Poll
	if poll <= 0 {
		poll = 2 * time.Second
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 3 * time.Minute
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	slog.Info("Waiting for asset on address API",
		"address", addr.String(),
		"policy", asset.PolicyID,
		"asset", logging.HexPrefix(asset.Name, 4),
	)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	attempt := 0
	for {
		attempt++
		_, err := FindByAsset(ctx, p, addr, asset)
		if err == nil {
			slog.Info("Asset visible on address API",
				"address", addr.String(),
				"policy", asset.PolicyID,
				"attempts", attempt,
			)
			return nil
		}
		slog.Debug("Asset not visible yet",
			"attempt", attempt,
			"policy", asset.PolicyID,
			"error", err,
		)
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for asset %s.%s at %s: %w", asset.PolicyID, asset.Name, addr.String(), ctx.Err())
		case <-ticker.C:
		}
	}
}
