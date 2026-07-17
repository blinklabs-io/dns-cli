package demo

import "strings"

func isSkippedRunsChild(name string) bool {
	switch name {
	case "shared", "states":
		return true
	default:
		return false
	}
}

func isSkippedTLDChild(name string) bool {
	switch name {
	case "contracts", "proofs", "config", "artifacts":
		return true
	default:
		return false
	}
}

func explorerURL(txID string) string {
	txID = strings.TrimSpace(txID)
	if txID == "" {
		return ""
	}
	return ExplorerURLPrefix + txID
}

func historyTx(step StepResult) HistoryTx {
	return HistoryTx{
		TxID:        step.TxID,
		Manifest:    step.Manifest,
		ExplorerURL: explorerURL(step.TxID),
	}
}
