package provider

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Apollo's Blockfrost backend surfaces unused addresses as HTTP 404, e.g.
//   blockfrost API error 404: {"status_code":404,"error":"Not Found",...}
// That means "no on-chain history yet" (0 lovelace), not a fatal provider failure.

var (
	reBFStatus   = regexp.MustCompile(`(?i)blockfrost API error (\d{3})`)
	reStatusJSON = regexp.MustCompile(`(?i)"status_code"\s*:\s*(\d{3})`)
)

// IsUnusedAddress reports whether err indicates an address with no on-chain
// history (Blockfrost 404 on address/UTxO endpoints).
func IsUnusedAddress(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	code := providerHTTPStatus(msg)
	if code != 404 {
		return false
	}
	lower := strings.ToLower(msg)
	// Prefer address/utxo query failures; avoid treating unrelated 404s as empty.
	if strings.Contains(lower, "address") ||
		strings.Contains(lower, "utxo") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "component has not been found") {
		return true
	}
	return strings.Contains(lower, "blockfrost")
}

// IsAuthFailure reports Blockfrost/provider authentication or authorization errors.
func IsAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	code := providerHTTPStatus(err.Error())
	return code == 401 || code == 403
}

// IsRateLimited reports provider rate-limit responses.
func IsRateLimited(err error) bool {
	if err == nil {
		return false
	}
	return providerHTTPStatus(err.Error()) == 429
}

func providerHTTPStatus(msg string) int {
	if m := reBFStatus.FindStringSubmatch(msg); len(m) == 2 {
		var code int
		_, _ = fmt.Sscanf(m[1], "%d", &code)
		return code
	}
	if m := reStatusJSON.FindStringSubmatch(msg); len(m) == 2 {
		var code int
		_, _ = fmt.Sscanf(m[1], "%d", &code)
		return code
	}
	return 0
}

// Explain maps a raw provider error to an operator-facing message.
// The original error is preserved via %w for errors.Is / logging.
func Explain(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case IsUnusedAddress(err):
		return fmt.Errorf("address has no on-chain history yet (treated as empty): %w", err)
	case IsAuthFailure(err):
		return fmt.Errorf("provider authentication failed (check project id / API key / headers): %w", err)
	case IsRateLimited(err):
		return fmt.Errorf("provider rate limit exceeded; retry after a short wait: %w", err)
	default:
		if code := providerHTTPStatus(err.Error()); code >= 500 {
			return fmt.Errorf("provider server unavailable (HTTP %d); retry later: %w", code, err)
		}
		return err
	}
}

// Utxos wraps the underlying ChainContext.Utxos call and normalizes Blockfrost
// unused-address 404 responses to an empty UTxO set.
func (w *wrapped) Utxos(address common.Address) ([]common.Utxo, error) {
	utxos, err := w.ChainContext.Utxos(address)
	if err == nil {
		return utxos, nil
	}
	if IsUnusedAddress(err) {
		slog.Debug("Address has no on-chain history; treating as empty UTxO set",
			"provider", w.name,
			"address", address.String(),
		)
		return []common.Utxo{}, nil
	}
	return nil, Explain(err)
}
