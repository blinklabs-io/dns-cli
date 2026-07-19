package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/chainquery"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/spf13/cobra"
)

func newWalletCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Manage dns-cli signing wallets",
	}
	cmd.AddCommand(newWalletCreateCmd(g))
	cmd.AddCommand(newWalletFundCmd(g))
	cmd.AddCommand(newWalletBalanceCmd(g))
	cmd.AddCommand(newWalletWaitFundsCmd(g))
	return cmd
}

func newWalletCreateCmd(g *GlobalFlags) *cobra.Command {
	var name, network, format, outDir string
	var force bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Generate a preprod wallet with text-envelope keys and optional mnemonic",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if network == "" {
				network = "preprod"
			}
			if format == "" {
				format = string(wallet.FormatBoth)
			}
			if outDir == "" {
				outDir = filepath.Join("wallets", name)
			}
			generated, err := wallet.GenerateWallet(wallet.GenerateOptions{
				Name:    name,
				Network: network,
				Format:  wallet.WalletFormat(format),
				OutDir:  outDir,
				Force:   force,
			})
			if err != nil {
				return WrapExit(ExitWallet, err)
			}
			return p.Success(Result{
				Command:   "wallet create",
				Network:   generated.Network,
				Operation: "wallet.create",
				Message:   fmt.Sprintf("created wallet %q at %s", generated.Name, outDir),
				Data: map[string]any{
					"name":           generated.Name,
					"address":        generated.Address,
					"paymentKeyHash": generated.PaymentKeyHash,
					"stakeKeyHash":   generated.StakeKeyHash,
					"paths":          generated.Paths,
					"outDir":         outDir,
					"format":         format,
				},
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "wallet name (required)")
	cmd.Flags().StringVar(&network, "network", "preprod", "network profile (preprod only)")
	cmd.Flags().StringVar(&format, "format", string(wallet.FormatBoth), "output format (key-envelope|mnemonic|both)")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "directory for wallet artifacts")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing wallet artifacts")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newWalletFundCmd(g *GlobalFlags) *cobra.Command {
	var fromActor, out string
	var collateral int64
	var allocations []string
	cmd := &cobra.Command{
		Use:   "fund",
		Short: "Build an unsigned preprod funding transaction for actors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if fromActor == "" {
				fromActor = "bootstrap"
			}
			if collateral <= 0 {
				collateral = chainquery.MinCollateralLovelace
			}
			if out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--out is required"))
			}
			if len(allocations) == 0 {
				return WrapExit(ExitUsage, fmt.Errorf("at least one --allocation actor=lovelace is required"))
			}
			parsed := make([]txbuilder.FundAllocation, 0, len(allocations))
			for _, raw := range allocations {
				alloc, err := txbuilder.ParseFundAllocation(raw)
				if err != nil {
					return WrapExit(ExitValidation, err)
				}
				parsed = append(parsed, alloc)
			}
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
			}
			artifact, err := runWalletFund(cmd.Context(), eff, fromActor, parsed, collateral, out)
			if err != nil {
				return err
			}
			data := unsignedBuildData(out, map[string]any{
				"fromActor":  fromActor,
				"collateral": collateral,
			})
			allocData := make([]map[string]any, 0, len(parsed))
			for _, a := range parsed {
				allocData = append(allocData, map[string]any{
					"actor":    a.Actor,
					"lovelace": a.Lovelace,
				})
			}
			data["allocations"] = allocData
			return p.Success(Result{
				Command:   "wallet fund",
				Network:   eff.Profile.Network.Name,
				Operation: "wallet.fund",
				Artifact:  artifact,
				Message:   "built unsigned wallet fund transaction",
				Data:      data,
			})
		},
	}
	cmd.Flags().StringVar(&fromActor, "from-actor", "bootstrap", "funding source actor (fee payer/signer)")
	cmd.Flags().StringArrayVar(&allocations, "allocation", nil, "destination funding as actor=lovelace (repeatable)")
	cmd.Flags().Int64Var(&collateral, "collateral", chainquery.MinCollateralLovelace, "ADA-only collateral lovelace per actor")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func newWalletBalanceCmd(g *GlobalFlags) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Query total lovelace for a configured actor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if actor == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--actor is required"))
			}
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
			}
			total, count, err := runWalletBalance(cmd.Context(), eff, actor)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "wallet balance",
				Network:   eff.Profile.Network.Name,
				Operation: "wallet.balance",
				Message:   fmt.Sprintf("actor %s holds %d lovelace across %d utxos", actor, total, count),
				Data: map[string]any{
					"actor":    actor,
					"lovelace": total,
					"utxos":    count,
					"address":  eff.Profile.Actors[actor].Address,
				},
			})
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "actor name from config")
	_ = cmd.MarkFlagRequired("actor")
	return cmd
}

func newWalletWaitFundsCmd(g *GlobalFlags) *cobra.Command {
	var actor string
	var minLovelace int64
	var poll time.Duration
	cmd := &cobra.Command{
		Use:   "wait-funds",
		Short: "Wait until a configured actor reaches a minimum lovelace balance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if actor == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--actor is required"))
			}
			if minLovelace <= 0 {
				return WrapExit(ExitUsage, fmt.Errorf("--min-lovelace must be positive"))
			}
			if poll <= 0 {
				return WrapExit(ExitUsage, fmt.Errorf("--poll must be positive"))
			}
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
			}
			timeout := time.Duration(0)
			if flag := cmd.Flag("timeout"); flag != nil && flag.Changed {
				timeout = g.Timeout
			}
			result, err := runWalletWaitFunds(cmd.Context(), eff, actor, minLovelace, poll, timeout)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "wallet wait-funds",
				Network:   eff.Profile.Network.Name,
				Operation: "wallet.wait-funds",
				Message:   fmt.Sprintf("actor %s funded with %d lovelace", actor, result.Lovelace),
				Data: map[string]any{
					"actor":    result.Actor,
					"address":  result.Address,
					"lovelace": result.Lovelace,
					"utxos":    result.UTXOs,
					"waited":   result.Waited.String(),
					"attempts": result.Attempts,
				},
			})
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "actor name from config")
	cmd.Flags().Int64Var(&minLovelace, "min-lovelace", 150000000, "minimum lovelace required before returning")
	cmd.Flags().DurationVar(&poll, "poll", 20*time.Second, "poll interval between balance checks")
	_ = cmd.MarkFlagRequired("actor")
	return cmd
}
