package domain

import "testing"

func TestParseLabel(t *testing.T) {
	l, err := ParseLabel("Hello-Handshake")
	if err != nil {
		t.Fatal(err)
	}
	if l.Canonical != "hello-handshake" {
		t.Fatalf("got %s", l.Canonical)
	}
}

func TestParseLabelRejectsDot(t *testing.T) {
	if _, err := ParseLabel("a.b"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordsEmptyOK(t *testing.T) {
	// clear-all is supported
	f := RecordsFile{Records: nil}
	if len(f.Records) != 0 {
		t.Fatal("expected empty")
	}
}
