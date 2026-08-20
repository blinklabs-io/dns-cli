package demo

import (
	"path/filepath"
	"testing"
)

// TestTLDStateStepRoundTrip pins setStep/stepTxID for every TLDState key.
// mintRegistrarToken previously had no backing field in TLDState.Confirmed,
// so setStep silently dropped it and stepTxID always returned "" — meaning
// a confirmed mint-registrar-token step was never actually recorded, and
// every resume re-minted a fresh registrar NFT.
func TestTLDStateStepRoundTrip(t *testing.T) {
	for _, key := range []string{"mintRegistrarToken", "fund", "deploy", "register", "activate"} {
		var st TLDState
		if got := st.stepTxID(key); got != "" {
			t.Fatalf("%s: stepTxID on zero-value state = %q, want empty", key, got)
		}
		st.setStep(key, StepResult{TxID: "tx-" + key, Manifest: "manifest-" + key})
		if got := st.stepTxID(key); got != "tx-"+key {
			t.Fatalf("%s: stepTxID after setStep = %q, want %q", key, got, "tx-"+key)
		}
	}
}

// TestTLDStateFileRoundTrip exercises the real persistence path
// (writeJSONAtomic -> loadTLDState) rather than just the in-memory
// setStep/stepTxID accessors, so a struct-tag typo or a step silently
// missing its JSON field (as mintRegistrarToken did) would be caught
// even if the accessor plumbing were correct.
func TestTLDStateFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	var st TLDState
	st.SchemaVersion = SchemaVersionState
	st.Mode = "fresh"
	st.Network = "preprod"
	st.Provider = "blockfrost"
	st.TLD = "example"
	for _, key := range []string{"mintRegistrarToken", "fund", "deploy", "register", "activate"} {
		st.setStep(key, StepResult{TxID: "tx-" + key, Manifest: "manifest-" + key})
	}

	if err := writeJSONAtomic(path, &st); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTLDState(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"mintRegistrarToken", "fund", "deploy", "register", "activate"} {
		if got := loaded.stepTxID(key); got != "tx-"+key {
			t.Fatalf("%s: stepTxID after file round-trip = %q, want %q", key, got, "tx-"+key)
		}
	}
}

func TestSLDStateStepRoundTrip(t *testing.T) {
	for _, key := range []string{"mintSld", "updateSld"} {
		var st SLDState
		if got := st.stepTxID(key); got != "" {
			t.Fatalf("%s: stepTxID on zero-value state = %q, want empty", key, got)
		}
		st.setStep(key, StepResult{TxID: "tx-" + key, Manifest: "manifest-" + key})
		if got := st.stepTxID(key); got != "tx-"+key {
			t.Fatalf("%s: stepTxID after setStep = %q, want %q", key, got, "tx-"+key)
		}
	}
}
