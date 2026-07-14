package protocol

import (
	"encoding/hex"
	"testing"
)

func TestTokenNamesDeterministic(t *testing.T) {
	name := []byte("hello-handshake")
	r := CreateReferenceTokenTN(name)
	u := CreateUserTokenTN(name)
	if len(r) != 64 || len(u) != 64 {
		t.Fatalf("unexpected lengths %d %d", len(r), len(u))
	}
	if r == u {
		t.Fatal("reference and user token names must differ")
	}
	// Round-trip decode
	if _, err := hex.DecodeString(r); err != nil {
		t.Fatal(err)
	}
}

func TestTLDRegisterDatumEncode(t *testing.T) {
	d := TLDRegisterDatum{
		TLD:         []byte("hello-handshake"),
		OwnerHNSKey: []byte{1, 2, 3},
		Minted:      0,
	}
	pd, err := d.ToPlutusData()
	if err != nil || pd == nil {
		t.Fatalf("encode: %v %v", pd, err)
	}
}

func TestOptionTTL(t *testing.T) {
	none := optionInt(nil)
	if none == nil {
		t.Fatal("nil option")
	}
	v := int64(300)
	some := optionInt(&v)
	if some == nil {
		t.Fatal("some option")
	}
}
