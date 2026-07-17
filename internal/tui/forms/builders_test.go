package forms

import "testing"

func TestNewActionFormAllActions(t *testing.T) {
	actions := []string{
		"wallet.create", "wallet.fund", "wallet.balance",
		"proof.generate",
		"system.prepare", "system.init", "system.bind",
		"registrar.register", "owner.activate", "owner.mint", "owner.update",
		"tx.inspect", "tx.sign", "tx.submit", "tx.status",
	}
	for _, a := range actions {
		v := ActionValues{}
		f := NewActionForm(a, &v)
		if f == nil {
			t.Fatalf("%s: nil form", a)
		}
	}
}

func TestParseAllocations(t *testing.T) {
	got := ParseAllocations("registrar=1, tldOwner=2\nsldOwner=3")
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestMutatingAndNeedsConfig(t *testing.T) {
	if !Mutating("tx.sign") || Mutating("wallet.balance") {
		t.Fatal("mutating flags wrong")
	}
	if NeedsConfig("wallet.balance") != true || NeedsConfig("wallet.create") != false {
		t.Fatal("needsConfig flags wrong")
	}
	if NeedsConfig("app.exit") {
		t.Fatal("exit must not require config")
	}
}
