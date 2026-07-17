// Package txbuilder constructs unsigned Apollo v2 transactions for DNS flows.
package txbuilder

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	apollo "github.com/Salvionied/apollo/v2"
	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/buildmeta"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/protocol"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
)

// BuildOutput is the unsigned transaction artifact bundle.
type BuildOutput struct {
	EnvelopePath     string
	ManifestPath     string
	BodyHash         string
	ContractRevision string
}

// Context carries shared builder dependencies.
type Context struct {
	Eff       *config.Effective
	Provider  provider.Provider
	Contracts chainquery.Contracts
	Blueprint *protocol.Blueprint
}

// NewContext resolves deployment metadata for transaction building.
func NewContext(ctx context.Context, eff *config.Effective) (*Context, error) {
	_ = ctx
	slog.Debug("Initializing transaction builder context", "profile", eff.Name, "network", eff.Profile.Network.Name)
	if err := config.RequireContractIDs(eff); err != nil {
		return nil, err
	}
	prov, err := provider.New(eff)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	contracts, err := chainquery.LoadContracts(eff)
	if err != nil {
		return nil, err
	}
	bp, err := protocol.LoadBlueprint(eff.Profile.Contracts.BlueprintPath)
	if err != nil {
		return nil, fmt.Errorf("load blueprint: %w", err)
	}
	slog.Debug("Transaction builder context ready", "provider", prov.Name(), "blueprint", eff.Profile.Contracts.BlueprintPath)
	return &Context{Eff: eff, Provider: prov, Contracts: contracts, Blueprint: bp}, nil
}

// NewFundingContext loads provider + actors only (no contract IDs or blueprint).
// Used for bootstrap funding before system bind.
func NewFundingContext(ctx context.Context, eff *config.Effective) (*Context, error) {
	_ = ctx
	slog.Debug("Initializing funding context", "profile", eff.Name, "network", eff.Profile.Network.Name)
	prov, err := provider.New(eff)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	slog.Debug("Funding context ready", "provider", prov.Name())
	return &Context{Eff: eff, Provider: prov}, nil
}

func (c *Context) newApollo(feePayer common.Address) *apollo.Apollo {
	return apollo.New(c.Provider).SetWallet(apollo.NewExternalWallet(feePayer))
}

func (c *Context) validityWindow() (start, ttl int64, err error) {
	tip, err := c.Provider.Tip()
	if err != nil {
		return 0, 0, fmt.Errorf("chain tip: %w", err)
	}
	start = int64(tip)
	ttl = start + c.Eff.Profile.Transaction.TTLSlots
	return start, ttl, nil
}

func (c *Context) addReferenceInput(a *apollo.Apollo, key string) error {
	ref, ok := c.Eff.Profile.Contracts.ReferenceUtxos[key]
	if !ok {
		return fmt.Errorf("missing reference utxo %q", key)
	}
	hash, index, err := config.ParseUTxORef(ref)
	if err != nil {
		return err
	}
	_, err = a.AddReferenceInput(hash, int(index))
	if err != nil {
		return err
	}
	slog.Debug("Added reference input", "key", key, "ref", ref)
	return nil
}

func (c *Context) finalize(a *apollo.Apollo, operation, description, outPrefix, contractRevision string, required []string, outputs []artifact.ExpectedOutput, extra map[string]string) (BuildOutput, error) {
	slog.Info("Completing transaction", "operation", operation)
	completed, err := a.Complete()
	if err != nil {
		slog.Error("Transaction build failed", "operation", operation, "error", err)
		return BuildOutput{}, fmt.Errorf("complete transaction: %w", err)
	}
	cborBytes, err := completed.GetTxCbor()
	if err != nil {
		return BuildOutput{}, fmt.Errorf("encode transaction: %w", err)
	}
	if c.Provider != nil {
		cborBytes, err = c.fixScriptDataHash(cborBytes)
		if err != nil {
			return BuildOutput{}, fmt.Errorf("fix script data hash: %w", err)
		}
	}
	tx, err := artifact.DecodeConwayTx(cborBytes)
	if err != nil {
		return BuildOutput{}, err
	}
	bodyHash, err := artifact.BodyHashHex(tx)
	if err != nil {
		return BuildOutput{}, err
	}
	start, ttl, _ := c.validityWindow()
	cfgRaw, _ := os.ReadFile(c.Eff.Path)
	manifest := artifact.Manifest{
		Operation:        operation,
		Network:          c.Eff.Profile.Network.Name,
		Provider:         c.Eff.Profile.Provider.Type,
		BodyHash:         bodyHash,
		RequiredSigners:  required,
		ExpectedOutputs:  outputs,
		ValidityInterval: artifact.ValidityInterval{Start: start, TTL: ttl},
		ConfigDigest:     artifact.DigestConfig(cfgRaw),
		ApolloRevision:   buildmeta.ApolloRevision(),
		ContractRevision: contractRevision,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Extra:            extra,
	}
	envPath, err := artifact.WriteUnsigned(outPrefix, cborBytes, description, manifest)
	if err != nil {
		return BuildOutput{}, err
	}
	slog.Info("Unsigned transaction written", "operation", operation, "envelope", envPath, "bodyHash", bodyHash, "signers", len(required))
	slog.Debug("Transaction manifest", "operation", operation, "outputs", len(outputs), "ttl", ttl)
	return BuildOutput{
		EnvelopePath:     envPath,
		ManifestPath:     outPrefix + ".manifest.json",
		BodyHash:         bodyHash,
		ContractRevision: contractRevision,
	}, nil
}

func actorAddress(eff *config.Effective, name string) (common.Address, error) {
	a, ok := eff.Profile.Actors[name]
	if !ok {
		return common.Address{}, fmt.Errorf("actor %q not found in config", name)
	}
	return common.NewAddress(a.Address)
}

func loadActorKeyHash(eff *config.Effective, name string) (string, error) {
	src, err := wallet.FromActor(name, eff.Profile.Actors[name], eff.Profile.Network.Name)
	if err != nil {
		return "", err
	}
	w, err := src.LoadWallet()
	if err != nil {
		return "", err
	}
	return wallet.PubKeyHashHex(w), nil
}

func policyBytes(hexID string) ([]byte, error) {
	return protocol.HexPolicyID(hexID)
}

func unit(policyID, assetNameHex string, qty int64) apollo.Unit {
	return apollo.NewUnit(policyID, assetNameHex, qty)
}

func plutusDatum(pd data.PlutusData) *common.Datum {
	d := protocol.ToDatum(pd)
	return &d
}

func plutusRedeemer(pd data.PlutusData) common.Datum {
	return protocol.ToDatum(pd)
}

func decodeKeyHash(hexPKH string) (common.Blake2b224, error) {
	b, err := hex.DecodeString(hexPKH)
	if err != nil || len(b) != 28 {
		return common.Blake2b224{}, fmt.Errorf("invalid key hash %q", hexPKH)
	}
	var pkh common.Blake2b224
	copy(pkh[:], b)
	return pkh, nil
}
