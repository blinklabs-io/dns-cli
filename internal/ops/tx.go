package ops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// TxApplyResult describes a signed, submitted, and confirmed transaction.
type TxApplyResult struct {
	TxID        string
	ExplorerURL string
	Status      string
	SignedPath  string
}

// TxInspect returns inspection data for a transaction envelope.
func (c *Client) TxInspect(txPath string) (map[string]any, error) {
	slog.Info("Inspecting transaction", "path", txPath)
	env, err := artifact.ReadEnvelope(txPath)
	if err != nil {
		return nil, err
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return nil, err
	}
	tx, err := artifact.DecodeConwayTx(raw)
	if err != nil {
		return nil, err
	}
	bodyHash, err := artifact.BodyHashHex(tx)
	if err != nil {
		return nil, err
	}
	manPath := artifact.SiblingManifestPath(txPath)
	data := map[string]any{
		"type":        env.Type,
		"description": env.Description,
		"bodyHash":    bodyHash,
		"witnesses":   artifact.CountVKeyWitnesses(tx),
	}
	if man, err := artifact.ReadManifest(manPath); err == nil {
		data["manifest"] = man
	}
	slog.Debug("Transaction inspected", "bodyHash", logging.HexPrefix(bodyHash, 4), "witnesses", data["witnesses"])
	return data, nil
}

// TxSign signs a transaction envelope for an actor.
func (c *Client) TxSign(eff *config.Effective, txPath, actor, out string, allowExtra bool) error {
	slog.Info("Signing transaction", "tx", txPath, "actor", actor, "out", out)
	a, ok := eff.Profile.Actors[actor]
	if !ok {
		return fmt.Errorf("unknown actor %q", actor)
	}
	src, err := wallet.FromActor(actor, a, eff.Profile.Network.Name)
	if err != nil {
		return err
	}
	w, err := src.LoadWallet()
	if err != nil {
		return err
	}
	manPath := artifact.SiblingManifestPath(txPath)
	if err := artifact.SignWithWallet(txPath, manPath, out, w, allowExtra); err != nil {
		slog.Error("tx sign failed", "error", err, "actor", actor)
		return err
	}
	slog.Info("Transaction signed", "actor", actor, "out", out)
	return nil
}

// TxSubmit submits a witnessed transaction. Returns tx hex id and explorer URL.
func (c *Client) TxSubmit(ctx context.Context, eff *config.Effective, txPath string) (string, string, error) {
	slog.Info("Submitting transaction", "tx", txPath, "provider", eff.Profile.Provider.Type)
	env, err := artifact.ReadEnvelope(txPath)
	if err != nil {
		return "", "", err
	}
	if env.Type != artifact.TypeWitnessedConway {
		return "", "", fmt.Errorf("transaction is not fully signed (type %q)", env.Type)
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return "", "", err
	}
	tx, err := artifact.DecodeConwayTx(raw)
	if err != nil {
		return "", "", err
	}
	if artifact.CountVKeyWitnesses(tx) == 0 {
		return "", "", fmt.Errorf("transaction has no vkey witnesses")
	}
	prov, err := provider.New(eff)
	if err != nil {
		return "", "", err
	}
	if man, err := artifact.ReadManifest(artifact.SiblingManifestPath(txPath)); err == nil {
		if !strings.EqualFold(man.Network, eff.Profile.Network.Name) {
			return "", "", fmt.Errorf("manifest network %q does not match profile %q", man.Network, eff.Profile.Network.Name)
		}
		if !strings.EqualFold(man.Provider, eff.Profile.Provider.Type) {
			return "", "", fmt.Errorf("manifest provider %q does not match profile %q", man.Provider, eff.Profile.Provider.Type)
		}
	}
	txID, err := prov.SubmitTx(raw)
	if err != nil {
		slog.Error("tx submit failed", "error", err)
		return "", "", err
	}
	txHex := txID.String()
	explorer := strings.ReplaceAll(eff.Profile.Network.ExplorerTxURL, "{txId}", txHex)
	slog.Info("Transaction submitted", "txId", txHex, "explorer", explorer)
	return txHex, explorer, nil
}

