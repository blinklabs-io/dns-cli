package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRegistrarCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registrar",
		Short: "Registrar-side domain operations",
	}
	cmd.AddCommand(newRegisterTLDCmd(g))
	return cmd
}

func newRegisterTLDCmd(g *GlobalFlags) *cobra.Command {
	var tld, proof, out string
	cmd := &cobra.Command{
		Use:   "register-tld",
		Short: "Build an unsigned TLD registration transaction",
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
			artifact, err := runRegisterTLD(cmd.Context(), eff, tld, proof, out)
			if err != nil {
				return err
			}
			return p.Success(Result{
				Command:   "registrar register-tld",
				Network:   eff.Profile.Network.Name,
				Operation: "register-tld",
				Artifact:  artifact,
				Message:   "built unsigned register-tld transaction",
				Data: unsignedBuildData(out, map[string]any{
					"tld":   tld,
					"proof": proof,
				}),
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "top-level domain label to register")
	cmd.Flags().StringVar(&proof, "proof", "", "path to static Handshake proof JSON bundle")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
	return cmd
}
