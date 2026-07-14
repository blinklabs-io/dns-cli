package protocol_test

import (
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/plutigo/data"
)

func TestEncodeByteArrayCBORHex(t *testing.T) {
	got, err := protocol.EncodeByteArrayCBORHex([]byte{0xab, 0xcd})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(got)
	if err != nil {
		t.Fatal(err)
	}
	pd, err := data.Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	bs, ok := pd.(*data.ByteString)
	if !ok {
		t.Fatalf("got %T", pd)
	}
	if hex.EncodeToString(bs.Inner) != "abcd" {
		t.Fatalf("got %x", bs.Inner)
	}
}

func TestEncodePolicyIDCBORHex(t *testing.T) {
	policy := make([]byte, 28)
	for i := range policy {
		policy[i] = byte(i)
	}
	got, err := protocol.EncodePolicyIDCBORHex(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.EncodePolicyIDCBORHex(policy[:10]); err == nil {
		t.Fatal("expected length error")
	}
	raw, _ := hex.DecodeString(got)
	pd, err := data.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	bs := pd.(*data.ByteString)
	if len(bs.Inner) != 28 {
		t.Fatalf("len %d", len(bs.Inner))
	}
}

func TestEncodeStakeKeyHashCredentialCBORHex(t *testing.T) {
	hash := make([]byte, 28)
	hash[0] = 0x11
	got, err := protocol.EncodeStakeKeyHashCredentialCBORHex(hash)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(got)
	if err != nil {
		t.Fatal(err)
	}
	pd, err := data.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Inline(VerificationKey(hash)) = Constr(0, [Constr(0, [Bytes])])
	outer, ok := pd.(*data.Constr)
	if !ok || outer.Tag != 0 || len(outer.Fields) != 1 {
		t.Fatalf("outer: %+v", pd)
	}
	inner, ok := outer.Fields[0].(*data.Constr)
	if !ok || inner.Tag != 0 || len(inner.Fields) != 1 {
		t.Fatalf("inner: %+v", outer.Fields[0])
	}
	bs, ok := inner.Fields[0].(*data.ByteString)
	if !ok || len(bs.Inner) != 28 || bs.Inner[0] != 0x11 {
		t.Fatalf("hash field: %+v", inner.Fields[0])
	}
}
