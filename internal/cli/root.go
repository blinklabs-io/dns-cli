package cli

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/logging"
	"github.com/spf13/cobra"
)

// GlobalFlags holds persistent flags shared by all commands.
type GlobalFlags struct {
	ConfigPath  string
	Network     string
	Provider    string
	Output      string
	Timeout     time.Duration
	Verbose     int
	ArtifactDir string
	NoColor     bool
}

// NewRoot constructs the dns-cli root command tree.
func NewRoot() *cobra.Command {
	g := &GlobalFlags{}
	root := &cobra.Command{
		Use:           "dns-cli",
		Short:         "Go-native CLI for Handshake DNS on Cardano",
		Long:          "dns-cli registers TLDs, activates them, mints SLDs, and updates DNS records without cardano-cli.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := logging.Configure(logging.Options{
				Verbose: g.Verbose,
				NoColor: g.NoColor || os.Getenv("NO_COLOR") != "",
				Output:  g.Output,
				Writer:  cmd.ErrOrStderr(),
			}); err != nil {
				return WrapExit(ExitUsage, err)
			}
			return nil
		},
	}

	root.PersistentFlags().StringVar(&g.ConfigPath, "config", "", "path to dns-cli JSON config")
	root.PersistentFlags().StringVar(&g.Network, "network", "", "network profile override (preview|preprod)")
	root.PersistentFlags().StringVar(&g.Provider, "provider", "", "provider override (utxorpc|blockfrost)")
	root.PersistentFlags().StringVar(&g.Output, "output", "human", "output format (human|json)")
	root.PersistentFlags().DurationVar(&g.Timeout, "timeout", 10*time.Minute, "operation timeout")
	root.PersistentFlags().IntVarP(&g.Verbose, "verbose", "v", 2, "log verbosity 0-4 (error|warn|info|debug|trace)")
	root.PersistentFlags().StringVar(&g.ArtifactDir, "artifact-dir", "", "directory for transaction artifacts")
	root.PersistentFlags().BoolVar(&g.NoColor, "no-color", false, "disable color in human output and logs")

	root.AddCommand(newVersionCmd(g))
	root.AddCommand(newConfigCmd(g))
	root.AddCommand(newWalletCmd(g))
	root.AddCommand(newSystemCmd(g))
	root.AddCommand(newRegistrarCmd(g))
	root.AddCommand(newOwnerCmd(g))
	root.AddCommand(newProofCmd(g))
	root.AddCommand(newTxCmd(g))

	return root
}

func newVersionCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and dependency metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := ParseOutputMode(g.Output)
			if err != nil {
				return WrapExit(ExitUsage, err)
			}
			v := ResolveVersion()
			slog.Debug("Printing version", "version", v.Version)
			p := NewPrinter(mode)
			p.Stdout = cmd.OutOrStdout()
			p.Stderr = cmd.ErrOrStderr()
			if mode == OutputJSON {
				return p.Success(Result{
					Command: "version",
					Data: map[string]any{
						"version":          v.Version,
						"gitCommit":        v.GitCommit,
						"buildDate":        v.BuildDate,
						"goVersion":        v.GoVersion,
						"apolloRevision":   v.ApolloRevision,
						"contractRevision": v.ContractRevision,
					},
				})
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatVersionHuman(v))
			return err
		},
	}
}

// printerFromFlags builds a printer from global flags.
func printerFromFlags(g *GlobalFlags, cmd *cobra.Command) (*Printer, error) {
	mode, err := ParseOutputMode(g.Output)
	if err != nil {
		return nil, WrapExit(ExitUsage, err)
	}
	p := NewPrinter(mode)
	p.Stdout = cmd.OutOrStdout()
	p.Stderr = cmd.ErrOrStderr()
	p.Color = !g.NoColor && os.Getenv("NO_COLOR") == ""
	return p, nil
}
