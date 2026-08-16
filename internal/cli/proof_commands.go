package cli

import (
	"fmt"

	"github.com/blinklabs-io/dns-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newProofCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proof",
		Short: "Handshake proof bundle utilities",
	}
	cmd.AddCommand(newProofGenerateCmd(g))
	return cmd
}

func newProofGenerateCmd(g *GlobalFlags) *cobra.Command {
	var tld, out, ownerKey string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an owner HNS key and a proof bundle for a TLD",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if tld == "" || out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--tld and --out-dir are required"))
			}
			bundleOut, err := domain.GenerateProofBundle(tld, out, ownerKey)
			if err != nil {
				return WrapExit(ExitValidation, err)
			}
			return p.Success(Result{
				Command:   "proof generate",
				Operation: "proof.generate",
				Message:   fmt.Sprintf("wrote proof bundle for %s", tld),
				Data: map[string]any{
					"tld":         tld,
					"outDir":      out,
					"ownerHns":    bundleOut.OwnerHNSPath,
					"proofBundle": bundleOut.ProofBundlePath,
					"ownerKeyIn":  ownerKey,
				},
			})
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "top-level domain label to sign")
	cmd.Flags().StringVar(&out, "out-dir", "", "output directory for owner.hns and proof-bundle.json")
	cmd.Flags().StringVar(&ownerKey, "owner-key", "", "existing owner HNS key JSON (generates a new key when omitted)")
	return cmd
}
