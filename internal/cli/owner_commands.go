package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newOwnerCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "owner",
		Short: "Owner-side TLD/SLD operations",
	}
	cmd.AddCommand(newActivateTLDCmd(g))
	cmd.AddCommand(newMintSLDCmd(g))
	cmd.AddCommand(newUpdateSLDCmd(g))
	return cmd
}

func newActivateTLDCmd(g *GlobalFlags) *cobra.Command {
	var tld, proof, out string
	cmd := &cobra.Command{
		Use:   "activate-tld",
		Short: "Build an unsigned TLD activation (first OwnerAction) transaction",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if tld == "" || proof == "" || out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tld, --proof, and --out are required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			artifact, err := runActivateTLD(cmd.Context(), eff, tld, proof, out)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "owner activate-tld",
				Network:   eff.Profile.Network.Name,
				Operation: "activate-tld",
				Artifact:  artifact,
				Message:   "built unsigned activate-tld transaction",
				Data: map[string]any{
					"tld": tld,
					"out": out,
				},
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "top-level domain label to activate")
	cmd.Flags().StringVar(&proof, "proof", "", "path to static Handshake proof JSON bundle")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest")
	return cmd
}

func newMintSLDCmd(g *GlobalFlags) *cobra.Command {
	var tld, sld, sldOwner, out string
	cmd := &cobra.Command{
		Use:   "mint-sld",
		Short: "Build an unsigned SLD mint under an activated TLD",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if tld == "" || sld == "" || sldOwner == "" || out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tld, --sld, --sld-owner, and --out are required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			artifact, err := runMintSLD(cmd.Context(), eff, tld, sld, sldOwner, out)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "owner mint-sld",
				Network:   eff.Profile.Network.Name,
				Operation: "mint-sld",
				Artifact:  artifact,
				Message:   "built unsigned mint-sld transaction",
				Data: map[string]any{
					"tld":      tld,
					"sld":      sld,
					"sldOwner": sldOwner,
					"out":      out,
				},
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "parent top-level domain")
	cmd.Flags().StringVar(&sld, "sld", "", "second-level domain label to mint")
	cmd.Flags().StringVar(&sldOwner, "sld-owner", "", "actor name that should receive the SLD user token")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest")
	return cmd
}

func newUpdateSLDCmd(g *GlobalFlags) *cobra.Command {
	var tld, sld, records, out string
	cmd := &cobra.Command{
		Use:   "update-sld",
		Short: "Build an unsigned SLD DNS record update transaction",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if tld == "" || sld == "" || records == "" || out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tld, --sld, --records, and --out are required"))
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			artifact, err := runUpdateSLD(cmd.Context(), eff, tld, sld, records, out)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "owner update-sld",
				Network:   eff.Profile.Network.Name,
				Operation: "update-sld",
				Artifact:  artifact,
				Message:   "built unsigned update-sld transaction",
				Data: map[string]any{
					"tld":     tld,
					"sld":     sld,
					"records": records,
					"out":     out,
				},
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "parent top-level domain")
	cmd.Flags().StringVar(&sld, "sld", "", "second-level domain whose records will be replaced")
	cmd.Flags().StringVar(&records, "records", "", "path to DNS records JSON file (complete replacement)")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest")
	return cmd
}
