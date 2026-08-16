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
	var tld, ownerKey, out string
	cmd := &cobra.Command{
		Use:   "activate-tld",
		Short: "Build an unsigned TLD activation (first OwnerAction) transaction",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if tld == "" || ownerKey == "" || out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tld, --owner-key, and --out are required"))
			}
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
			}
			artifact, err := runActivateTLD(cmd.Context(), eff, tld, ownerKey, out)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "owner activate-tld",
				Network:   eff.Profile.Network.Name,
				Operation: "activate-tld",
				Artifact:  artifact,
				Message:   "built unsigned activate-tld transaction",
				Data: unsignedBuildData(out, map[string]any{
					"tld":      tld,
					"ownerKey": ownerKey,
				}),
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "top-level domain label to activate")
	cmd.Flags().StringVar(&ownerKey, "owner-key", "", "path to the owner's HNS key JSON file (signs just-in-time against the located registration UTxO)")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
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
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
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
				Data: unsignedBuildData(out, map[string]any{
					"tld":      tld,
					"sld":      sld,
					"sldOwner": sldOwner,
				}),
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "parent top-level domain")
	cmd.Flags().StringVar(&sld, "sld", "", "second-level domain label to mint")
	cmd.Flags().StringVar(&sldOwner, "sld-owner", "", "actor name that should receive the SLD user token")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
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
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
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
				Data: unsignedBuildData(out, map[string]any{
					"tld":     tld,
					"sld":     sld,
					"records": records,
				}),
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "parent top-level domain")
	cmd.Flags().StringVar(&sld, "sld", "", "second-level domain whose records will be replaced")
	cmd.Flags().StringVar(&records, "records", "", "path to DNS records JSON file (complete replacement)")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
	return cmd
}
