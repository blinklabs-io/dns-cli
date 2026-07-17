package wallet

import (
	"crypto/ed25519"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
)

type staticSigner struct{}

func (staticSigner) SignTxBody(common.Blake2b256) (common.VkeyWitness, error) {
	return common.VkeyWitness{
		Vkey:      ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)),
		Signature: make([]byte, ed25519.SignatureSize),
	}, nil
}

func (staticSigner) PubKeyHash() common.Blake2b224 {
	return common.Blake2b224{}
}

func TestSignEnvelopeClearsCachedCBOR(t *testing.T) {
	original := &conway.ConwayTransaction{
		WitnessSet: conway.ConwayTransactionWitnessSet{},
		TxIsValid:  true,
	}
	raw, err := cbor.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded conway.ConwayTransaction
	if _, err := cbor.Decode(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Cbor() == nil {
		t.Fatal("expected decoded transaction to retain original CBOR")
	}

	if err := SignEnvelope(&decoded, staticSigner{}, "", nil, false); err != nil {
		t.Fatal(err)
	}
	encoded, err := cbor.Encode(&decoded)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip conway.ConwayTransaction
	if _, err := cbor.Decode(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if got := len(roundTrip.WitnessSet.VkeyWitnesses.Items()); got != 1 {
		t.Fatalf("witness count after encode/decode = %d, want 1", got)
	}
}
