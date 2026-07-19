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

// UpdateSLD builds an unsigned SLD DNS record replacement transaction.
func UpdateSLD(ctx context.Context, bctx *Context, tld, sld domain.Label, records []domain.ParsedRecord, outPrefix, contractRevision string) (BuildOutput, error) {
	slog.Info("Building update-sld transaction", "tld", tld.Canonical, "sld", sld.Canonical, "records", len(records))
	sldOwner, err := actorAddress(bctx.Eff, "sldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	refTN := protocol.CreateReferenceTokenTN(sld.Bytes)
	userTN := protocol.CreateUserTokenTN(sld.Bytes)
	if err := chainquery.WaitByAsset(ctx, bctx.Provider, bctx.Contracts.SLDReferenceAddr, chainquery.AssetID{
		PolicyID: bctx.Contracts.SLDReferencePolicyID,
		Name:     refTN,
	}, chainquery.WaitByAssetOpts{}); err != nil {
		return BuildOutput{}, err
	}
	if err := chainquery.WaitByAsset(ctx, bctx.Provider, sldOwner, chainquery.AssetID{
		PolicyID: bctx.Contracts.SLDReferencePolicyID,
		Name:     userTN,
	}, chainquery.WaitByAssetOpts{}); err != nil {
		return BuildOutput{}, fmt.Errorf("sld user token: %w", err)
	}
	node, err := chainquery.FindSLDNode(ctx, bctx.Provider, bctx.Contracts, tld, sld)
	if err != nil {
		return BuildOutput{}, err
	}

	sldOwnerPKH, err := loadActorKeyHash(bctx.Eff, "sldOwner")
	if err != nil {
		return BuildOutput{}, err
	}
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, sldOwner, chainquery.MinActorFundingLovelace); err != nil {
		return BuildOutput{}, fmt.Errorf("sldOwner funding: %w", err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, sldOwner)
	if err != nil {
		return BuildOutput{}, err
	}
	collateral, err := chainquery.FindCollateral(ctx, bctx.Provider, sldOwner, chainquery.MinCollateralLovelace)
	if err != nil {
		return BuildOutput{}, err
	}
	sldUser, err := chainquery.FindUserToken(ctx, bctx.Provider, sldOwner, bctx.Contracts.SLDReferencePolicyID, sld)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("sld user token: %w", err)
	}

	protoRecords := make([]protocol.DNSRecord, len(records))
	for i, r := range records {
		protoRecords[i] = protocol.FromParsed(r)
	}
	updatedDatumPD, err := protocol.SLDReferenceDatum{
		TLD:     tld.Bytes,
		SLD:     sld.Bytes,
		Records: protoRecords,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	updatedDatum := plutusDatum(updatedDatumPD)

	sldRefTN := protocol.CreateReferenceTokenTN(sld.Bytes)
	sldUserTN := protocol.CreateUserTokenTN(sld.Bytes)
	refUnit := unit(bctx.Contracts.SLDReferencePolicyID, sldRefTN, 1)
	userUnit := unit(bctx.Contracts.SLDReferencePolicyID, sldUserTN, 1)

	spendRed := plutusRedeemer(protocol.EmptyDataRedeemer())

	a := bctx.newApollo(sldOwner)
	a.AddLoadedUTxOs(funding...)
	a.AddLoadedUTxOs(sldUser.Utxo)
	a.AddCollateral(collateral)
	a.CollectFrom(node.UTxO.Utxo, spendRed, common.ExUnits{})
	if err := bctx.addReferenceInput(a, "sldReference"); err != nil {
		return BuildOutput{}, err
	}
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(sldOwnerPKH)
	if err != nil {
		return BuildOutput{}, err
	}
	a.AddRequiredSigner(pkh)

	a.PayToContract(bctx.Contracts.SLDReferenceAddr, updatedDatum, 0, refUnit)
	a.PayToAddress(sldOwner, 0, userUnit)

	return bctx.finalize(a, "update-sld", "unsigned update-sld", outPrefix, contractRevision,
		[]string{sldOwnerPKH},
		[]artifact.ExpectedOutput{
			{Role: "sld-reference-updated", Index: 0},
			{Role: "sld-user-token-return", Index: 1},
		},
		map[string]string{
			"tld":            tld.Canonical,
			"sld":            sld.Canonical,
			"recordCount":    fmt.Sprintf("%d", len(records)),
			"oldRecordCount": fmt.Sprintf("%d", len(node.Datum.Records)),
		},
	)
}
