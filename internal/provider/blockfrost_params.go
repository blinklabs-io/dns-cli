package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Salvionied/apollo/v2/backend"
)

// ProtocolParams returns Apollo's Blockfrost params with cost models replaced
// by Blockfrost cost_models_raw when available.
//
// Named cost_models can be truncated/reordered after Plutus cost-model bumps;
// that produces ScriptIntegrityHashMismatch on submit even when EvaluateTx
// succeeds. Prefer the canonical raw arrays (same approach as cardano-client-lib).
func (b *blockfrostProvider) ProtocolParams() (backend.ProtocolParameters, error) {
	pp, err := b.ChainContext.ProtocolParams()
	if err != nil {
		return backend.ProtocolParameters{}, err
	}
	raw, err := b.costModelsRaw(context.Background())
	if err != nil {
		slog.Warn("Failed to load Blockfrost cost_models_raw; using named cost_models", "error", err)
		return pp, nil
	}
	merged, err := mergeCostModelsPreferRaw(pp.CostModels, raw)
	if err != nil {
		return backend.ProtocolParameters{}, err
	}
	if merged != nil {
		pp.CostModels = merged
	}
	return pp, nil
}

func mergeCostModelsPreferRaw(named map[string][]int64, rawJSON json.RawMessage) (map[string][]int64, error) {
	if len(rawJSON) == 0 || string(rawJSON) == "null" {
		return named, nil
	}
	var raw map[string][]int64
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse cost_models_raw: %w", err)
	}
	if len(raw) == 0 {
		return named, nil
	}
	out := make(map[string][]int64, len(raw))
	for lang, costs := range raw {
		if len(costs) == 0 {
			continue
		}
		dup := make([]int64, len(costs))
		copy(dup, costs)
		out[lang] = dup
	}
	if len(out) == 0 {
		return named, nil
	}
	return out, nil
}

func (b *blockfrostProvider) costModelsRaw(ctx context.Context) (json.RawMessage, error) {
	b.paramsMu.Lock()
	if b.cachedCostModelsRaw != nil && time.Since(b.paramsCacheAt) < 2*time.Minute {
		raw := append(json.RawMessage(nil), b.cachedCostModelsRaw...)
		b.paramsMu.Unlock()
		return raw, nil
	}
	b.paramsMu.Unlock()

	base := strings.TrimRight(b.baseURL, "/")
	url := base + "/epochs/latest/parameters"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("project_id", b.projectID)
	client := b.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blockfrost GET /epochs/latest/parameters: status %d: %s", res.StatusCode, truncateForLog(body, 200))
	}
	var parsed struct {
		CostModelsRaw json.RawMessage `json:"cost_models_raw"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse epoch parameters: %w", err)
	}
	if len(parsed.CostModelsRaw) == 0 || string(parsed.CostModelsRaw) == "null" {
		return nil, fmt.Errorf("cost_models_raw missing in epoch parameters")
	}

	b.paramsMu.Lock()
	b.cachedCostModelsRaw = append(json.RawMessage(nil), parsed.CostModelsRaw...)
	b.paramsCacheAt = time.Now()
	b.paramsMu.Unlock()
	return append(json.RawMessage(nil), parsed.CostModelsRaw...), nil
}
