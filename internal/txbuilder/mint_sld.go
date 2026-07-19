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

// MintSLD builds an unsigned SLD mint transaction under an activated TLD.
func MintSLD(ctx context.Context, bctx *Context, tld, sld domain.Label, sldOwnerActor, outPrefix, contractRevision string) (BuildOutput, error) {
	slog.Info("Building mint-sld transaction", "tld", tld.Canonical, "sld", sld.Canonical)
	tldOwner, err := actorAddress(bctx.Eff, "tldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	refTN := protocol.CreateReferenceTokenTN(tld.Bytes)
	userTN := protocol.CreateUserTokenTN(tld.Bytes)
	if err := chainquery.WaitByAsset(ctx, bctx.Provider, bctx.Contracts.TLDReferenceAddr, chainquery.AssetID{
		PolicyID: bctx.Contracts.TLDReferencePolicyID,
		Name:     refTN,
	}, chainquery.WaitByAssetOpts{}); err != nil {
		return BuildOutput{}, fmt.Errorf("tld reference: %w", err)
	}
	if err := chainquery.WaitByAsset(ctx, bctx.Provider, tldOwner, chainquery.AssetID{
		PolicyID: bctx.Contracts.TLDReferencePolicyID,
		Name:     userTN,
	}, chainquery.WaitByAssetOpts{}); err != nil {
		return BuildOutput{}, fmt.Errorf("tld user token: %w", err)
	}
	node, err := chainquery.FindTLDNode(ctx, bctx.Provider, bctx.Contracts, tld)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("tld reference: %w", err)
	}
	if chainquery.ContainsSLD(node.Datum.SLDs, sld.Bytes) {
		return BuildOutput{}, fmt.Errorf("sld %q already exists under tld %q", sld.Canonical, tld.Canonical)
	}
	updatedSLDs, err := protocol.InsertSortedSLD(node.Datum.SLDs, sld.Bytes)
	if err != nil {
		return BuildOutput{}, err
	}

	tldOwnerPKH, err := loadActorKeyHash(bctx.Eff, "tldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	sldOwner, err := actorAddress(bctx.Eff, sldOwnerActor)
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

	tldUser, err := chainquery.FindUserToken(ctx, bctx.Provider, tldOwner, bctx.Contracts.TLDReferencePolicyID, tld)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("tld user token: %w", err)
	}

	updatedTLDDatumPD, err := protocol.TLDReferenceDatum{
		TLD:                  node.Datum.TLD,
		SLDs:                 updatedSLDs,
		SLDReferencePolicyID: node.Datum.SLDReferencePolicyID,
		Next:                 node.Datum.Next,
		Records:              node.Datum.Records,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	updatedTLDDatum := plutusDatum(updatedTLDDatumPD)

	sldDatumPD, err := protocol.SLDReferenceDatum{
		TLD:     tld.Bytes,
		SLD:     sld.Bytes,
		Records: nil,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	sldDatum := plutusDatum(sldDatumPD)

	tldRefTN := protocol.CreateReferenceTokenTN(tld.Bytes)
	tldUserTN := protocol.CreateUserTokenTN(tld.Bytes)
	sldRefTN := protocol.CreateReferenceTokenTN(sld.Bytes)
	sldUserTN := protocol.CreateUserTokenTN(sld.Bytes)

	tldRefUnit := unit(bctx.Contracts.TLDReferencePolicyID, tldRefTN, 1)
	tldUserUnit := unit(bctx.Contracts.TLDReferencePolicyID, tldUserTN, 1)
	sldRefUnit := unit(bctx.Contracts.SLDReferencePolicyID, sldRefTN, 1)
	sldUserUnit := unit(bctx.Contracts.SLDReferencePolicyID, sldUserTN, 1)

	spendRed := plutusRedeemer(protocol.SpendReferenceRedeemer())
	mintRedPD, err := protocol.MintSldRedeemer{
		TLD:      tld.Bytes,
		MintSLDs: [][]byte{sld.Bytes},
		BurnSLDs: nil,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	mintRed := plutusRedeemer(mintRedPD)

	a := bctx.newApollo(tldOwner)
	a.AddLoadedUTxOs(funding...)
	a.AddLoadedUTxOs(tldUser.Utxo)
	a.AddCollateral(collateral)
	a.CollectFrom(node.UTxO.Utxo, spendRed, common.ExUnits{})
	if err := bctx.addReferenceInput(a, "tldReference"); err != nil {
		return BuildOutput{}, err
	}
	if err := bctx.addReferenceInput(a, "sldReference"); err != nil {
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

	a.Mint(sldRefUnit, &mintRed, nil)
	a.Mint(sldUserUnit, &mintRed, nil)
	a.PayToContract(bctx.Contracts.TLDReferenceAddr, updatedTLDDatum, 0, tldRefUnit)
	a.PayToContract(bctx.Contracts.SLDReferenceAddr, sldDatum, 0, sldRefUnit)
	a.PayToAddress(sldOwner, 0, sldUserUnit)
	a.PayToAddress(tldOwner, 0, tldUserUnit)

	required := []string{tldOwnerPKH}
	return bctx.finalize(a, "mint-sld", "unsigned mint-sld", outPrefix, contractRevision,
		required,
		[]artifact.ExpectedOutput{
			{Role: "tld-reference-updated", Index: 0},
			{Role: "sld-reference", Index: 1},
			{Role: "sld-user-token", Index: 2},
			{Role: "tld-user-token-return", Index: 3},
		},
		map[string]string{"tld": tld.Canonical, "sld": sld.Canonical, "sldOwner": sldOwnerActor},
	)
}
