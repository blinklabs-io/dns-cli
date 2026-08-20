package txbuilder

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// ActivateTLD builds an unsigned TLD activation (OwnerAction + InitRemoveReference) transaction.
// The owner's signature can't be pre-generated: it's bound to the exact
// registration UTxO being spent, which is only known once located on-chain
// here, so ownerKey signs just-in-time instead of proof carrying a static
// signature.
func ActivateTLD(ctx context.Context, bctx *Context, tld domain.Label, ownerKey *secp256k1.PrivateKey, outPrefix, contractRevision string) (BuildOutput, error) {
	slog.Info("Building activate-tld transaction", "tld", tld.Canonical)
	ownerPub := ownerKey.PubKey().SerializeCompressed()
	// Register may be confirmed via tx APIs before the registrar address index catches up.
	regAsset := chainquery.AssetID{
		PolicyID: bctx.Contracts.RegistrarPolicyID,
		Name:     bctx.Contracts.TLDReferencePolicyID,
	}
	if err := chainquery.WaitByAsset(ctx, bctx.Provider, bctx.Contracts.RegistrarAddr, regAsset, chainquery.WaitByAssetOpts{}); err != nil {
		return BuildOutput{}, fmt.Errorf("registration not found: %w", err)
	}
	reg, err := chainquery.FindRegistration(ctx, bctx.Provider, bctx.Contracts, tld)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("registration not found: %w", err)
	}
	if reg.Datum.Minted != 0 {
		return BuildOutput{}, fmt.Errorf("tld %q is already activated (minted=%d)", tld.Canonical, reg.Datum.Minted)
	}
	if string(reg.Datum.OwnerHNSKey) != string(ownerPub) {
		return BuildOutput{}, fmt.Errorf("owner key does not match registration datum")
	}

	tldOwner, err := actorAddress(bctx.Eff, "tldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	tldOwnerPKH, err := loadActorKeyHash(bctx.Eff, "tldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, tldOwner, chainquery.MinActorFundingLovelace); err != nil {
		return BuildOutput{}, fmt.Errorf("tldOwner funding: %w", err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, tldOwner)
	if err != nil {
		return BuildOutput{}, err
	}
	collateral, err := chainquery.FindCollateral(ctx, bctx.Provider, tldOwner, chainquery.MinCollateralLovelace)
	if err != nil {
		return BuildOutput{}, err
	}

	// Signed message must match tld_registrar's OwnerAction check exactly:
	// tld ++ serialise_data(receiver_address) ++ serialise_data(output_reference).
	receiverPD := tldOwner.ToPlutusData()
	if receiverPD == nil {
		return BuildOutput{}, fmt.Errorf("tldOwner address has no Plutus Data representation")
	}
	receiverCBOR, err := data.Encode(receiverPD)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("encode receiver address: %w", err)
	}
	outputRefPD, err := protocol.OutputReferencePlutusData(reg.UTxO.Utxo.Id.Id().Bytes(), uint32(reg.UTxO.Utxo.Id.Index()))
	if err != nil {
		return BuildOutput{}, fmt.Errorf("encode output reference: %w", err)
	}
	outputRefCBOR, err := data.Encode(outputRefPD)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("encode output reference: %w", err)
	}
	message := append(append(append([]byte{}, tld.Bytes...), receiverCBOR...), outputRefCBOR...)
	ownerSig, err := domain.SignMessage(ownerKey, message)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("sign owner action: %w", err)
	}

	spendRed, err := protocol.OwnerActionRedeemer{
		OwnerSignature:  ownerSig,
		ReceiverAddress: tldOwner,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	updatedDatumPD, err := protocol.TLDRegisterDatum{
		TLD:         tld.Bytes,
		OwnerHNSKey: reg.Datum.OwnerHNSKey,
		Minted:      1,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	updatedDatum := plutusDatum(updatedDatumPD)

	sldPolicy, err := policyBytes(bctx.Contracts.SLDReferencePolicyID)
	if err != nil {
		return BuildOutput{}, err
	}
	tldRefDatumPD, err := protocol.TLDReferenceDatum{
		TLD:                  tld.Bytes,
		SLDs:                 nil,
		SLDReferencePolicyID: sldPolicy,
		Next:                 nil,
		Records:              nil,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	tldRefDatum := plutusDatum(tldRefDatumPD)

	userTN := protocol.CreateUserTokenTN(tld.Bytes)
	refTN := protocol.CreateReferenceTokenTN(tld.Bytes)
	userUnit := unit(bctx.Contracts.TLDReferencePolicyID, userTN, 1)
	refUnit := unit(bctx.Contracts.TLDReferencePolicyID, refTN, 1)
	regUnit := unit(bctx.Contracts.RegistrarPolicyID, bctx.Contracts.TLDReferencePolicyID, 1)

	a := bctx.newApollo(tldOwner)
	a.AddLoadedUTxOs(funding...)
	a.AddCollateral(collateral)
	a.CollectFrom(reg.UTxO.Utxo, plutusRedeemer(spendRed), common.ExUnits{})
	if err := bctx.addReferenceInput(a, "tldRegistrar"); err != nil {
		return BuildOutput{}, err
	}
	if err := bctx.addReferenceInput(a, "tldReference"); err != nil {
		return BuildOutput{}, err
	}
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(tldOwnerPKH)
	if err != nil {
		return BuildOutput{}, err
	}
	a.AddRequiredSigner(pkh)

	mintRed := plutusRedeemer(protocol.InitRemoveReferenceRedeemer())
	a.Mint(userUnit, &mintRed, nil)
	a.Mint(refUnit, &mintRed, nil)
	a.PayToContract(bctx.Contracts.RegistrarAddr, updatedDatum, 0, regUnit)
	a.PayToContract(bctx.Contracts.TLDReferenceAddr, tldRefDatum, 0, refUnit)
	a.PayToAddress(tldOwner, 0, userUnit)

	return bctx.finalize(a, "activate-tld", "unsigned activate-tld", outPrefix, contractRevision,
		[]string{tldOwnerPKH},
		[]artifact.ExpectedOutput{
			{Role: "registrar-updated", Index: 0},
			{Role: "tld-reference", Index: 1},
			{Role: "tld-user-token", Index: 2},
		},
		map[string]string{"tld": tld.Canonical},
	)
}
