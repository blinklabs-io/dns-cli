package domain

import (
	"strings"
	"testing"
)

func FuzzParseLabel(f *testing.F) {
	seeds := []string{"hello", "HELLO-WORLD", "xn--nxasmq5b", "", "a.b", "-bad", "good-label"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		label, err := ParseLabel(raw)
		if err != nil {
			return
		}
		if strings.Contains(label.Canonical, ".") {
			t.Fatalf("canonical contains dot: %q", label.Canonical)
		}
		if len(label.Bytes) == 0 {
			t.Fatal("empty bytes")
		}
	})
}
