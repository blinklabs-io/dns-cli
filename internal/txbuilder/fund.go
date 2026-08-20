package txbuilder

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/config"
)

// FundAllocation is a destination actor and total ADA to send (lovelace).
type FundAllocation struct {
	Actor    string
	Lovelace int64
}

// feeBufferLovelace is a conservative lower bound reserved for fees when
// checking bootstrap balance before coin selection.
const feeBufferLovelace int64 = 200_000

// ParseFundAllocation parses "actor=lovelace" (e.g. registrar=30000000).
func ParseFundAllocation(raw string) (FundAllocation, error) {
	raw = strings.TrimSpace(raw)
	actor, amount, ok := strings.Cut(raw, "=")
	if !ok {
		return FundAllocation{}, fmt.Errorf("invalid allocation %q (want actor=lovelace)", raw)
	}
	actor = strings.TrimSpace(actor)
	amount = strings.TrimSpace(amount)
	if actor == "" {
		return FundAllocation{}, fmt.Errorf("invalid allocation %q: empty actor", raw)
	}
	if amount == "" {
		return FundAllocation{}, fmt.Errorf("invalid allocation %q: empty lovelace amount", raw)
	}
	lovelace, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		return FundAllocation{}, fmt.Errorf("invalid allocation %q: lovelace must be an integer", raw)
	}
	if lovelace <= 0 {
		return FundAllocation{}, fmt.Errorf("invalid allocation %q: lovelace must be positive", raw)
	}
	return FundAllocation{Actor: actor, Lovelace: lovelace}, nil
}

// ValidateFundAllocations checks collateral constraints and uniqueness.
func ValidateFundAllocations(allocations []FundAllocation, collateralLovelace int64, actors map[string]string) error {
	if collateralLovelace <= 0 {
		return fmt.Errorf("collateral must be positive (got %d)", collateralLovelace)
	}
	if len(allocations) == 0 {
		return fmt.Errorf("at least one --allocation is required")
	}
	seenActors := make(map[string]struct{}, len(allocations))
	seenAddrs := make(map[string]struct{}, len(allocations))
	for _, alloc := range allocations {
		if alloc.Actor == "" {
			return fmt.Errorf("allocation actor is required")
		}
		if alloc.Lovelace <= collateralLovelace {
			return fmt.Errorf("allocation for %q (%d) must be greater than collateral (%d)", alloc.Actor, alloc.Lovelace, collateralLovelace)
		}
		if _, dup := seenActors[alloc.Actor]; dup {
			return fmt.Errorf("duplicate allocation actor %q", alloc.Actor)
		}
		seenActors[alloc.Actor] = struct{}{}
		addr, ok := actors[alloc.Actor]
		if !ok {
			return fmt.Errorf("unknown actor %q", alloc.Actor)
		}
		if addr == "" {
			return fmt.Errorf("actor %q has empty address", alloc.Actor)
		}
		if _, dup := seenAddrs[addr]; dup {
			return fmt.Errorf("duplicate destination address for actor %q", alloc.Actor)
		}
		seenAddrs[addr] = struct{}{}
	}
	return nil
}

// FundActors builds an unsigned ADA funding transaction from fromActor.
// For each allocation it emits two ADA-only outputs in order:
//
//  1. collateral exactly collateralLovelace
//  2. spend = allocation - collateral
func FundActors(ctx context.Context, bctx *Context, fromActor string, allocations []FundAllocation, collateralLovelace int64, outPrefix string) (BuildOutput, error) {
	slog.Info("Building wallet fund transaction", "from", fromActor, "allocations", len(allocations), "collateral", collateralLovelace)

	network := strings.ToLower(strings.TrimSpace(bctx.Eff.Profile.Network.Name))
	if _, err := config.NetworkDefaults(network); err != nil {
		return BuildOutput{}, fmt.Errorf("wallet fund: %w", err)
	}
	if fromActor == "" {
		return BuildOutput{}, fmt.Errorf("from-actor is required")
	}
	if strings.TrimSpace(outPrefix) == "" {
		return BuildOutput{}, fmt.Errorf("out prefix is required")
	}

	actorAddrs := make(map[string]string, len(bctx.Eff.Profile.Actors))
	for name, a := range bctx.Eff.Profile.Actors {
		actorAddrs[name] = a.Address
	}
	if err := ValidateFundAllocations(allocations, collateralLovelace, actorAddrs); err != nil {
		return BuildOutput{}, err
	}
	if _, ok := bctx.Eff.Profile.Actors[fromActor]; !ok {
		return BuildOutput{}, fmt.Errorf("unknown from-actor %q", fromActor)
	}

	fromAddr, err := actorAddress(bctx.Eff, fromActor)
	if err != nil {
		return BuildOutput{}, err
	}
	fromPKH, err := loadActorKeyHash(bctx.Eff, fromActor)
	if err != nil {
		return BuildOutput{}, err
	}

	var totalAlloc int64
	for _, alloc := range allocations {
		totalAlloc += alloc.Lovelace
	}
	needed := totalAlloc + feeBufferLovelace
	// Faucet / prior funding can be confirmed while the address API still 404s.
	if err := chainquery.EnsureFundingVisible(ctx, bctx.Provider, fromAddr, needed); err != nil {
		return BuildOutput{}, fmt.Errorf("bootstrap funding: %w", err)
	}
	funding, err := chainquery.LoadFundingUTxOs(ctx, bctx.Provider, fromAddr)
	if err != nil {
		return BuildOutput{}, fmt.Errorf("load funding utxos: %w", err)
	}
	var available int64
	for _, u := range funding {
		available += u.Output.Amount().Int64()
	}
	if available < needed {
		return BuildOutput{}, fmt.Errorf("insufficient bootstrap funds: need at least %d lovelace (allocations %d + fees), have %d", needed, totalAlloc, available)
	}

	a := bctx.newApollo(fromAddr)
	a.AddLoadedUTxOs(funding...)
	start, ttl, err := bctx.validityWindow()
	if err != nil {
		return BuildOutput{}, err
	}
	a.SetValidityStart(start).SetTtl(ttl)
	pkh, err := decodeKeyHash(fromPKH)
	if err != nil {
		return BuildOutput{}, err
	}
	a.AddRequiredSigner(pkh)

	var expected []artifact.ExpectedOutput
	var outIndex uint32
	extra := map[string]string{
		"fromActor":  fromActor,
		"collateral": strconv.FormatInt(collateralLovelace, 10),
	}
	for i, alloc := range allocations {
		dest, err := actorAddress(bctx.Eff, alloc.Actor)
		if err != nil {
			return BuildOutput{}, err
		}
		spend := alloc.Lovelace - collateralLovelace
		a.PayToAddress(dest, collateralLovelace)
		a.PayToAddress(dest, spend)
		expected = append(expected,
			artifact.ExpectedOutput{Role: alloc.Actor + "-collateral", Index: outIndex},
			artifact.ExpectedOutput{Role: alloc.Actor + "-spend", Index: outIndex + 1},
		)
		outIndex += 2
		extra[fmt.Sprintf("allocation.%d.actor", i)] = alloc.Actor
		extra[fmt.Sprintf("allocation.%d.lovelace", i)] = strconv.FormatInt(alloc.Lovelace, 10)
		slog.Debug("Queued fund outputs", "actor", alloc.Actor, "collateral", collateralLovelace, "spend", spend)
	}

	return bctx.finalize(a, "wallet.fund", "unsigned wallet fund", outPrefix, "",
		[]string{fromPKH},
		expected,
		extra,
	)
}
