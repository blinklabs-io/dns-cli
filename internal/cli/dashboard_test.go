package cli

import (
	"testing"
)

func TestRootContainsDashboard(t *testing.T) {
	root := NewRoot()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "dashboard" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected dashboard command on root")
	}
}
