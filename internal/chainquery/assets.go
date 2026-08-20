// Package chainquery discovers on-chain DNS contract state.
package chainquery

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// AssetID identifies a native asset by policy and asset name hex.
type AssetID struct {
	PolicyID string
	Name     string
}

// UTxORef is a located UTxO with decoded metadata.
type UTxORef struct {
	Utxo common.Utxo
	Ref  string
}

// FindByAsset scans address UTxOs for exactly one match of policy+name.
func FindByAsset(ctx context.Context, p provider.Provider, addr common.Address, asset AssetID) (UTxORef, error) {
	_ = ctx
	utxos, err := p.Utxos(addr)
	if err != nil {
		return UTxORef{}, fmt.Errorf("query utxos: %w", err)
	}
	policy, name, err := decodeAsset(asset)
	if err != nil {
		return UTxORef{}, err
	}
	var matches []UTxORef
	for _, u := range utxos {
		if hasAsset(u, policy, name) {
			ref := fmt.Sprintf("%s#%d", hex.EncodeToString(u.Id.Id().Bytes()), u.Id.Index())
			matches = append(matches, UTxORef{Utxo: u, Ref: ref})
		}
	}
	switch len(matches) {
	case 0:
		return UTxORef{}, fmt.Errorf("no utxo with asset %s.%s at %s", asset.PolicyID, asset.Name, addr.String())
	case 1:
		slog.Debug("Found asset UTxO", "ref", matches[0].Ref, "policy", asset.PolicyID, "asset", logging.HexPrefix(asset.Name, 4))
		return matches[0], nil
	default:
		slog.Warn("Duplicate asset UTxOs", "count", len(matches), "policy", asset.PolicyID)
		return UTxORef{}, fmt.Errorf("duplicate utxos (%d) with asset %s.%s", len(matches), asset.PolicyID, asset.Name)
	}
}

// FindCollateral selects an ADA-only UTxO suitable for script collateral.
func FindCollateral(ctx context.Context, p provider.Provider, addr common.Address, minLovelace int64) (common.Utxo, error) {
	_ = ctx
	utxos, err := p.Utxos(addr)
	if err != nil {
		return common.Utxo{}, fmt.Errorf("query utxos: %w", err)
	}
	var best *common.Utxo
	var bestAda int64
	for i := range utxos {
		u := utxos[i]
		if u.Output.Assets() != nil {
			continue
		}
		if u.Output.ScriptRef() != nil {
			continue
		}
		ada := u.Output.Amount().Int64()
		if ada < minLovelace {
			continue
		}
		if best == nil || ada > bestAda {
			copyU := u
			best = &copyU
			bestAda = ada
		}
	}
	if best == nil {
		return common.Utxo{}, fmt.Errorf("no suitable collateral utxo at %s (need >= %d lovelace, ADA-only)", addr.String(), minLovelace)
	}
	ref := fmt.Sprintf("%s#%d", hex.EncodeToString(best.Id.Id().Bytes()), best.Id.Index())
	slog.Debug("Selected collateral UTxO", "ref", ref, "lovelace", bestAda)
	return *best, nil
}

// LoadFundingUTxOs returns spendable UTxOs for an actor address.
// Unused addresses (Blockfrost 404) are normalized by the provider wrapper to
// an empty slice; other provider failures are returned with clearer context.
func LoadFundingUTxOs(ctx context.Context, p provider.Provider, addr common.Address) ([]common.Utxo, error) {
	_ = ctx
	utxos, err := p.Utxos(addr)
	if err != nil {
		return nil, fmt.Errorf("query utxos at %s: %w", addr.String(), err)
	}
	return utxos, nil
}

// SumLovelace returns the total lovelace at an address across all UTxOs.
// An unused address with no chain history is reported as 0 lovelace / 0 UTxOs.
func SumLovelace(ctx context.Context, p provider.Provider, addr common.Address) (int64, int, error) {
	utxos, err := LoadFundingUTxOs(ctx, p, addr)
	if err != nil {
		return 0, 0, err
	}
	var total int64
	for _, u := range utxos {
		total += u.Output.Amount().Int64()
	}
	return total, len(utxos), nil
}

