package cli

import (
	"fmt"
	"os"

	"github.com/blinklabs-io/dns-cli/internal/artifact"
	"github.com/spf13/cobra"
)

func newTxCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx",
		Short: "Offline transaction inspect, sign, submit, and status",
	}
	cmd.AddCommand(newTxInspectCmd(g))
	cmd.AddCommand(newTxSignCmd(g))
	cmd.AddCommand(newTxSubmitCmd(g))
	cmd.AddCommand(newTxStatusCmd(g))
	cmd.AddCommand(newTxApplyCmd(g))
	return cmd
}

func newTxInspectCmd(g *GlobalFlags) *cobra.Command {
	var txPath string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a text-envelope transaction artifact",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if txPath == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tx is required"))
			}
			data, err := runTxInspect(txPath)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "tx inspect",
				Operation: "tx.inspect",
				Data:      data,
			})
		},
	}
	cmd.Flags().StringVar(&txPath, "tx", "", "path to unsigned or signed text envelope")
	return cmd
}

func newTxSignCmd(g *GlobalFlags) *cobra.Command {
	var txPath, actor, out, signedAlias string
	var allowExtra bool
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Add one actor witness to a transaction text envelope",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			outPath := firstNonEmptyFlag(out, signedAlias)
			if out != "" && signedAlias != "" && out != signedAlias {
				return WrapExit(ExitUsage, fmt.Errorf("--out and --signed must match when both are set"))
			}
			if txPath == "" || actor == "" || outPath == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tx, --actor, and --out (or --signed) are required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			if err := runTxSign(eff, txPath, actor, outPath, allowExtra); err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "tx sign",
				Network:   eff.Profile.Network.Name,
				Operation: "tx.sign",
				Artifact:  outPath,
				Message:   fmt.Sprintf("added witness for actor %s", actor),
				Data: map[string]any{
					"tx":       txPath,
					"actor":    actor,
					"signed":   outPath,
					"manifest": artifact.SiblingManifestPath(txPath),
				},
			})
		},
	}
	cmd.Flags().StringVar(&txPath, "tx", "", "path to text envelope to sign")
	cmd.Flags().StringVar(&actor, "actor", "", "actor name from config (registrar|tldOwner|sldOwner|custom)")
	cmd.Flags().StringVar(&out, "out", "", "output signed text envelope path")
	cmd.Flags().StringVar(&signedAlias, "signed", "", "alias for --out")
	cmd.Flags().BoolVar(&allowExtra, "allow-extra-signer", false, "allow a signer not listed in the manifest")
	return cmd
}

func newTxSubmitCmd(g *GlobalFlags) *cobra.Command {
	var txPath string
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a fully signed transaction to the selected provider",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if txPath == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tx is required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			txID, explorer, err := runTxSubmit(cmd.Context(), eff, txPath)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:     "tx submit",
				Network:     eff.Profile.Network.Name,
				Operation:   "tx.submit",
				TxID:        txID,
				ExplorerURL: explorer,
				Message:     "transaction submitted",
				Data: map[string]any{
					"tx":          txPath,
					"txId":        txID,
					"explorerUrl": explorer,
				},
			})
		},
	}
	cmd.Flags().StringVar(&txPath, "tx", "", "path to fully signed text envelope")
	return cmd
}

func newTxStatusCmd(g *GlobalFlags) *cobra.Command {
	var txID, manifest string
	var wait bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check or wait for transaction confirmation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if txID == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tx-id is required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			status, err := runTxStatus(cmd.Context(), eff, txID, manifest, wait, g.Timeout, g.Output, !g.NoColor && os.Getenv("NO_COLOR") == "")
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "tx status",
				Network:   eff.Profile.Network.Name,
				Operation: "tx.status",
				TxID:      txID,
				Message:   status,
				Data: map[string]any{
					"txId":     txID,
					"manifest": manifest,
					"status":   status,
					"wait":     wait,
				},
			})
		},
	}
	cmd.Flags().StringVar(&txID, "tx-id", "", "transaction hash to query")
	cmd.Flags().StringVar(&manifest, "manifest", "", "optional manifest with expected output indexes")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait until confirmed or timeout")
	return cmd
}

func newTxApplyCmd(g *GlobalFlags) *cobra.Command {
	var txPath, actor, signed, outAlias, manifest string
	var allowExtra bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Sign, submit, and wait for transaction confirmation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			signedPath := firstNonEmptyFlag(signed, outAlias)
			if signed != "" && outAlias != "" && signed != outAlias {
				return WrapExit(ExitUsage, fmt.Errorf("--signed and --out must match when both are set"))
			}
			if txPath == "" || actor == "" || signedPath == "" || manifest == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tx, --actor, --signed (or --out), and --manifest are required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			result, err := runTxApply(cmd.Context(), eff, txPath, actor, signedPath, manifest, allowExtra, g.Timeout, g.Output, !g.NoColor && os.Getenv("NO_COLOR") == "")
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:     "tx apply",
				Network:     eff.Profile.Network.Name,
				Operation:   "tx.apply",
				TxID:        result.TxID,
				ExplorerURL: result.ExplorerURL,
				Message:     "transaction confirmed",
				Data: map[string]any{
					"tx":          txPath,
					"actor":       actor,
					"signed":      result.SignedPath,
					"manifest":    manifest,
					"status":      result.Status,
					"explorerUrl": result.ExplorerURL,
				},
			})
		},
	}
	cmd.Flags().StringVar(&txPath, "tx", "", "path to unsigned text envelope")
	cmd.Flags().StringVar(&actor, "actor", "", "actor name from config")
	cmd.Flags().StringVar(&signed, "signed", "", "output signed text envelope path")
	cmd.Flags().StringVar(&outAlias, "out", "", "alias for --signed")
	cmd.Flags().StringVar(&manifest, "manifest", "", "manifest with expected output indexes")
	cmd.Flags().BoolVar(&allowExtra, "allow-extra-signer", false, "allow a signer not listed in the manifest")
	return cmd
}
