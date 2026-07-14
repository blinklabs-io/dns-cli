package protocol

import (
	"fmt"
	"math/big"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
)

// DecodeTLDRegisterDatum parses inline registrar datum bytes.
func DecodeTLDRegisterDatum(d *common.Datum) (TLDRegisterDatum, error) {
	if d == nil {
		return TLDRegisterDatum{}, fmt.Errorf("nil datum")
	}
	c, err := expectConstr(d.Data, ConstrTLDRegisterDatum, 3)
	if err != nil {
		return TLDRegisterDatum{}, err
	}
	tld, err := expectBytes(c.Fields[0])
	if err != nil {
		return TLDRegisterDatum{}, fmt.Errorf("tld: %w", err)
	}
	owner, err := expectBytes(c.Fields[1])
	if err != nil {
		return TLDRegisterDatum{}, fmt.Errorf("owner_hns_key: %w", err)
	}
	minted, err := expectInt(c.Fields[2])
	if err != nil {
		return TLDRegisterDatum{}, fmt.Errorf("minted: %w", err)
	}
	return TLDRegisterDatum{TLD: tld, OwnerHNSKey: owner, Minted: minted}, nil
}

// DecodeTLDReferenceDatum parses a TLD reference inline datum.
func DecodeTLDReferenceDatum(d *common.Datum) (TLDReferenceDatum, error) {
	if d == nil {
		return TLDReferenceDatum{}, fmt.Errorf("nil datum")
	}
	c, err := expectConstr(d.Data, ConstrTLDReferenceDatum, 5)
	if err != nil {
		return TLDReferenceDatum{}, err
	}
	tld, err := expectBytes(c.Fields[0])
	if err != nil {
		return TLDReferenceDatum{}, fmt.Errorf("tld: %w", err)
	}
	slds, err := expectBytesList(c.Fields[1])
	if err != nil {
		return TLDReferenceDatum{}, fmt.Errorf("slds: %w", err)
	}
	policy, err := expectBytes(c.Fields[2])
	if err != nil {
		return TLDReferenceDatum{}, fmt.Errorf("sld_reference_policy_id: %w", err)
	}
	next, err := expectBytes(c.Fields[3])
	if err != nil {
		return TLDReferenceDatum{}, fmt.Errorf("next: %w", err)
	}
	records, err := decodeDNSRecords(c.Fields[4])
	if err != nil {
		return TLDReferenceDatum{}, fmt.Errorf("records: %w", err)
	}
	return TLDReferenceDatum{
		TLD:                  tld,
		SLDs:                 slds,
		SLDReferencePolicyID: policy,
		Next:                 next,
		Records:              records,
	}, nil
}

// DecodeSLDReferenceDatum parses an SLD reference inline datum.
func DecodeSLDReferenceDatum(d *common.Datum) (SLDReferenceDatum, error) {
	if d == nil {
		return SLDReferenceDatum{}, fmt.Errorf("nil datum")
	}
	c, err := expectConstr(d.Data, ConstrSLDReferenceDatum, 3)
	if err != nil {
		return SLDReferenceDatum{}, err
	}
	tld, err := expectBytes(c.Fields[0])
	if err != nil {
		return SLDReferenceDatum{}, fmt.Errorf("tld: %w", err)
	}
	sld, err := expectBytes(c.Fields[1])
	if err != nil {
		return SLDReferenceDatum{}, fmt.Errorf("sld: %w", err)
	}
	records, err := decodeDNSRecords(c.Fields[2])
	if err != nil {
		return SLDReferenceDatum{}, fmt.Errorf("records: %w", err)
	}
	return SLDReferenceDatum{TLD: tld, SLD: sld, Records: records}, nil
}

