package logging

import (
	"encoding/hex"

	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/spf13/cobra"
)

// RedactHex returns a short non-secret fingerprint for binary fields.
func RedactHex(b []byte) string {
	return domain.Fingerprint(b)
}

// Command returns slog attrs including the cobra command path.
func Command(cmd *cobra.Command, attrs ...any) []any {
	out := make([]any, 0, len(attrs)+2)
	if cmd != nil {
		out = append(out, "command", cmd.CommandPath())
	}
	out = append(out, attrs...)
	return out
}

// HexPrefix returns a truncated hex prefix safe for logs.
func HexPrefix(s string, maxBytes int) string {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) == 0 {
		if len(s) <= maxBytes*2 {
			return s
		}
		return s[:maxBytes*2] + "…"
	}
	fp := domain.Fingerprint(b)
	return fp + "…"
}