func hasAsset(u common.Utxo, policy []byte, name []byte) bool {
	assets := u.Output.Assets()
	if assets == nil {
		return false
	}
	var pid common.Blake2b224
	copy(pid[:], policy)
	qty := assets.Asset(pid, name)
	return qty != nil && qty.Sign() > 0
}

func decodeAsset(asset AssetID) (policy []byte, name []byte, err error) {
	policy, err = hex.DecodeString(strings.ToLower(asset.PolicyID))
	if err != nil || len(policy) != 28 {
		return nil, nil, fmt.Errorf("invalid policy id %q", asset.PolicyID)
	}
	name, err = hex.DecodeString(strings.ToLower(asset.Name))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid asset name %q", asset.Name)
	}
	return policy, name, nil
}

// Contracts bundles configured deployment identifiers.
type Contracts struct {
	RegistrarAddr           common.Address
	TLDReferenceAddr        common.Address
	SLDReferenceAddr        common.Address
	RegistrarPolicyID       string
	TLDReferencePolicyID    string
	SLDReferencePolicyID    string
	RegistrarTokenPolicyID  string
	RegistrarTokenAssetName string
}

// LoadContracts parses contract addresses from config.
func LoadContracts(eff *config.Effective) (Contracts, error) {
	c := eff.Profile.Contracts
	registrar, err := common.NewAddress(c.TLDRegistrarAddress)
	if err != nil {
		return Contracts{}, fmt.Errorf("tldRegistrarAddress: %w", err)
	}
	tldRef, err := common.NewAddress(c.TLDReferenceAddress)
	if err != nil {
		return Contracts{}, fmt.Errorf("tldReferenceAddress: %w", err)
	}
	sldRef, err := common.NewAddress(c.SLDReferenceAddress)
	if err != nil {
		return Contracts{}, fmt.Errorf("sldReferenceAddress: %w", err)
	}
	return Contracts{
		RegistrarAddr:           registrar,
		TLDReferenceAddr:        tldRef,
		SLDReferenceAddr:        sldRef,
		RegistrarPolicyID:       strings.ToLower(c.TLDRegistrarPolicyID),
		TLDReferencePolicyID:    strings.ToLower(c.TLDReferencePolicyID),
		SLDReferencePolicyID:    strings.ToLower(c.SLDReferencePolicyID),
		RegistrarTokenPolicyID:  strings.ToLower(c.RegistrarTokenPolicyID),
		RegistrarTokenAssetName: strings.ToLower(c.RegistrarTokenAssetName),
	}, nil
}

// RegistrationState is a located registrar registration UTxO.
type RegistrationState struct {
	UTxO  UTxORef
	Datum protocol.TLDRegisterDatum
}

// FindRegistration locates the registrar NFT for a TLD.
func FindRegistration(ctx context.Context, p provider.Provider, contracts Contracts, tld domain.Label) (RegistrationState, error) {
	asset := AssetID{
		PolicyID: contracts.RegistrarPolicyID,
		Name:     contracts.TLDReferencePolicyID,
	}
	u, err := FindByAsset(ctx, p, contracts.RegistrarAddr, asset)
	if err != nil {
		return RegistrationState{}, err
	}
	d := u.Utxo.Output.Datum()
	if d == nil {
		return RegistrationState{}, fmt.Errorf("registration utxo missing inline datum")
	}
	datum, err := protocol.DecodeTLDRegisterDatum(d)
	if err != nil {
		return RegistrationState{}, fmt.Errorf("decode registration datum: %w", err)
	}
	if string(datum.TLD) != tld.Canonical {
		return RegistrationState{}, fmt.Errorf("registration datum tld %q does not match %q", string(datum.TLD), tld.Canonical)
	}
	return RegistrationState{UTxO: u, Datum: datum}, nil
}

