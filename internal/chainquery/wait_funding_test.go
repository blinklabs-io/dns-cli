package chainquery

import "testing"

func TestNormalizeRefs(t *testing.T) {
	m := normalizeRefs([]string{" Ab#1 ", "ab#1", ""})
	if len(m) != 1 {
		t.Fatalf("got %d", len(m))
	}
	if _, ok := m["ab#1"]; !ok {
		t.Fatalf("missing key: %#v", m)
	}
}

func TestFundingSyncStateEmpty(t *testing.T) {
	stale, total := fundingSyncState(nil, normalizeRefs([]string{"aa#0"}))
	if len(stale) != 0 || total != 0 {
		t.Fatalf("stale=%v total=%d", stale, total)
	}
}

func TestBlakeFromHex(t *testing.T) {
	hex32 := "e644611f706ed566e7dcc131803d2da89df72991f0a196739290553467b3af47"
	h, err := blakeFromHex(hex32)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 32 {
		t.Fatalf("len=%d", len(h))
	}
	if _, err := blakeFromHex("abcd"); err == nil {
		t.Fatal("expected error")
	}
}
