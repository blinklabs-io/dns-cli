package domain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ByteField encodes string data either as UTF-8 text or hex for contract ByteArray fields.
type ByteField struct {
	Encoding string `json:"encoding"` // "text" or "hex"
	Value    string `json:"value"`
}

// Bytes returns the on-chain bytes.
func (b ByteField) Bytes() ([]byte, error) {
	switch strings.ToLower(b.Encoding) {
	case "", "text":
		if b.Value == "" {
			return nil, fmt.Errorf("empty text value")
		}
		return []byte(b.Value), nil
	case "hex":
		raw, err := hex.DecodeString(strings.TrimPrefix(b.Value, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid hex: %w", err)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", b.Encoding)
	}
}

// RecordInput is the JSON representation of one DNSRecord.
type RecordInput struct {
	LHS   ByteField `json:"lhs"`
	TTL   *int64    `json:"ttl"`
	Class ByteField `json:"class"`
	RType ByteField `json:"rtype"`
	RData ByteField `json:"rdata"`
}

// RecordsFile is the complete replacement set for SLD records.
type RecordsFile struct {
	Records []RecordInput `json:"records"`
}

// ParsedRecord is a validated contract DNS record.
type ParsedRecord struct {
	LHS   []byte
	TTL   *int64
	Class []byte
	RType []byte
	RData []byte
}

// LoadRecordsFile loads and validates a records JSON file.
func LoadRecordsFile(path string, maxRecords int) ([]ParsedRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f RecordsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse records: %w", err)
	}
	if maxRecords > 0 && len(f.Records) > maxRecords {
		return nil, fmt.Errorf("too many records: %d (max %d)", len(f.Records), maxRecords)
	}
	out := make([]ParsedRecord, 0, len(f.Records))
	seen := map[string]struct{}{}
	for i, r := range f.Records {
		pr, err := parseRecord(r)
		if err != nil {
			return nil, fmt.Errorf("records[%d]: %w", i, err)
		}
		key := canonicalKey(pr)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("records[%d]: duplicate record", i)
		}
		seen[key] = struct{}{}
		if pr.TTL != nil && (*pr.TTL < 0 || *pr.TTL > 2147483647) {
			return nil, fmt.Errorf("records[%d]: ttl out of range", i)
		}
		out = append(out, pr)
	}
	return out, nil
}

func parseRecord(r RecordInput) (ParsedRecord, error) {
	lhs, err := r.LHS.Bytes()
	if err != nil {
		return ParsedRecord{}, fmt.Errorf("lhs: %w", err)
	}
	class, err := r.Class.Bytes()
	if err != nil {
		return ParsedRecord{}, fmt.Errorf("class: %w", err)
	}
	rtype, err := r.RType.Bytes()
	if err != nil {
		return ParsedRecord{}, fmt.Errorf("rtype: %w", err)
	}
	rdata, err := r.RData.Bytes()
	if err != nil {
		return ParsedRecord{}, fmt.Errorf("rdata: %w", err)
	}
	return ParsedRecord{LHS: lhs, TTL: r.TTL, Class: class, RType: rtype, RData: rdata}, nil
}

func canonicalKey(p ParsedRecord) string {
	ttl := "none"
	if p.TTL != nil {
		ttl = fmt.Sprintf("%d", *p.TTL)
	}
	return hex.EncodeToString(p.LHS) + "|" + ttl + "|" + hex.EncodeToString(p.Class) + "|" + hex.EncodeToString(p.RType) + "|" + hex.EncodeToString(p.RData)
}
