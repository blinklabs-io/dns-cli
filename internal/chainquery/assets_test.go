package chainquery

import "testing"

func TestContainsSLD(t *testing.T) {
	slds := [][]byte{[]byte("aaa"), []byte("bbb")}
	if !ContainsSLD(slds, []byte("aaa")) {
		t.Fatal("expected true")
	}
	if ContainsSLD(slds, []byte("ccc")) {
		t.Fatal("expected false")
	}
}
