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
)

// ActivateTLD builds an unsigned TLD activation (OwnerAction + InitRemoveReference) transaction.
func ActivateTLD(ctx context.Context, bctx *Context, tld domain.Label, proof domain.ParsedProof, outPrefix, contractRevision string) (BuildOutput, error) {
	slog.Info("Building activate-tld transaction", "tld", tld.Canonical)
	if err := domain.ValidateProofForActivation(proof, tld.Bytes); err != nil {
		return BuildOutput{}, fmt.Errorf("invalid owner proof: %w", err)
	}
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
	if string(reg.Datum.OwnerHNSKey) != string(proof.OwnerPublicKey) {
		return BuildOutput{}, fmt.Errorf("proof owner key does not match registration datum")
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

	spendRed, err := protocol.OwnerActionRedeemer{OwnerSignature: proof.OwnerSignature}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	updatedDatumPD, err := protocol.TLDRegisterDatum{
		TLD:         tld.Bytes,
		OwnerHNSKey: proof.OwnerPublicKey,
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
