package txbuilder

import (
	"testing"

	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	"github.com/blinklabs-io/plutigo/data"
)

func TestComputeConwayScriptDataHashOmitsEmptyDatums(t *testing.T) {
	pd := data.NewConstr(0)
	redeemerKey := common.RedeemerKey{Tag: common.RedeemerTagMint, Index: 0}
	redeemerVal := common.RedeemerValue{
		Data:    protocol.ToDatum(pd),
		ExUnits: common.ExUnits{Memory: 1000, Steps: 2000},
	}
	tx := &conway.ConwayTransaction{
		WitnessSet: conway.ConwayTransactionWitnessSet{
			WsRedeemers: conway.ConwayRedeemers{
				Redeemers: map[common.RedeemerKey]common.RedeemerValue{
					redeemerKey: redeemerVal,
				},
			},
		},
	}
	// Preserve redeemer CBOR as the ledger does after decode.
	reb, err := cbor.Encode(tx.WitnessSet.WsRedeemers.Redeemers)
	if err != nil {
		t.Fatal(err)
	}
	tx.WitnessSet.WsRedeemers.SetCbor(reb)

	costs := map[string][]int64{
		"PlutusV3": {1, 2, 3, 4, 5},
	}
	got, err := computeConwayScriptDataHash(tx, costs)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected hash")
	}

	// Manual preimage: redeemers || (no datums) || lang views for V3 only.
	lang, err := common.EncodeLangViews(map[uint]struct{}{2: {}}, map[uint][]int64{2: costs["PlutusV3"]})
	if err != nil {
		t.Fatal(err)
	}
	preimage := append(append([]byte{}, reb...), lang...)
	want := common.Blake2b256Hash(preimage)
	if *got != want {
		t.Fatalf("hash mismatch:\n got %x\nwant %x", got.Bytes(), want.Bytes())
	}

	// Apollo-style empty datums must NOT match.
	emptyDatums, err := cbor.Encode([]common.Datum{})
	if err != nil {
		t.Fatal(err)
	}
	apolloPreimage := append(append(append([]byte{}, reb...), emptyDatums...), lang...)
	apolloHash := common.Blake2b256Hash(apolloPreimage)
	if *got == apolloHash {
		t.Fatal("hash unexpectedly matches Apollo empty-datums preimage")
	}
}
