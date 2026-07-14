package protocol

import (
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
)

// ToDatum wraps encoded Plutus data for Apollo/gouroboros transaction building.
func ToDatum(pd data.PlutusData) common.Datum {
	return common.Datum{Data: pd}
}

// ToDatumPtr returns a pointer suitable for PayToContract.
func ToDatumPtr(pd data.PlutusData) *common.Datum {
	d := ToDatum(pd)
	return &d
}
