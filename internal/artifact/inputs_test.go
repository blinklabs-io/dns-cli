package artifact

import "testing"

func TestTxInputRefsMissing(t *testing.T) {
	if _, err := TxInputRefs(t.TempDir() + "/missing.json"); err == nil {
		t.Fatal("expected error for missing envelope")
	}
}

func TestUTxORef(t *testing.T) {
	got := UTxORef([]byte{0xab, 0xcd}, 3)
	if got != "abcd#3" {
		t.Fatalf("got %q", got)
	}
}
