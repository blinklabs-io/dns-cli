package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func runWalletFund(ctx context.Context, eff *config.Effective, fromActor string, allocations []txbuilder.FundAllocation, collateralLovelace int64, out string) (string, error) {
	slog.Info("Starting wallet fund", "from", fromActor, "allocations", len(allocations), "collateral", collateralLovelace, "out", out)
	bctx, err := txbuilder.NewFundingContext(ctx, eff)
	if err != nil {
		return "", mapContextErr(err)
	}
	outBuild, err := txbuilder.FundActors(ctx, bctx, fromActor, allocations, collateralLovelace, out)
	if err != nil {
		slog.Error("wallet fund failed", "error", err)
		msg := err.Error()
		switch {
		case strings.Contains(msg, "preprod only"),
			strings.Contains(msg, "allocation"),
			strings.Contains(msg, "unknown"),
			strings.Contains(msg, "duplicate"),
			strings.Contains(msg, "collateral"),
			strings.Contains(msg, "insufficient"):
			return "", WrapExit(ExitValidation, err)
		default:
			return "", WrapExit(ExitBuild, err)
		}
	}
	slog.Info("Built wallet fund transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

func runWalletBalance(ctx context.Context, eff *config.Effective, actor string) (int64, int, error) {
	a, ok := eff.Profile.Actors[actor]
	if !ok {
		return 0, 0, WrapExit(ExitValidation, fmt.Errorf("unknown actor %q", actor))
	}
	addr, err := common.NewAddress(a.Address)
	if err != nil {
		return 0, 0, WrapExit(ExitValidation, fmt.Errorf("actor %s: invalid address: %w", actor, err))
	}
	prov, err := provider.New(eff)
	if err != nil {
		return 0, 0, WrapExit(ExitProvider, err)
	}
	total, count, err := chainquery.SumLovelace(ctx, prov, addr)
	if err != nil {
		return 0, 0, WrapExit(ExitProvider, err)
	}
	slog.Info("Queried wallet balance", "actor", actor, "lovelace", total, "utxos", count)
	return total, count, nil
}

func runRegisterTLD(ctx context.Context, eff *config.Effective, tldRaw, proofPath, out string) (string, error) {
	slog.Info("Starting register-tld", "tld", tldRaw, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	proof, err := domain.LoadProofBundle(proofPath, tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", mapContextErr(err)
	}
	outBuild, err := txbuilder.RegisterTLD(ctx, bctx, tld, proof, out, ContractRevision)
	if err != nil {
		slog.Error("register-tld failed", "error", err)
		return "", WrapExit(ExitBuild, err)
	}
	slog.Info("Built register-tld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

func runActivateTLD(ctx context.Context, eff *config.Effective, tldRaw, proofPath, out string) (string, error) {
	slog.Info("Starting activate-tld", "tld", tldRaw, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	proof, err := domain.LoadProofBundle(proofPath, tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", mapContextErr(err)
	}
	outBuild, err := txbuilder.ActivateTLD(ctx, bctx, tld, proof, out, ContractRevision)
	if err != nil {
		slog.Error("activate-tld failed", "error", err)
		return "", WrapExit(ExitBuild, err)
	}
	slog.Info("Built activate-tld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

func runMintSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, sldOwner, out string) (string, error) {
	slog.Info("Starting mint-sld", "tld", tldRaw, "sld", sldRaw, "sldOwner", sldOwner, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	sld, err := domain.ParseLabel(sldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	if _, ok := eff.Profile.Actors[sldOwner]; !ok {
		return "", WrapExit(ExitValidation, fmt.Errorf("unknown actor %q", sldOwner))
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", mapContextErr(err)
	}
	outBuild, err := txbuilder.MintSLD(ctx, bctx, tld, sld, sldOwner, out, ContractRevision)
	if err != nil {
		slog.Error("mint-sld failed", "error", err)
		return "", WrapExit(ExitBuild, err)
	}
	slog.Info("Built mint-sld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

func runUpdateSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, recordsPath, out string) (string, error) {
	slog.Info("Starting update-sld", "tld", tldRaw, "sld", sldRaw, "records", recordsPath, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	sld, err := domain.ParseLabel(sldRaw)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	maxRecords := 256
	if eff.Profile.Transaction.MaxDatumBytes > 0 {
		maxRecords = 256
	}
	records, err := domain.LoadRecordsFile(recordsPath, maxRecords)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", mapContextErr(err)
	}
	outBuild, err := txbuilder.UpdateSLD(ctx, bctx, tld, sld, records, out, ContractRevision)
	if err != nil {
		slog.Error("update-sld failed", "error", err)
		return "", WrapExit(ExitBuild, err)
	}
	slog.Info("Built update-sld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4), "recordCount", len(records))
	return outBuild.EnvelopePath, nil
}

func runTxInspect(txPath string) (map[string]any, error) {
	slog.Info("Inspecting transaction", "path", txPath)
	env, err := artifact.ReadEnvelope(txPath)
	if err != nil {
		return nil, WrapExit(ExitValidation, err)
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return nil, WrapExit(ExitValidation, err)
	}
	tx, err := artifact.DecodeConwayTx(raw)
	if err != nil {
		return nil, WrapExit(ExitValidation, err)
	}
	bodyHash, err := artifact.BodyHashHex(tx)
	if err != nil {
		return nil, WrapExit(ExitValidation, err)
	}
	manPath := artifact.SiblingManifestPath(txPath)
	data := map[string]any{
		"type":        env.Type,
		"description": env.Description,
		"bodyHash":    bodyHash,
		"witnesses":   artifact.CountVKeyWitnesses(tx),
	}
	if man, err := artifact.ReadManifest(manPath); err == nil {
		data["manifest"] = man
	}
	slog.Debug("Transaction inspected", "bodyHash", logging.HexPrefix(bodyHash, 4), "witnesses", data["witnesses"])
	return data, nil
}

func runTxSign(eff *config.Effective, txPath, actor, out string, allowExtra bool) error {
	slog.Info("Signing transaction", "tx", txPath, "actor", actor, "out", out)
	a, ok := eff.Profile.Actors[actor]
	if !ok {
		return WrapExit(ExitValidation, fmt.Errorf("unknown actor %q", actor))
	}
	src, err := wallet.FromActor(actor, a, eff.Profile.Network.Name)
	if err != nil {
		return WrapExit(ExitWallet, err)
	}
	w, err := src.LoadWallet()
	if err != nil {
		return WrapExit(ExitWallet, err)
	}
	manPath := artifact.SiblingManifestPath(txPath)
	if err := artifact.SignWithWallet(txPath, manPath, out, w, allowExtra); err != nil {
		slog.Error("tx sign failed", "error", err, "actor", actor)
		return WrapExit(ExitSign, err)
	}
	slog.Info("Transaction signed", "actor", actor, "out", out)
	return nil
}

func runTxSubmit(ctx context.Context, eff *config.Effective, txPath string) (string, string, error) {
	slog.Info("Submitting transaction", "tx", txPath, "provider", eff.Profile.Provider.Type)
	env, err := artifact.ReadEnvelope(txPath)
	if err != nil {
		return "", "", WrapExit(ExitValidation, err)
	}
	if env.Type != artifact.TypeWitnessedConway {
		return "", "", WrapExit(ExitSubmit, fmt.Errorf("transaction is not fully signed (type %q)", env.Type))
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return "", "", WrapExit(ExitValidation, err)
	}
	tx, err := artifact.DecodeConwayTx(raw)
	if err != nil {
		return "", "", WrapExit(ExitValidation, err)
	}
	if artifact.CountVKeyWitnesses(tx) == 0 {
		return "", "", WrapExit(ExitSubmit, fmt.Errorf("transaction has no vkey witnesses"))
	}
	prov, err := provider.New(eff)
	if err != nil {
		return "", "", WrapExit(ExitProvider, err)
	}
	if man, err := artifact.ReadManifest(artifact.SiblingManifestPath(txPath)); err == nil {
		if !strings.EqualFold(man.Network, eff.Profile.Network.Name) {
			return "", "", WrapExit(ExitSubmit, fmt.Errorf("manifest network %q does not match profile %q", man.Network, eff.Profile.Network.Name))
		}
		if !strings.EqualFold(man.Provider, eff.Profile.Provider.Type) {
			return "", "", WrapExit(ExitSubmit, fmt.Errorf("manifest provider %q does not match profile %q", man.Provider, eff.Profile.Provider.Type))
		}
	}
	txID, err := prov.SubmitTx(raw)
	if err != nil {
		slog.Error("tx submit failed", "error", err)
		return "", "", WrapExit(ExitSubmit, err)
	}
	txHex := txID.String()
	explorer := strings.ReplaceAll(eff.Profile.Network.ExplorerTxURL, "{txId}", txHex)
	slog.Info("Transaction submitted", "txId", txHex, "explorer", explorer)
	return txHex, explorer, nil
}

func runTxStatus(ctx context.Context, eff *config.Effective, txID, manifestPath string, wait bool, timeout time.Duration) (string, error) {
	slog.Info("Checking transaction status", "txId", txID, "wait", wait)
	prov, err := provider.New(eff)
	if err != nil {
		return "", WrapExit(ExitProvider, err)
	}
	var indexes []uint32
	if manifestPath == "" {
		manifestPath = ""
	}
	if manifestPath != "" {
		man, err := artifact.ReadManifest(manifestPath)
		if err != nil {
			return "", WrapExit(ExitValidation, err)
		}
		for _, o := range man.ExpectedOutputs {
			indexes = append(indexes, o.Index)
		}
	}
	hash, err := txIDToBlake(txID)
	if err != nil {
		return "", WrapExit(ExitValidation, err)
	}
	if !wait {
		if len(indexes) == 0 {
			return "status check requires --manifest with expected outputs or --wait", nil
		}
		for _, idx := range indexes {
			utxo, err := prov.UtxoByRef(hash, idx)
			if err != nil || utxo == nil {
				return "pending", nil
			}
		}
		return "confirmed", nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(indexes) == 0 {
		return "", WrapExit(ExitValidation, fmt.Errorf("--wait requires --manifest with expected output indexes"))
	}
	if err := prov.AwaitOutputs(ctx, hash, indexes); err != nil {
		slog.Warn("Transaction confirmation timed out or canceled", "txId", txID, "error", err)
		return "", WrapExit(ExitTimeout, err)
	}
	slog.Info("Transaction confirmed", "txId", txID)
	return "confirmed", nil
}

// runConfigValidateOnline checks placeholders, provider health, and reference UTxOs.
func runConfigValidateOnline(ctx context.Context, eff *config.Effective) error {
	if err := config.ValidateOnline(eff); err != nil {
		return err
	}
	prov, err := provider.New(eff)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	if err := prov.Health(ctx); err != nil {
		return fmt.Errorf("provider health (%s): %w", prov.Name(), err)
	}
	for name, ref := range eff.Profile.Contracts.ReferenceUtxos {
		path := fmt.Sprintf("profiles.%s.contracts.referenceUtxos.%s", eff.Name, name)
		txHash, index, err := config.ParseUTxORef(ref)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hash, err := txIDToBlake(txHash)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		utxo, err := prov.UtxoByRef(hash, index)
		if err != nil {
			return fmt.Errorf("%s: resolve %s: %w", path, ref, err)
		}
		if utxo == nil {
			return fmt.Errorf("%s: utxo %s not found", path, ref)
		}
		if utxo.Output.ScriptRef() == nil {
			return fmt.Errorf("%s: utxo %s exists but has no reference script", path, ref)
		}
		slog.Debug("Reference UTxO verified", "key", name, "ref", ref)
	}
	slog.Info("Online config validation passed", "profile", eff.Name, "provider", prov.Name())
	return nil
}

func mapContextErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "provider"):
		return WrapExit(ExitProvider, err)
	case strings.Contains(msg, "contracts"):
		return WrapExit(ExitConfig, err)
	default:
		return WrapExit(ExitBuild, err)
	}
}
