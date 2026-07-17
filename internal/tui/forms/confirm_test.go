package forms

import "testing"

func TestConfirmAcceptReject(t *testing.T) {
	var c ConfirmState
	c.Ask("Sign?", "actor=bootstrap", "tx.sign")
	if !c.Active || c.Action != "tx.sign" {
		t.Fatalf("ask failed: %+v", c)
	}
	c.Clear()
	if c.Active {
		t.Fatal("expected cleared")
	}
}
