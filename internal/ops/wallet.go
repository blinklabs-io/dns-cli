package ops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// WaitFundsResult describes the final observed funding state for an actor.
type WaitFundsResult struct {
	Actor    string
	Address  string
	Lovelace int64
	UTXOs    int
	Waited   time.Duration
	Attempts int
}

// WalletCreate generates a preprod wallet under outDir.
func (c *Client) WalletCreate(opts wallet.GenerateOptions) (*wallet.GeneratedWallet, error) {
	return wallet.GenerateWallet(opts)
}

// WalletFund builds an unsigned funding transaction. Returns the envelope path.
func (c *Client) WalletFund(ctx context.Context, eff *config.Effective, fromActor string, allocations []txbuilder.FundAllocation, collateralLovelace int64, out string) (string, error) {
	slog.Info("Starting wallet fund", "from", fromActor, "allocations", len(allocations), "collateral", collateralLovelace, "out", out)
	bctx, err := txbuilder.NewFundingContext(ctx, eff)
	if err != nil {
		return "", err
	}
	outBuild, err := txbuilder.FundActors(ctx, bctx, fromActor, allocations, collateralLovelace, out)
	if err != nil {
		slog.Error("wallet fund failed", "error", err)
		return "", err
	}
	slog.Info("Built wallet fund transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

// IsWalletFundValidation reports whether err is a funding validation failure.
func IsWalletFundValidation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "preprod only") ||
		strings.Contains(msg, "allocation") ||
		strings.Contains(msg, "unknown") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "collateral") ||
		strings.Contains(msg, "insufficient")
}

// WalletBalance returns total lovelace and UTxO count for an actor.
func (c *Client) WalletBalance(ctx context.Context, eff *config.Effective, actor string) (int64, int, error) {
	a, ok := eff.Profile.Actors[actor]
	if !ok {
		return 0, 0, fmt.Errorf("unknown actor %q", actor)
	}
	addr, err := common.NewAddress(a.Address)
	if err != nil {
		return 0, 0, fmt.Errorf("actor %s: invalid address: %w", actor, err)
	}
	prov, err := provider.New(eff)
	if err != nil {
		return 0, 0, err
	}
	total, count, err := chainquery.SumLovelace(ctx, prov, addr)
	if err != nil {
		return 0, 0, provider.Explain(err)
	}
	slog.Info("Queried wallet balance", "actor", actor, "lovelace", total, "utxos", count, "address", a.Address)
	return total, count, nil
}

// WalletWaitFunds polls an actor balance until it reaches minLovelace. A timeout
// of 0 means the caller wants to wait until the parent context is canceled.
func (c *Client) WalletWaitFunds(ctx context.Context, eff *config.Effective, actor string, minLovelace int64, poll, timeout time.Duration) (WaitFundsResult, error) {
	a, ok := eff.Profile.Actors[actor]
	if !ok {
		return WaitFundsResult{}, fmt.Errorf("unknown actor %q", actor)
	}
	if _, err := common.NewAddress(a.Address); err != nil {
		return WaitFundsResult{}, fmt.Errorf("actor %s: invalid address: %w", actor, err)
	}
	if minLovelace <= 0 {
		return WaitFundsResult{}, fmt.Errorf("min lovelace must be positive")
	}
	if poll <= 0 {
		return WaitFundsResult{}, fmt.Errorf("poll interval must be positive")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	start := time.Now()
	attempts := 0
	slog.Info("Waiting for wallet funds", "actor", actor, "address", a.Address, "minLovelace", minLovelace, "poll", poll)
	for {
		attempts++
		total, count, err := c.WalletBalance(ctx, eff, actor)
		if err == nil {
			if total >= minLovelace {
				return WaitFundsResult{
					Actor:    actor,
					Address:  a.Address,
					Lovelace: total,
					UTXOs:    count,
					Waited:   time.Since(start).Round(time.Second),
					Attempts: attempts,
				}, nil
			}
			slog.Info("Wallet is underfunded; waiting", "actor", actor, "lovelace", total, "minLovelace", minLovelace, "attempt", attempts)
		} else if isWaitFundsFatal(err) {
			return WaitFundsResult{}, err
		} else {
			slog.Warn("Transient wallet balance query failure; retrying", "actor", actor, "attempt", attempts, "error", err)
		}

		if attempts%3 == 1 {
			slog.Info("Fund this address to continue", "actor", actor, "address", a.Address)
		}
		select {
		case <-ctx.Done():
			return WaitFundsResult{}, ctx.Err()
		case <-time.After(poll):
		}
	}
}

func isWaitFundsFatal(err error) bool {
	if err == nil {
		return false
	}
	if provider.IsAuthFailure(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "unknown actor") ||
		strings.Contains(msg, "invalid address") ||
		strings.Contains(msg, "environment variable") ||
		strings.Contains(msg, "provider authentication failed")
}
