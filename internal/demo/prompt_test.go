package demo

import (
	"bytes"
	"strings"
	"testing"
)

func TestAskChoiceNumberedMenu(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("2\n")
	p := NewPrompter(in, &out, false)
	got := p.AskChoice("Mode", "fresh", []string{"fresh", "existing"})
	if got != "existing" {
		t.Fatalf("got %q want existing", got)
	}
	text := out.String()
	for _, want := range []string{"Mode", "1) fresh (default)", "2) existing", "Enter number [1]:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestAskChoiceDefaultOnEmpty(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("\n")
	p := NewPrompter(in, &out, false)
	got := p.AskChoice("Provider", "blockfrost", []string{"blockfrost", "utxorpc"})
	if got != "blockfrost" {
		t.Fatalf("got %q want blockfrost", got)
	}
}

func TestAskChoiceAcceptsName(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("utxorpc\n")
	p := NewPrompter(in, &out, false)
	got := p.AskChoice("Provider", "blockfrost", []string{"blockfrost", "utxorpc"})
	if got != "utxorpc" {
		t.Fatalf("got %q want utxorpc", got)
	}
}

func TestAskChoiceInvalidKeepsDefault(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("9\n")
	p := NewPrompter(in, &out, false)
	got := p.AskChoice("Mode", "fresh", []string{"fresh", "existing"})
	if got != "fresh" {
		t.Fatalf("got %q want fresh", got)
	}
	if !strings.Contains(out.String(), "Invalid choice") {
		t.Fatalf("expected invalid choice message:\n%s", out.String())
	}
}

func TestAskChoiceAssumeYes(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader(""), &out, true)
	got := p.AskChoice("Mode", "existing", []string{"fresh", "existing"})
	if got != "existing" {
		t.Fatalf("got %q want existing", got)
	}
	if !strings.Contains(out.String(), "assume-yes") {
		t.Fatalf("expected assume-yes:\n%s", out.String())
	}
}

func TestAskChoiceFallsBackToString(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("www\n")
	p := NewPrompter(in, &out, false)
	got := p.AskChoice("SLD", "app", nil)
	if got != "www" {
		t.Fatalf("got %q want www", got)
	}
}
