package protocol

import (
	"fmt"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// PreprodNetworkID is Cardano testnet network id 0 (preprod/preview).
const PreprodNetworkID uint8 = common.AddressNetworkTestnet

// MainnetNetworkID is Cardano mainnet network id 1.
const MainnetNetworkID uint8 = common.AddressNetworkMainnet

// ScriptBaseAddress builds a script+key base address (payment script + stake key).
func ScriptBaseAddress(networkID uint8, scriptHash, stakeKeyHash []byte) (common.Address, error) {
	if networkID != common.AddressNetworkTestnet && networkID != common.AddressNetworkMainnet {
		return common.Address{}, fmt.Errorf("unsupported network id %d", networkID)
	}
	if len(scriptHash) != 28 {
		return common.Address{}, fmt.Errorf("script hash must be 28 bytes, got %d", len(scriptHash))
	}
	if len(stakeKeyHash) != 28 {
		return common.Address{}, fmt.Errorf("stake key hash must be 28 bytes, got %d", len(stakeKeyHash))
	}
	return common.NewAddressFromParts(
		common.AddressTypeScriptKey,
		networkID,
		scriptHash,
		stakeKeyHash,
	)
}

// ScriptBaseAddressHex is ScriptBaseAddress with hex-encoded hashes.
func ScriptBaseAddressHex(networkID uint8, scriptHashHex, stakeKeyHashHex string) (common.Address, error) {
	scriptHash, err := HexPolicyID(scriptHashHex)
	if err != nil {
		return common.Address{}, fmt.Errorf("script hash: %w", err)
	}
	stakeHash, err := HexPolicyID(stakeKeyHashHex)
	if err != nil {
		return common.Address{}, fmt.Errorf("stake key hash: %w", err)
	}
	return ScriptBaseAddress(networkID, scriptHash, stakeHash)
}
