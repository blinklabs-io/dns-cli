package txbuilder

import (
	"fmt"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
)

// fixScriptDataHash recomputes TxScriptDataHash the Conway ledger way.
//
// Apollo's ComputeScriptDataHash always CBOR-encodes an empty datum list when
// there are no witness datums. Conway (and Alonzo/Babbage) omit datums from the
// preimage when WsPlutusData is empty, which yields ScriptIntegrityHashMismatch
// on submit for mint-only txs with inline output datums only.
func (c *Context) fixScriptDataHash(txCbor []byte) ([]byte, error) {
	tx, err := artifact.DecodeConwayTx(txCbor)
	if err != nil {
		return nil, err
	}
	if tx.Body.TxScriptDataHash == nil {
		return txCbor, nil
	}
	pp, err := c.Provider.ProtocolParams()
	if err != nil {
		return nil, fmt.Errorf("protocol params for script data hash: %w", err)
	}
	hash, err := computeConwayScriptDataHash(tx, pp.CostModels)
	if err != nil {
		return nil, err
	}
	tx.Body.TxScriptDataHash = hash
	tx.SetCbor(nil)
	tx.Body.SetCbor(nil)
	return artifact.EncodeConwayTx(tx)
}

func computeConwayScriptDataHash(tx *conway.ConwayTransaction, costModels map[string][]int64) (*common.Blake2b256, error) {
	wits := &tx.WitnessSet
	hasRedeemers := wits.WsRedeemers.Len() > 0
	hasDatums := len(wits.WsPlutusData.Items()) > 0
	if !hasRedeemers && !hasDatums {
		return nil, nil
	}

	redeemersCbor := wits.WsRedeemers.Cbor()
	if len(redeemersCbor) == 0 {
		if wits.WsRedeemers.Len() == 0 {
			redeemersCbor = []byte{0xa0}
		} else {
			var err error
			redeemersCbor, err = wits.WsRedeemers.MarshalCBOR()
			if err != nil {
				return nil, fmt.Errorf("encode redeemers: %w", err)
			}
		}
	}

	var datumsCbor []byte
	if hasDatums {
		datumsCbor = wits.WsPlutusData.Cbor()
		if len(datumsCbor) == 0 {
			var err error
			datumsCbor, err = cbor.Encode(wits.WsPlutusData)
			if err != nil {
				return nil, fmt.Errorf("encode datums: %w", err)
			}
		}
	}

	usedVersions := plutusVersionsInWitness(wits)
	if len(usedVersions) == 0 && hasRedeemers {
		// dns-cli scripts are PlutusV3 reference scripts; mint/spend redeemers
		// without inline witness scripts still require the V3 language view.
		usedVersions[2] = struct{}{}
	}
	numeric := make(map[uint][]int64, len(costModels))
	for lang, costs := range costModels {
		var version uint
		switch lang {
		case "PlutusV1":
			version = 0
		case "PlutusV2":
			version = 1
		case "PlutusV3":
			version = 2
		default:
			continue
		}
		numeric[version] = costs
	}
	langViewsCbor, err := common.EncodeLangViews(usedVersions, numeric)
	if err != nil {
		return nil, fmt.Errorf("encode lang views: %w", err)
	}

	preimage := make([]byte, 0, len(redeemersCbor)+len(datumsCbor)+len(langViewsCbor))
	preimage = append(preimage, redeemersCbor...)
	preimage = append(preimage, datumsCbor...)
	preimage = append(preimage, langViewsCbor...)
	hash := common.Blake2b256Hash(preimage)
	return &hash, nil
}

func plutusVersionsInWitness(wits *conway.ConwayTransactionWitnessSet) map[uint]struct{} {
	used := make(map[uint]struct{})
	if len(wits.WsPlutusV1Scripts.Items()) > 0 {
		used[0] = struct{}{}
	}
	if len(wits.WsPlutusV2Scripts.Items()) > 0 {
		used[1] = struct{}{}
	}
	if len(wits.WsPlutusV3Scripts.Items()) > 0 {
		used[2] = struct{}{}
	}
	return used
}
