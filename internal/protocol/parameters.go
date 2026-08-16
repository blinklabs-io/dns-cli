package protocol

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/data"
)

// Constructor indexes for Aiken cardano/address types.
const (
	ConstrStakeInline               = 0 // StakeCredential.Inline
	ConstrCredentialVerificationKey = 0 // Credential.VerificationKey
	ConstrCredentialScript          = 1 // Credential.Script
)

// ConstrOutputReference is aiken/cardano/transaction.OutputReference's
// (only) constructor index.
const ConstrOutputReference = 0

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

// EncodeOutputRefCBORHex encodes OutputReference{transaction_id, output_index}
// as Plutus Data CBOR, for parameterizing one-shot minting policies.
// Matches aiken/cardano/transaction.OutputReference.
func EncodeOutputRefCBORHex(txHash []byte, index uint32) (string, error) {
	if len(txHash) != 32 {
		return "", fmt.Errorf("tx hash must be 32 bytes, got %d", len(txHash))
	}
	ref := data.NewConstr(ConstrOutputReference,
		data.NewByteString(txHash),
		data.NewInteger(new(big.Int).SetUint64(uint64(index))),
	)
	return encodePlutusCBORHex(ref)
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
