package txbuilder

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
)

// RegisterTLD builds an unsigned registrar registration transaction.
func RegisterTLD(ctx context.Context, bctx *Context, tld domain.Label, proof domain.ParsedProof, outPrefix, contractRevision string) (BuildOutput, error) {
	slog.Info("Building register-tld transaction", "tld", tld.Canonical)
	if err := domain.ValidateProofForRegistration(proof, tld.Bytes); err != nil {
		return BuildOutput{}, fmt.Errorf("invalid registrar proof: %w", err)
	}
	if _, err := chainquery.FindRegistration(ctx, bctx.Provider, bctx.Contracts, tld); err == nil {
		return BuildOutput{}, fmt.Errorf("tld %q is already registered", tld.Canonical)
	}
	registrar, err := actorAddress(bctx.Eff, "registrar")
	if err != nil {
		return BuildOutput{}, err
	}
	registrarPKH, err := loadActorKeyHash(bctx.Eff, "registrar")
	if err != nil {
		return BuildOutput{}, err
	}
	// Fund tx confirmation can race registrar address indexing (empty/404 briefly).
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, registrar, chainquery.MinActorFundingLovelace); err != nil {
		return BuildOutput{}, fmt.Errorf("registrar funding: %w", err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, registrar)
	if err != nil {
		return BuildOutput{}, err
	}
	collateral, err := chainquery.FindCollateral(ctx, bctx.Provider, registrar, chainquery.MinCollateralLovelace)
	if err != nil {
		return BuildOutput{}, err
	}

	tldRefPolicy, err := policyBytes(bctx.Contracts.TLDReferencePolicyID)
	if err != nil {
		return BuildOutput{}, err
	}
	mintRed, err := protocol.RegisterTLDRedeemer{
		TLD:                  tld.Bytes,
		Owner:                proof.OwnerPublicKey,
		TLDReferencePolicyID: tldRefPolicy,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	datumPD, err := protocol.TLDRegisterDatum{
		TLD:         tld.Bytes,
		OwnerHNSKey: proof.OwnerPublicKey,
		Minted:      0,
	}.ToPlutusData()
	if err != nil {
		return BuildOutput{}, err
	}
	datum := plutusDatum(datumPD)

	a := bctx.newApollo(registrar)
	a.AddLoadedUTxOs(funding...)
	a.AddCollateral(collateral)
	if err := bctx.addReferenceInput(a, "tldRegistrar"); err != nil {
		return BuildOutput{}, err
	}
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(registrarPKH)
	if err != nil {
		return BuildOutput{}, err
	}
	a.AddRequiredSigner(pkh)

	regAsset := unit(bctx.Contracts.RegistrarPolicyID, bctx.Contracts.TLDReferencePolicyID, 1)
	rd := plutusRedeemer(mintRed)
	a.Mint(regAsset, &rd, nil)
	a.PayToContract(bctx.Contracts.RegistrarAddr, datum, 0, regAsset)

	return bctx.finalize(a, "register-tld", "unsigned register-tld", outPrefix, contractRevision,
		[]string{registrarPKH},
		[]artifact.ExpectedOutput{{Role: "registrar-registration", Index: 0}},
		map[string]string{"tld": tld.Canonical},
	)
}
