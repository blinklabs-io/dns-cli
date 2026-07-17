package tui

import (
	"strings"
	"testing"
)

func TestChecklistMarks(t *testing.T) {
	s := statusState{Checklist: checklist{Prepare: true, Register: true}}
	view := renderStatusPane(s)
	if !strings.Contains(view, "[x] prepare") {
		t.Fatalf("missing prepare mark in %q", view)
	}
	if !strings.Contains(view, "[ ] activate") {
		t.Fatalf("missing activate mark in %q", view)
	}
}
