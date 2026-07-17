package chainquery

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// WaitReferenceOpts controls WaitReferenceUTxOs.
type WaitReferenceOpts struct {
	Poll    time.Duration
	Timeout time.Duration
	// RequireScript when true also waits until each UTxO carries a script ref.
	RequireScript bool
}

// WaitReferenceUTxOs polls until every named reference UTxO is resolvable via
// the provider. Needed after deploy confirmation: Blockfrost tx-output APIs
// can succeed before address UTxOs (used by UtxoByRef / Apollo fee calc) catch up.
func WaitReferenceUTxOs(ctx context.Context, p provider.Provider, refs map[string]string, opts WaitReferenceOpts) error {
	if p == nil {
		return fmt.Errorf("nil provider")
	}
	if len(refs) == 0 {
		return nil
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

	parsed := make([]refTarget, 0, len(refs))
	for name, ref := range refs {
		txHash, index, err := config.ParseUTxORef(ref)
		if err != nil {
			return fmt.Errorf("reference utxo %s: %w", name, err)
		}
		hash, err := blakeFromHex(txHash)
		if err != nil {
			return fmt.Errorf("reference utxo %s: %w", name, err)
		}
		parsed = append(parsed, refTarget{Name: name, Ref: ref, Hash: hash, Index: index})
	}

	slog.Info("Waiting for reference UTxOs to resolve", "count", len(parsed), "requireScript", opts.RequireScript)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	attempt := 0
	for {
		attempt++
		missing := missingReferenceUTxOs(p, parsed, opts.RequireScript)
		if len(missing) == 0 {
			slog.Info("Reference UTxOs ready", "count", len(parsed), "attempts", attempt)
			return nil
		}
		slog.Debug("Reference UTxOs not ready yet", "attempt", attempt, "missing", missing)
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for reference UTxOs %v: %w", missing, ctx.Err())
		case <-ticker.C:
		}
	}
}

type refTarget struct {
	Name  string
	Ref   string
	Hash  common.Blake2b256
	Index uint32
}

func missingReferenceUTxOs(p provider.Provider, refs []refTarget, requireScript bool) []string {
	var missing []string
	for _, r := range refs {
		utxo, err := p.UtxoByRef(r.Hash, r.Index)
		if err != nil || utxo == nil {
			missing = append(missing, r.Name+"="+r.Ref)
			continue
		}
		if requireScript && utxo.Output.ScriptRef() == nil {
			missing = append(missing, r.Name+"="+r.Ref+"(no script)")
		}
	}
	return missing
}

func blakeFromHex(s string) (common.Blake2b256, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	b, err := hex.DecodeString(s)
	if err != nil {
		return common.Blake2b256{}, fmt.Errorf("invalid hex: %w", err)
	}
	if len(b) != 32 {
		return common.Blake2b256{}, fmt.Errorf("want 32-byte hash, got %d", len(b))
	}
	var h common.Blake2b256
	copy(h[:], b)
	return h, nil
}