func decodeDNSRecords(pd data.PlutusData) ([]DNSRecord, error) {
	l, ok := pd.(*data.List)
	if !ok {
		return nil, fmt.Errorf("expected list, got %T", pd)
	}
	out := make([]DNSRecord, 0, len(l.Items))
	for i, item := range l.Items {
		rec, err := decodeDNSRecord(item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

func decodeDNSRecord(pd data.PlutusData) (DNSRecord, error) {
	c, err := expectConstr(pd, ConstrDNSRecord, 5)
	if err != nil {
		return DNSRecord{}, err
	}
	lhs, err := expectBytes(c.Fields[0])
	if err != nil {
		return DNSRecord{}, fmt.Errorf("lhs: %w", err)
	}
	ttl, err := decodeOptionInt(c.Fields[1])
	if err != nil {
		return DNSRecord{}, fmt.Errorf("ttl: %w", err)
	}
	class, err := expectBytes(c.Fields[2])
	if err != nil {
		return DNSRecord{}, fmt.Errorf("class: %w", err)
	}
	rtype, err := expectBytes(c.Fields[3])
	if err != nil {
		return DNSRecord{}, fmt.Errorf("rtype: %w", err)
	}
	rdata, err := expectBytes(c.Fields[4])
	if err != nil {
		return DNSRecord{}, fmt.Errorf("rdata: %w", err)
	}
	return DNSRecord{LHS: lhs, TTL: ttl, Class: class, RType: rtype, RData: rdata}, nil
}

func decodeOptionInt(pd data.PlutusData) (*int64, error) {
	c, ok := pd.(*data.Constr)
	if !ok {
		return nil, fmt.Errorf("expected constr option, got %T", pd)
	}
	switch c.Tag {
	case 1:
		if len(c.Fields) != 0 {
			return nil, fmt.Errorf("None must have no fields")
		}
		return nil, nil
	case 0:
		if len(c.Fields) != 1 {
			return nil, fmt.Errorf("Some must have one field")
		}
		v, err := expectInt(c.Fields[0])
		if err != nil {
			return nil, err
		}
		return &v, nil
	default:
		return nil, fmt.Errorf("invalid option constructor %d", c.Tag)
	}
}

func expectConstr(pd data.PlutusData, tag uint, fields int) (*data.Constr, error) {
	c, ok := pd.(*data.Constr)
	if !ok {
		return nil, fmt.Errorf("expected constr, got %T", pd)
	}
	if c.Tag != tag {
		return nil, fmt.Errorf("expected constructor %d, got %d", tag, c.Tag)
	}
	if len(c.Fields) != fields {
		return nil, fmt.Errorf("expected %d fields, got %d", fields, len(c.Fields))
	}
	return c, nil
}

func expectBytes(pd data.PlutusData) ([]byte, error) {
	b, ok := pd.(*data.ByteString)
	if !ok {
		return nil, fmt.Errorf("expected bytestring, got %T", pd)
	}
	return append([]byte(nil), b.Inner...), nil
}

func expectInt(pd data.PlutusData) (int64, error) {
	i, ok := pd.(*data.Integer)
	if !ok {
		return 0, fmt.Errorf("expected integer, got %T", pd)
	}
	if i.Inner == nil || !i.Inner.IsInt64() {
		return 0, fmt.Errorf("integer out of int64 range")
	}
	return i.Inner.Int64(), nil
}

func expectBytesList(pd data.PlutusData) ([][]byte, error) {
	l, ok := pd.(*data.List)
	if !ok {
		return nil, fmt.Errorf("expected list, got %T", pd)
	}
	out := make([][]byte, 0, len(l.Items))
	for i, item := range l.Items {
		b, err := expectBytes(item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, b)
	}
	return out, nil
}

// InsertSortedSLD inserts canonical SLD bytes preserving strict lexicographic order.
func InsertSortedSLD(existing [][]byte, sld []byte) ([][]byte, error) {
	for _, e := range existing {
		if bytesEqual(e, sld) {
			return nil, fmt.Errorf("duplicate sld")
		}
	}
	out := append([][]byte{}, existing...)
	out = append(out, append([]byte(nil), sld...))
	sortBytesSlice(out)
	return out, nil
}

func sortBytesSlice(items [][]byte) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && bytesLess(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func bytesLess(a, b []byte) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// BytesToBigInt is a helper for tests.
func BytesToBigInt(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}