// TxStatus checks or waits for confirmation. When wait is false, returns "pending"|"confirmed"|message.
// reporter may be nil when wait is false.
func (c *Client) TxStatus(ctx context.Context, eff *config.Effective, txID, manifestPath string, wait bool, timeout time.Duration, reporter logging.WaitReporter) (string, error) {
	slog.Info("Checking transaction status", "txId", txID, "wait", wait)
	prov, err := provider.New(eff)
	if err != nil {
		return "", err
	}
	var indexes []uint32
	var ttlSlot int64
	if manifestPath != "" {
		man, err := artifact.ReadManifest(manifestPath)
		if err != nil {
			return "", err
		}
		for _, o := range man.ExpectedOutputs {
			indexes = append(indexes, o.Index)
		}
		ttlSlot = man.ValidityInterval.TTL
	}
	hash, err := TxIDToBlake(txID)
	if err != nil {
		return "", err
	}
	if !wait {
		if len(indexes) == 0 {
			return "status check requires --manifest with expected outputs or --wait", nil
		}
		for _, idx := range indexes {
			utxo, err := prov.UtxoByRef(hash, idx)
			if err != nil || utxo == nil {
				return "pending", nil
			}
		}
		return "confirmed", nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(indexes) == 0 {
		return "", fmt.Errorf("--wait requires --manifest with expected output indexes")
	}
	if ttlSlot > 0 {
		if tip, tipErr := prov.Tip(); tipErr == nil && tip >= uint64(ttlSlot) {
			return "", fmt.Errorf(
				"transaction validity already expired: tip slot %d >= ttl %d; rebuild and resubmit the unsigned tx",
				tip, ttlSlot,
			)
		}
		var cancelTTL context.CancelCauseFunc
		ctx, cancelTTL = provider.WithTTLCancel(ctx, prov.Tip, ttlSlot)
		defer cancelTTL(nil)
	}
	if reporter == nil {
		reporter = logging.NewStatusBox(logging.StatusBoxOptions{ForcePlain: true})
	}
	if cs, ok := reporter.(interface{ SetCancel(context.CancelFunc) }); ok {
		cs.SetCancel(cancel)
	}
	explorer := strings.ReplaceAll(eff.Profile.Network.ExplorerTxURL, "{txId}", txID)
	if err := prov.AwaitOutputs(ctx, hash, indexes, explorer, reporter); err != nil {
		if cause := context.Cause(ctx); cause != nil &&
			!errors.Is(cause, context.Canceled) &&
			!errors.Is(cause, context.DeadlineExceeded) {
			slog.Warn("Transaction confirmation aborted", "txId", txID, "error", cause)
			return "", cause
		}
		slog.Warn("Transaction confirmation timed out or canceled", "txId", txID, "error", err)
		return "", err
	}
	slog.Info("Transaction confirmed", "txId", txID)
	return "confirmed", nil
}

// TxApply signs, submits, and waits for confirmation of a transaction.
func (c *Client) TxApply(ctx context.Context, eff *config.Effective, unsignedPath, actor, signedPath, manifestPath string, allowExtra bool, timeout time.Duration, reporter logging.WaitReporter) (TxApplyResult, error) {
	if unsignedPath == "" {
		return TxApplyResult{}, fmt.Errorf("--tx is required")
	}
	if actor == "" {
		return TxApplyResult{}, fmt.Errorf("--actor is required")
	}
	if signedPath == "" {
		return TxApplyResult{}, fmt.Errorf("--signed is required")
	}
	if manifestPath == "" {
		return TxApplyResult{}, fmt.Errorf("--manifest is required")
	}
	if _, err := os.Stat(unsignedPath); err != nil {
		return TxApplyResult{}, err
	}
	if _, err := os.Stat(manifestPath); err != nil {
		return TxApplyResult{}, err
	}
	if err := c.TxSign(eff, unsignedPath, actor, signedPath, allowExtra); err != nil {
		return TxApplyResult{}, err
	}
	txID, explorer, err := c.TxSubmit(ctx, eff, signedPath)
	if err != nil {
		return TxApplyResult{}, err
	}
	status, err := c.TxStatus(ctx, eff, txID, manifestPath, true, timeout, reporter)
	if err != nil {
		return TxApplyResult{}, err
	}
	if status != "confirmed" {
		return TxApplyResult{}, fmt.Errorf("transaction status %q", status)
	}
	// Tx-output confirmation can race the address UTxO index used by the next build.
	if err := syncActorFundingAfterApply(ctx, eff, actor, signedPath); err != nil {
		return TxApplyResult{}, err
	}
	return TxApplyResult{
		TxID:        txID,
		ExplorerURL: explorer,
		Status:      status,
		SignedPath:  signedPath,
	}, nil
}

// syncActorFundingAfterApply waits until the signing actor's spent inputs are
// gone from the address API (Blockfrost lag after AwaitOutputs).
func syncActorFundingAfterApply(ctx context.Context, eff *config.Effective, actor, signedPath string) error {
	a, ok := eff.Profile.Actors[actor]
	if !ok || strings.TrimSpace(a.Address) == "" {
		slog.Debug("Skip funding UTxO sync; actor address missing", "actor", actor)
		return nil
	}
	addr, err := common.NewAddress(a.Address)
	if err != nil {
		return fmt.Errorf("actor %s address: %w", actor, err)
	}
	refs, err := artifact.TxInputRefs(signedPath)
	if err != nil {
		slog.Warn("Could not read spent inputs for UTxO sync; skipping", "path", signedPath, "error", err)
		return nil
	}
	if len(refs) == 0 {
		return nil
	}
	prov, err := provider.New(eff)
	if err != nil {
		return err
	}
	slog.Info("Waiting for signer funding UTxOs to refresh after confirm", "actor", actor, "exclude", len(refs))
	if err := chainquery.SyncFundingAfterSpend(ctx, prov, addr, refs); err != nil {
		return fmt.Errorf("address UTxO sync after confirm (actor %s): %w", actor, err)
	}
	return nil
}
