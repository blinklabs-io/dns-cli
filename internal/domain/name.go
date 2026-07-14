// Package domain validates DNS labels, proof bundles, and record inputs.
package domain

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

// Label is a validated canonical DNS label (TLD or SLD).
type Label struct {
	Original  string
	Canonical string
	Bytes     []byte
}

// ParseLabel normalizes and validates a single DNS label (no dots).
func ParseLabel(raw string) (Label, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Label{}, fmt.Errorf("label is empty")
	}
	if strings.Contains(raw, ".") {
		return Label{}, fmt.Errorf("label %q must not contain dots; pass TLD and SLD separately", raw)
	}
	ascii, err := idna.Lookup.ToASCII(raw)
	if err != nil {
		return Label{}, fmt.Errorf("invalid label %q: %w", raw, err)
	}
	ascii = strings.ToLower(ascii)
	if len(ascii) == 0 || len(ascii) > 63 {
		return Label{}, fmt.Errorf("label %q length must be 1..63", raw)
	}
	if ascii[0] == '-' || ascii[len(ascii)-1] == '-' {
		return Label{}, fmt.Errorf("label %q must not start or end with hyphen", raw)
	}
	for _, r := range ascii {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			continue
		}
		return Label{}, fmt.Errorf("label %q has invalid character %q", raw, r)
	}
	return Label{Original: raw, Canonical: ascii, Bytes: []byte(ascii)}, nil
}
