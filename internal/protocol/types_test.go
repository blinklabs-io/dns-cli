package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
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

func TestRegisterTLDRedeemerEncode(t *testing.T) {
	pd, err := RegisterTLDRedeemer{
		TLD:                  []byte("hello-handshake"),
		Owner:                []byte{1, 2, 3},
		TLDReferencePolicyID: make([]byte, 28),
	}.ToPlutusData()
	if err != nil {
		t.Fatal(err)
	}
	constr, ok := pd.(*data.Constr)
	if !ok || constr.Tag != ConstrRegisterTLD || len(constr.Fields) != 3 {
		t.Fatalf("got %+v", pd)
	}
}

// TestOwnerActionRedeemerEncode pins ReceiverAddress's encoded bytes against
// receiverAddressCborHex from decentralized-dns-contracts/hns-sig/sign.js,
// which encodes mock_pub_key_address("u") — proven correct against Aiken's
// own serialise_data by success_tld_owner_spend2 (see the OutputReference
// encoder test for the full argument). Confirms gouroboros'
// common.Address.ToPlutusData(), not a hand-rolled encoder, produces the
// exact on-chain byte layout.
func TestOwnerActionRedeemerEncode(t *testing.T) {
	paymentHash := common.Blake2b224Hash([]byte("u"))
	addr, err := common.NewAddressFromParts(common.AddressTypeKeyNone, PreprodNetworkID, paymentHash[:], nil)
	if err != nil {
		t.Fatal(err)
	}
	pd, err := OwnerActionRedeemer{
		OwnerSignature:  []byte{0xaa, 0xbb},
		ReceiverAddress: addr,
	}.ToPlutusData()
	if err != nil {
		t.Fatal(err)
	}
	constr, ok := pd.(*data.Constr)
	if !ok || constr.Tag != ConstrOwnerAction || len(constr.Fields) != 2 {
		t.Fatalf("got %+v", pd)
	}
	addrRaw, err := data.Encode(constr.Fields[1])
	if err != nil {
		t.Fatal(err)
	}
	want := "d8799fd8799f581ccf2020680b6315ff98ffdddde4400839a628e2360a1d1a20ed519439ffd87a80ff"
	if hex.EncodeToString(addrRaw) != want {
		t.Fatalf("receiver address got %s want %s", hex.EncodeToString(addrRaw), want)
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
