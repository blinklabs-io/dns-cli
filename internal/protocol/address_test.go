package protocol_test

import (
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func TestScriptBaseAddress(t *testing.T) {
	scriptHash := make([]byte, 28)
	stakeHash := make([]byte, 28)
	for i := range scriptHash {
		scriptHash[i] = byte(i + 1)
		stakeHash[i] = byte(i + 100)
	}
	addr, err := protocol.ScriptBaseAddress(protocol.PreprodNetworkID, scriptHash, stakeHash)
	if err != nil {
		t.Fatal(err)
	}
	if addr.Type() != common.AddressTypeScriptKey {
		t.Fatalf("type %d", addr.Type())
	}
	if addr.NetworkId() != uint(protocol.PreprodNetworkID) {
		t.Fatalf("network %d", addr.NetworkId())
	}
	if addr.PaymentKeyHash() != common.Blake2b224(scriptHash) {
		// PaymentKeyHash for script addresses returns script hash via payment payload.
		got := addr.PaymentKeyHash()
		if hex.EncodeToString(got[:]) != hex.EncodeToString(scriptHash) {
			t.Fatalf("payment hash mismatch: %x vs %x", got, scriptHash)
		}
	}
	gotStake := addr.StakeKeyHash()
	if hex.EncodeToString(gotStake[:]) != hex.EncodeToString(stakeHash) {
		t.Fatalf("stake hash mismatch: %x vs %x", gotStake, stakeHash)
	}
	s := addr.String()
	if s == "" || s[:10] != "addr_test1" {
		t.Fatalf("unexpected bech32 %q", s)
	}
	roundTrip, err := common.NewAddress(s)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.String() != s {
		t.Fatalf("round-trip %s vs %s", roundTrip.String(), s)
	}
}

func TestScriptBaseAddressRejectsBadLengths(t *testing.T) {
	if _, err := protocol.ScriptBaseAddress(0, make([]byte, 10), make([]byte, 28)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := protocol.ScriptBaseAddress(0, make([]byte, 28), make([]byte, 10)); err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapAndLoadPlutusEnvelope(t *testing.T) {
	compiled, _ := hex.DecodeString("587f0101003232")
	env, err := protocol.WrapCompiledCodeEnvelope(compiled, "test")
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != protocol.TypePlutusScriptV3 {
		t.Fatalf("type %s", env.Type)
	}
	script, err := protocol.ScriptFromEnvelope(&env)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(script) != hex.EncodeToString(compiled) {
		t.Fatalf("script mismatch %x vs %x", script, compiled)
	}
}