// TLDNodeState is a located TLD reference node.
type TLDNodeState struct {
	UTxO  UTxORef
	Datum protocol.TLDReferenceDatum
}

// FindTLDNode locates the TLD reference UTxO for a canonical TLD label.
func FindTLDNode(ctx context.Context, p provider.Provider, contracts Contracts, tld domain.Label) (TLDNodeState, error) {
	refTN := protocol.CreateReferenceTokenTN(tld.Bytes)
	asset := AssetID{PolicyID: contracts.TLDReferencePolicyID, Name: refTN}
	u, err := FindByAsset(ctx, p, contracts.TLDReferenceAddr, asset)
	if err != nil {
		return TLDNodeState{}, err
	}
	d := u.Utxo.Output.Datum()
	if d == nil {
		return TLDNodeState{}, fmt.Errorf("tld reference utxo missing inline datum")
	}
	datum, err := protocol.DecodeTLDReferenceDatum(d)
	if err != nil {
		return TLDNodeState{}, fmt.Errorf("decode tld reference datum: %w", err)
	}
	if string(datum.TLD) != tld.Canonical {
		return TLDNodeState{}, fmt.Errorf("tld reference datum tld %q does not match %q", string(datum.TLD), tld.Canonical)
	}
	if len(datum.Next) > 0 {
		return TLDNodeState{}, fmt.Errorf("linked-list continuation not supported in this release (non-empty next field)")
	}
	return TLDNodeState{UTxO: u, Datum: datum}, nil
}

// SLDNodeState is a located SLD reference UTxO.
type SLDNodeState struct {
	UTxO  UTxORef
	Datum protocol.SLDReferenceDatum
}

// FindSLDNode locates the SLD reference UTxO for a TLD/SLD pair.
func FindSLDNode(ctx context.Context, p provider.Provider, contracts Contracts, tld, sld domain.Label) (SLDNodeState, error) {
	refTN := protocol.CreateReferenceTokenTN(sld.Bytes)
	asset := AssetID{PolicyID: contracts.SLDReferencePolicyID, Name: refTN}
	u, err := FindByAsset(ctx, p, contracts.SLDReferenceAddr, asset)
	if err != nil {
		return SLDNodeState{}, err
	}
	d := u.Utxo.Output.Datum()
	if d == nil {
		return SLDNodeState{}, fmt.Errorf("sld reference utxo missing inline datum")
	}
	datum, err := protocol.DecodeSLDReferenceDatum(d)
	if err != nil {
		return SLDNodeState{}, fmt.Errorf("decode sld reference datum: %w", err)
	}
	if string(datum.TLD) != tld.Canonical || string(datum.SLD) != sld.Canonical {
		return SLDNodeState{}, fmt.Errorf("sld reference identity mismatch")
	}
	return SLDNodeState{UTxO: u, Datum: datum}, nil
}

// FindUserToken locates a user token UTxO at an owner address.
func FindUserToken(ctx context.Context, p provider.Provider, owner common.Address, policyID string, label domain.Label) (UTxORef, error) {
	userTN := protocol.CreateUserTokenTN(label.Bytes)
	return FindByAsset(ctx, p, owner, AssetID{PolicyID: policyID, Name: userTN})
}

// ContainsSLD reports whether sorted SLD bytes already include label bytes.
func ContainsSLD(slds [][]byte, sld []byte) bool {
	for _, existing := range slds {
		if bytesEqual(existing, sld) {
			return true
		}
	}
	return false
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

// MinCollateralLovelace is a conservative default before protocol params are applied.
const MinCollateralLovelace = 5_000_000

// BigIntLovelace converts big.Int to int64 with bounds check.
func BigIntLovelace(v *big.Int) (int64, error) {
	if v == nil || !v.IsInt64() {
		return 0, fmt.Errorf("lovelace amount out of range")
	}
	return v.Int64(), nil
}
