package wallet

import (
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func testActorAddress(t *testing.T) string {
	t.Helper()
	var raw [57]byte
	raw[0] = 0x00
	raw[1] = 0xAA
	raw[29] = 0xBB
	addr, err := common.NewAddressFromBytes(raw[:])
	if err != nil {
		t.Fatal(err)
	}
	return addr.String()
}

func TestFromActorMutualExclusion(t *testing.T) {
	_, err := FromActor("bad", config.ActorConfig{
		Address:        testActorAddress(t),
		SigningKeyFile: "keys/a.skey",
		MnemonicEnv:    "BOTH",
	}, "preview")
	if err == nil {
		t.Fatal("expected error when both credentials set")
	}
}

func TestFromActorRequiresOneCredential(t *testing.T) {
	_, err := FromActor("bad", config.ActorConfig{
		Address: testActorAddress(t),
	}, "preview")
	if err == nil {
		t.Fatal("expected error when no credential set")
	}
}
