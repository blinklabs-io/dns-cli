package protocol

import (
	"encoding/hex"
	"fmt"

	"github.com/blinklabs-io/plutigo/data"
)

// Constructor indexes for Aiken cardano/address types.
const (
	ConstrStakeInline               = 0 // StakeCredential.Inline
	ConstrCredentialVerificationKey = 0 // Credential.VerificationKey
	ConstrCredentialScript          = 1 // Credential.Script
)

// EncodeByteArrayCBORHex encodes a Plutus bytes value as hex CBOR for aiken apply.
func EncodeByteArrayCBORHex(b []byte) (string, error) {
	return encodePlutusCBORHex(data.NewByteString(b))
}

// EncodePolicyIDCBORHex encodes a 28-byte PolicyId as Plutus bytes CBOR hex.
func EncodePolicyIDCBORHex(policyID []byte) (string, error) {
	if len(policyID) != 28 {
		return "", fmt.Errorf("policy id must be 28 bytes, got %d", len(policyID))
	}
	return EncodeByteArrayCBORHex(policyID)
}

// EncodeStakeKeyHashCredentialCBORHex encodes Inline(VerificationKey(hash)) StakeCredential.
// Matches Aiken cardano/address StakeCredential (Referenced Credential).
func EncodeStakeKeyHashCredentialCBORHex(stakeKeyHash []byte) (string, error) {
	if len(stakeKeyHash) != 28 {
		return "", fmt.Errorf("stake key hash must be 28 bytes, got %d", len(stakeKeyHash))
	}
	cred := data.NewConstr(ConstrCredentialVerificationKey, data.NewByteString(stakeKeyHash))
	stake := data.NewConstr(ConstrStakeInline, cred)
	return encodePlutusCBORHex(stake)
}

// StakeCredentialPlutusData builds Inline(VerificationKey(keyHash)).
func StakeCredentialPlutusData(stakeKeyHash []byte) (data.PlutusData, error) {
	if len(stakeKeyHash) != 28 {
		return nil, fmt.Errorf("stake key hash must be 28 bytes, got %d", len(stakeKeyHash))
	}
	cred := data.NewConstr(ConstrCredentialVerificationKey, data.NewByteString(stakeKeyHash))
	return data.NewConstr(ConstrStakeInline, cred), nil
}

func encodePlutusCBORHex(pd data.PlutusData) (string, error) {
	raw, err := data.Encode(pd)
	if err != nil {
		return "", fmt.Errorf("encode plutus data: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
