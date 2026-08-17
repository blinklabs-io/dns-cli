package demo

import "testing"

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
