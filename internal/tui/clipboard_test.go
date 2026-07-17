package tui

import "testing"

func TestCopyTargetPriority(t *testing.T) {
	if got := CopyTarget("txid", "http://x", "/a"); got != "txid" {
		t.Fatalf("got %q", got)
	}
	if got := CopyTarget("", "http://x", "/a"); got != "http://x" {
		t.Fatalf("got %q", got)
	}
	if got := CopyTarget("", "", "/a"); got != "/a" {
		t.Fatalf("got %q", got)
	}
}
