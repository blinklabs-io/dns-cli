package cli

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func decodeHex32(s string) ([]byte, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("want 32-byte hash, got %d", len(b))
	}
	return b, nil
}

// txIDToBlake converts hex tx id to Blake2b256 for provider queries.
func txIDToBlake(s string) (common.Blake2b256, error) {
	b, err := decodeHex32(s)
	if err != nil {
		return common.Blake2b256{}, err
	}
	var h common.Blake2b256
	copy(h[:], b)
	return h, nil
}
