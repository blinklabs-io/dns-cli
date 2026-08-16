// Package protocol encodes Handshake DNS contract datums and redeemers.
package protocol

import (
	"fmt"
	"math/big"

	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/plutigo/data"
)

// Constructor indexes match dns-contracts/onchain/lib/types.ak.
const (
	ConstrTLDRegisterDatum = 0

	ConstrRegisterTLD     = 0
	ConstrRegistrarAction = 1
	ConstrOwnerAction     = 2

	ConstrDNSRecord = 0

	ConstrTLDReferenceDatum = 0

	ConstrInitRemoveReference     = 0
	ConstrMintAdditionalReference = 1
	ConstrSpendReference          = 2
	ConstrBurnReference           = 3

	ConstrSLDReferenceDatum = 0
	ConstrMintSld           = 0
)

// TLDRegisterDatum mirrors types.TLDRegisterDatum.
type TLDRegisterDatum struct {
	TLD         []byte
	OwnerHNSKey []byte
	Minted      int64
}

// ToPlutusData encodes the datum.
func (d TLDRegisterDatum) ToPlutusData() (data.PlutusData, error) {
	return data.NewConstr(ConstrTLDRegisterDatum,
		data.NewByteString(d.TLD),
		data.NewByteString(d.OwnerHNSKey),
		data.NewInteger(big.NewInt(d.Minted)),
	), nil
}

// RegisterTLDRedeemer is RegistrarRedeemer.RegisterTLD.
type RegisterTLDRedeemer struct {
	TLD                  []byte
	Owner                []byte
	RegistrarSignature   []byte
	TLDReferencePolicyID []byte
}

func (r RegisterTLDRedeemer) ToPlutusData() (data.PlutusData, error) {
	return data.NewConstr(ConstrRegisterTLD,
		data.NewByteString(r.TLD),
		data.NewByteString(r.Owner),
		data.NewByteString(r.RegistrarSignature),
		data.NewByteString(r.TLDReferencePolicyID),
	), nil
}

// OwnerActionRedeemer is RegistrarRedeemer.OwnerAction.
type OwnerActionRedeemer struct {
	OwnerSignature []byte
}

func (r OwnerActionRedeemer) ToPlutusData() (data.PlutusData, error) {
	return data.NewConstr(ConstrOwnerAction, data.NewByteString(r.OwnerSignature)), nil
}

// DNSRecord mirrors types.DNSRecord.
type DNSRecord struct {
	LHS   []byte
	TTL   *int64
	Class []byte
	RType []byte
	RData []byte
}

func (r DNSRecord) ToPlutusData() (data.PlutusData, error) {
	ttl := optionInt(r.TTL)
	return data.NewConstr(ConstrDNSRecord,
		data.NewByteString(r.LHS),
		ttl,
		data.NewByteString(r.Class),
		data.NewByteString(r.RType),
		data.NewByteString(r.RData),
	), nil
}

// FromParsed converts a domain.ParsedRecord.
func FromParsed(p domain.ParsedRecord) DNSRecord {
	return DNSRecord{LHS: p.LHS, TTL: p.TTL, Class: p.Class, RType: p.RType, RData: p.RData}
}

// TLDReferenceDatum mirrors types.TLDReferenceDatum.
type TLDReferenceDatum struct {
	TLD                  []byte
	SLDs                 [][]byte
	SLDReferencePolicyID []byte
	Next                 []byte
	Records              []DNSRecord
}

func (d TLDReferenceDatum) ToPlutusData() (data.PlutusData, error) {
	slds := make([]data.PlutusData, len(d.SLDs))
	for i, s := range d.SLDs {
		slds[i] = data.NewByteString(s)
	}
	recs := make([]data.PlutusData, len(d.Records))
	for i, r := range d.Records {
		pd, err := r.ToPlutusData()
		if err != nil {
			return nil, err
		}
		recs[i] = pd
	}
	return data.NewConstr(ConstrTLDReferenceDatum,
		data.NewByteString(d.TLD),
		data.NewList(slds...),
		data.NewByteString(d.SLDReferencePolicyID),
		data.NewByteString(d.Next),
		data.NewList(recs...),
	), nil
}

// SpendReferenceRedeemer is TLDReferenceAction.SpendReference (constructor 2, no fields).
func SpendReferenceRedeemer() data.PlutusData {
	return data.NewConstr(ConstrSpendReference)
}

// InitRemoveReferenceRedeemer is TLDReferenceAction.InitRemoveReference.
func InitRemoveReferenceRedeemer() data.PlutusData {
	return data.NewConstr(ConstrInitRemoveReference)
}

// SLDReferenceDatum mirrors types.SLDReferenceDatum.
type SLDReferenceDatum struct {
	TLD     []byte
	SLD     []byte
	Records []DNSRecord
}

func (d SLDReferenceDatum) ToPlutusData() (data.PlutusData, error) {
	recs := make([]data.PlutusData, len(d.Records))
	for i, r := range d.Records {
		pd, err := r.ToPlutusData()
		if err != nil {
			return nil, err
		}
		recs[i] = pd
	}
	return data.NewConstr(ConstrSLDReferenceDatum,
		data.NewByteString(d.TLD),
		data.NewByteString(d.SLD),
		data.NewList(recs...),
	), nil
}

// MintSldRedeemer mirrors types.MintSld.
type MintSldRedeemer struct {
	TLD      []byte
	MintSLDs [][]byte
	BurnSLDs [][]byte
}

func (r MintSldRedeemer) ToPlutusData() (data.PlutusData, error) {
	mints := make([]data.PlutusData, len(r.MintSLDs))
	for i, s := range r.MintSLDs {
		mints[i] = data.NewByteString(s)
	}
	burns := make([]data.PlutusData, len(r.BurnSLDs))
	for i, s := range r.BurnSLDs {
		burns[i] = data.NewByteString(s)
	}
	return data.NewConstr(ConstrMintSld,
		data.NewByteString(r.TLD),
		data.NewList(mints...),
		data.NewList(burns...),
	), nil
}

// EmptyDataRedeemer is the generic SLD spend redeemer used by contract tests (empty bytes).
func EmptyDataRedeemer() data.PlutusData {
	return data.NewByteString(nil)
}

// RegistrarTokenMintRedeemer is registrar_token's mint redeemer: the raw
// AssetName being minted or burned, no constructor wrapper.
func RegistrarTokenMintRedeemer(assetName []byte) data.PlutusData {
	return data.NewByteString(assetName)
}

func optionInt(v *int64) data.PlutusData {
	// Aiken Option: None = Constr 1 [], Some(x) = Constr 0 [x]
	if v == nil {
		return data.NewConstr(1)
	}
	return data.NewConstr(0, data.NewInteger(big.NewInt(*v)))
}

// MustPlutus panics on encode error — for tests only. Prefer ToPlutusData in production.
func MustPlutus(pd data.PlutusData, err error) data.PlutusData {
	if err != nil {
		panic(err)
	}
	return pd
}

// HexPolicyID decodes a 28-byte policy id hex string.
func HexPolicyID(s string) ([]byte, error) {
	if len(s) != 56 {
		return nil, fmt.Errorf("policy id must be 56 hex chars")
	}
	b := make([]byte, 28)
	for i := 0; i < 28; i++ {
		var v byte
		_, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &v)
		if err != nil {
			return nil, err
		}
		b[i] = v
	}
	return b, nil
}
