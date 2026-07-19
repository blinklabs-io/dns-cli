package provider

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	submit "github.com/utxorpc/go-codegen/utxorpc/v1alpha/submit"
	sdk "github.com/utxorpc/go-sdk"
)

// utxorpcProvider wraps Apollo's UTxO RPC backend and confirms txs via
// Submit.WaitForTx raced with ReadUtxos polling.
//
// Demeter (and some other UTxO RPC gateways) often emit STAGE_UNSPECIFIED once
// and then stall the WaitForTx stream without ever sending STAGE_CONFIRMED.
// Blockfrost confirmation works because it polls tx outputs; we mirror that
// here so confirmation does not depend on stage alone.
type utxorpcProvider struct {
	*wrapped
	client *sdk.UtxorpcClient
}

type waitForTxEvent struct {
	stage submit.Stage
	err   error
	done  bool
}

func (u *utxorpcProvider) outputsReady(txID common.Blake2b256, indexes []uint32) bool {
	for _, idx := range indexes {
		utxo, err := u.UtxoByRef(txID, idx)
		if err != nil || utxo == nil {
			return false
		}
	}
	return true
}

// AwaitOutputs races WaitForTx stage updates with ReadUtxos polling.
// Success = expected outputs visible, regardless of whether STAGE_CONFIRMED
// arrived (Demeter frequently never sends it).
func (u *utxorpcProvider) AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32, explorerURL string, reporter logging.WaitReporter) error {
	if len(indexes) == 0 {
		return fmt.Errorf("no output indexes to await")
	}
	txHex := hex.EncodeToString(txID.Bytes())
	slog.Debug("Awaiting transaction outputs via UTxO RPC", "txId", txHex, "indexes", indexes)

	started := time.Now()
	var deadline time.Time
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	progress := logging.WaitProgress{
		Stage:       "tx.confirm",
		Process:     "waiting for tx stage / outputs",
		TxID:        txHex,
		ExplorerURL: explorerURL,
		Indexes:     indexes,
		StartedAt:   started,
		Deadline:    deadline,
	}

	// Fast path: already visible (resume / late confirmation).
	if u.outputsReady(txID, indexes) {
		slog.Info("Transaction outputs confirmed", "txId", txHex)
		if reporter != nil {
			reporter.Done(progress, nil)
		}
		return nil
	}

	events := make(chan waitForTxEvent, 8)
	go u.watchWaitForTx(ctx, txID, events)

	interval := u.pollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	poll := 0
	lastStage := submit.Stage_STAGE_UNSPECIFIED
	for {
		poll++
		progress.Poll = poll
		if lastStage != submit.Stage_STAGE_UNSPECIFIED {
			progress.Process = "stage " + lastStage.String() + " + polling outputs"
		} else {
			progress.Process = "polling outputs (WaitForTx stage unspecified/pending)"
		}
		if reporter != nil {
			reporter.Tick(progress)
		}

		if u.outputsReady(txID, indexes) {
			slog.Info("Transaction outputs confirmed", "txId", txHex, "via", "utxo-poll")
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
		case ev := <-events:
			if ev.err != nil {
				slog.Warn("UTxO RPC WaitForTx ended; continuing with ReadUtxos poll", "txId", txHex, "error", ev.err)
				return u.pollOutputsOnly(ctx, txID, indexes, &progress, reporter)
			}
			if ev.done {
				slog.Warn("UTxO RPC WaitForTx ended without STAGE_CONFIRMED; continuing with ReadUtxos poll", "txId", txHex)
				return u.pollOutputsOnly(ctx, txID, indexes, &progress, reporter)
			}
			lastStage = ev.stage
			slog.Debug("UTxO RPC tx stage", "txId", txHex, "stage", lastStage.String())
			// STAGE_CONFIRMED is a strong hint; still verify outputs exist.
			if lastStage == submit.Stage_STAGE_CONFIRMED && u.outputsReady(txID, indexes) {
				slog.Info("Transaction outputs confirmed", "txId", txHex, "via", "wait-for-tx")
				if reporter != nil {
					reporter.Done(progress, nil)
				}
				return nil
			}
		case <-ticker.C:
			slog.Log(ctx, logging.LevelTrace, "Polling UTxO RPC for transaction outputs", "txId", txHex)
		}
	}
}

func (u *utxorpcProvider) watchWaitForTx(ctx context.Context, txID common.Blake2b256, out chan<- waitForTxEvent) {
	defer close(out)
	req := connect.NewRequest(&submit.WaitForTxRequest{
		Ref: [][]byte{append([]byte(nil), txID.Bytes()...)},
	})
	u.client.AddHeadersToRequest(req)
	stream, err := u.client.WaitForTxWithContext(ctx, req)
	if err != nil {
		select {
		case out <- waitForTxEvent{err: err}:
		case <-ctx.Done():
		}
		return
	}
	for stream.Receive() {
		msg := stream.Msg()
		stage := submit.Stage_STAGE_UNSPECIFIED
		if msg != nil {
			stage = msg.GetStage()
		}
		select {
		case out <- waitForTxEvent{stage: stage}:
		case <-ctx.Done():
			return
		}
	}
	if err := stream.Err(); err != nil {
		select {
		case out <- waitForTxEvent{err: err}:
		case <-ctx.Done():
		}
		return
	}
	select {
	case out <- waitForTxEvent{done: true}:
	case <-ctx.Done():
	}
}

func (u *utxorpcProvider) pollOutputsOnly(ctx context.Context, txID common.Blake2b256, indexes []uint32, progress *logging.WaitProgress, reporter logging.WaitReporter) error {
	progress.Process = "waiting for outputs"
	return u.waitOutputsVisible(ctx, txID, indexes, progress, reporter)
}

func (u *utxorpcProvider) waitOutputsVisible(ctx context.Context, txID common.Blake2b256, indexes []uint32, progress *logging.WaitProgress, reporter logging.WaitReporter) error {
	if u.outputsReady(txID, indexes) {
		return nil
	}
	interval := u.pollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("confirmation timeout or canceled: %w", context.Cause(ctx))
			if reporter != nil {
				reporter.Done(*progress, err)
			}
			return err
		case <-ticker.C:
			progress.Poll++
			if reporter != nil {
				reporter.Tick(*progress)
			}
			if u.outputsReady(txID, indexes) {
				return nil
			}
		}
	}
}
