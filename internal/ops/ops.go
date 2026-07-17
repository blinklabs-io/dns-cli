// Package ops is the shared façade for dns-cli business operations.
// Cobra commands and the TUI call this package. Ops returns plain errors;
// the CLI layer maps them to process exit codes via WrapExit.
package ops

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Client holds shared options for ops calls.
type Client struct {
	// ContractRevision is stamped into built transaction metadata.
	ContractRevision string
}

// New returns a Client. Empty contractRevision becomes "unknown".
func New(contractRevision string) *Client {
	if strings.TrimSpace(contractRevision) == "" {
		contractRevision = "unknown"
	}
	return &Client{ContractRevision: contractRevision}
}

// TxIDToBlake converts a hex transaction id to Blake2b256 for provider queries.
func TxIDToBlake(s string) (common.Blake2b256, error) {
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
