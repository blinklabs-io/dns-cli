package protocol

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func TestDecodeTLDRegisterDatumRoundTrip(t *testing.T) {
	orig := TLDRegisterDatum{
		TLD:         []byte("hello-handshake"),
		OwnerHNSKey: []byte{0xaa, 0xbb},
		Minted:      0,
	}
	pd, err := orig.ToPlutusData()
	if err != nil {
		t.Fatal(err)
	}
	d := ToDatum(pd)
	got, err := DecodeTLDRegisterDatum(&d)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.TLD) != string(orig.TLD) || got.Minted != orig.Minted {
		t.Fatalf("got %+v want %+v", got, orig)
	}
}

func TestInsertSortedSLD(t *testing.T) {
	base := [][]byte{[]byte("aaa"), []byte("ccc")}
	out, err := InsertSortedSLD(base, []byte("bbb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || string(out[1]) != "bbb" {
		t.Fatalf("unexpected order: %v", out)
	}
	_, err = InsertSortedSLD(base, []byte("aaa"))
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestDecodeFromNilDatum(t *testing.T) {
	if _, err := DecodeTLDRegisterDatum(nil); err == nil {
		t.Fatal("expected error")
	}
	var d common.Datum
	if _, err := DecodeSLDReferenceDatum(&d); err == nil {
		t.Fatal("expected error for empty datum")
	}
}
