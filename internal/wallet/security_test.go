package wallet

import (
	"strings"
	"testing"
)

func TestSourceStringRedactsMnemonic(t *testing.T) {
	s := Source{
		Name:        "tldOwner",
		MnemonicEnv: "DNS_CLI_TLD_OWNER_MNEMONIC",
	}
	out := s.String()
	if strings.Contains(out, "abandon") {
		t.Fatal("string must not contain mnemonic words")
	}
	if !strings.Contains(out, "mnemonicEnv") {
		t.Fatal("expected mnemonicEnv reference")
	}
}
