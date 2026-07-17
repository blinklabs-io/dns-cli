package tui

import (
	"strings"

	"github.com/atotto/clipboard"
)

// CopyTarget picks the best value to copy: txId > explorer > artifact.
func CopyTarget(txID, explorer, artifact string) string {
	if s := strings.TrimSpace(txID); s != "" {
		return s
	}
	if s := strings.TrimSpace(explorer); s != "" {
		return s
	}
	return strings.TrimSpace(artifact)
}

func copyToClipboard(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return clipboard.WriteAll(s)
}
