package ops

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
)

// RegisterTLD builds an unsigned TLD registration transaction. Returns envelope path.
func (c *Client) RegisterTLD(ctx context.Context, eff *config.Effective, tldRaw, proofPath, out string) (string, error) {
	slog.Info("Starting register-tld", "tld", tldRaw, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", err
	}
	proof, err := domain.LoadProofBundle(proofPath, tldRaw)
	if err != nil {
		return "", err
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", err
	}
	outBuild, err := txbuilder.RegisterTLD(ctx, bctx, tld, proof, out, c.ContractRevision)
	if err != nil {
		slog.Error("register-tld failed", "error", err)
		return "", err
	}
	slog.Info("Built register-tld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

// ActivateTLD builds an unsigned TLD activation transaction.
func (c *Client) ActivateTLD(ctx context.Context, eff *config.Effective, tldRaw, proofPath, out string) (string, error) {
	slog.Info("Starting activate-tld", "tld", tldRaw, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", err
	}
	proof, err := domain.LoadProofBundle(proofPath, tldRaw)
	if err != nil {
		return "", err
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", err
	}
	outBuild, err := txbuilder.ActivateTLD(ctx, bctx, tld, proof, out, c.ContractRevision)
	if err != nil {
		slog.Error("activate-tld failed", "error", err)
		return "", err
	}
	slog.Info("Built activate-tld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

// MintSLD builds an unsigned SLD mint transaction.
func (c *Client) MintSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, sldOwner, out string) (string, error) {
	slog.Info("Starting mint-sld", "tld", tldRaw, "sld", sldRaw, "sldOwner", sldOwner, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", err
	}
	sld, err := domain.ParseLabel(sldRaw)
	if err != nil {
		return "", err
	}
	if _, ok := eff.Profile.Actors[sldOwner]; !ok {
		return "", fmt.Errorf("unknown actor %q", sldOwner)
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", err
	}
	outBuild, err := txbuilder.MintSLD(ctx, bctx, tld, sld, sldOwner, out, c.ContractRevision)
	if err != nil {
		slog.Error("mint-sld failed", "error", err)
		return "", err
	}
	slog.Info("Built mint-sld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4))
	return outBuild.EnvelopePath, nil
}

// UpdateSLD builds an unsigned DNS record update transaction.
func (c *Client) UpdateSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, recordsPath, out string) (string, error) {
	slog.Info("Starting update-sld", "tld", tldRaw, "sld", sldRaw, "records", recordsPath, "out", out)
	tld, err := domain.ParseLabel(tldRaw)
	if err != nil {
		return "", err
	}
	sld, err := domain.ParseLabel(sldRaw)
	if err != nil {
		return "", err
	}
	maxRecords := 256
	records, err := domain.LoadRecordsFile(recordsPath, maxRecords)
	if err != nil {
		return "", err
	}
	bctx, err := txbuilder.NewContext(ctx, eff)
	if err != nil {
		return "", err
	}
	outBuild, err := txbuilder.UpdateSLD(ctx, bctx, tld, sld, records, out, c.ContractRevision)
	if err != nil {
		slog.Error("update-sld failed", "error", err)
		return "", err
	}
	slog.Info("Built update-sld transaction", "artifact", outBuild.EnvelopePath, "bodyHash", logging.HexPrefix(outBuild.BodyHash, 4), "recordCount", len(records))
	return outBuild.EnvelopePath, nil
}
