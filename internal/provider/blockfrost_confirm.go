package provider

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// blockfrostProvider wraps Apollo's Blockfrost backend and confirms txs via
// GET /txs/{hash}/utxos without calling Apollo UtxoByRef.
//
// Apollo's Blockfrost UtxoByRef hydrates outputs with toUtxo(), which requires
// tx_hash on each output. Blockfrost omits tx_hash on /txs/{hash}/utxos
// outputs (hash is only top-level), so Apollo always errors and AwaitOutputs
// never completes. Address UTxOs still work and are used for UtxoByRef.
type blockfrostProvider struct {
	*wrapped
	baseURL    string
	projectID  string
	httpClient *http.Client

	paramsMu            sync.Mutex
	cachedCostModelsRaw json.RawMessage
	paramsCacheAt       time.Time
}

type bfTxUtxosResponse struct {
	Hash    string `json:"hash"`
	Outputs []struct {
		Address     string `json:"address"`
		OutputIndex int    `json:"output_index"`
	} `json:"outputs"`
}

func txUtxosContainIndexes(body []byte, indexes []uint32) (bool, error) {
	var resp bfTxUtxosResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("parse tx utxos: %w", err)
	}
	present := make(map[uint32]struct{}, len(resp.Outputs))
	for _, o := range resp.Outputs {
		if o.OutputIndex < 0 {
			continue
		}
		present[uint32(o.OutputIndex)] = struct{}{}
	}
	for _, idx := range indexes {
		if _, ok := present[idx]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func txUtxosOutputAddress(body []byte, index uint32) (string, error) {
	var resp bfTxUtxosResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse tx utxos: %w", err)
	}
	for _, o := range resp.Outputs {
		if o.OutputIndex >= 0 && uint32(o.OutputIndex) == index {
			if strings.TrimSpace(o.Address) == "" {
				return "", fmt.Errorf("output %d has empty address", index)
			}
			return o.Address, nil
		}
	}
	return "", fmt.Errorf("utxo not found at index %d", index)
}

func (b *blockfrostProvider) fetchTxUtxos(ctx context.Context, txHex string) ([]byte, int, error) {
	base := strings.TrimRight(b.baseURL, "/")
	url := fmt.Sprintf("%s/txs/%s/utxos", base, txHex)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("project_id", b.projectID)
	client := b.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return body, res.StatusCode, nil
}

func (b *blockfrostProvider) outputsReady(ctx context.Context, txID common.Blake2b256, indexes []uint32) (bool, error) {
	txHex := hex.EncodeToString(txID.Bytes())
	body, status, err := b.fetchTxUtxos(ctx, txHex)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return txUtxosContainIndexes(body, indexes)
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("blockfrost GET /txs/%s/utxos: status %d: %s", txHex, status, truncateForLog(body, 200))
	}
}

func truncateForLog(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// AwaitOutputs polls Blockfrost tx UTxOs directly (not Apollo UtxoByRef).
func (b *blockfrostProvider) AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32, explorerURL string, reporter logging.WaitReporter) error {
	if len(indexes) == 0 {
		return fmt.Errorf("no output indexes to await")
	}
	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()
	txHex := hex.EncodeToString(txID.Bytes())
	slog.Debug("Awaiting transaction outputs via Blockfrost tx utxos", "txId", txHex, "indexes", indexes)

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
	for {
		poll++
		progress.Poll = poll
		if reporter != nil {
			reporter.Tick(progress)
		}
		ready, err := b.outputsReady(ctx, txID, indexes)
		if err != nil {
			slog.Warn("Transaction confirmation poll error", "txId", txHex, "poll", poll, "error", err)
		} else if ready {
			slog.Info("Transaction outputs confirmed", "txId", txHex)
			if reporter != nil {
				reporter.Done(progress, nil)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			err := fmt.Errorf("confirmation timeout or canceled: %w", ctx.Err())
			if reporter != nil {
				reporter.Done(progress, err)
			}
			return err
		case <-ticker.C:
			slog.Log(ctx, logging.LevelTrace, "Polling for transaction outputs", "txId", txHex)
		}
	}
}

// UtxoByRef resolves via tx utxos address lookup + Apollo address UTxOs.
// Avoids Apollo's broken /txs/{hash}/utxos hydrate path.
func (b *blockfrostProvider) UtxoByRef(txHash common.Blake2b256, index uint32) (*common.Utxo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	txHex := hex.EncodeToString(txHash.Bytes())
	body, status, err := b.fetchTxUtxos(ctx, txHex)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("utxo not found")
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("blockfrost GET /txs/%s/utxos: status %d: %s", txHex, status, truncateForLog(body, 200))
	}
	addrStr, err := txUtxosOutputAddress(body, index)
	if err != nil {
		return nil, err
	}
	addr, err := common.NewAddress(addrStr)
	if err != nil {
		return nil, err
	}
	utxos, err := b.Utxos(addr)
	if err != nil {
		return nil, err
	}
	for i := range utxos {
		u := &utxos[i]
		if hex.EncodeToString(u.Id.Id().Bytes()) == txHex && u.Id.Index() == index {
			return u, nil
		}
	}
	return nil, fmt.Errorf("utxo not found")
}
