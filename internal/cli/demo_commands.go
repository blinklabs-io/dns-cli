package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/demo"
	"github.com/spf13/cobra"
)

func newDemoCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Preprod demo helpers (history and orchestration)",
	}
	cmd.AddCommand(newDemoHistoryCmd(g))
	cmd.AddCommand(newDemoRunCmd(g))
	return cmd
}

func newDemoRunCmd(g *GlobalFlags) *cobra.Command {
	var demoRoot, runsRoot, mode, provider, tld, sld, logLevel string
	var yes, skipInstall, noClipboard bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the resumable Preprod demo lifecycle (fresh|existing)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if demoRoot == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--demo-root is required"))
			}
			err := demo.Run(cmd.Context(), demo.Options{
				DemoRoot:    demoRoot,
				RunsRoot:    runsRoot,
				Mode:        mode,
				Provider:    provider,
				TLD:         tld,
				SLD:         sld,
				Yes:         yes,
				SkipInstall: skipInstall,
				NoClipboard: noClipboard,
				NoColor:     g.NoColor || os.Getenv("NO_COLOR") != "",
				LogLevel:    logLevel,
				ContractRev: ContractRevision,
				Stdin:       cmd.InOrStdin(),
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
			})
			if err != nil {
				return mapDemoRunErr(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&demoRoot, "demo-root", "", "demo directory containing config/, fixtures/, runs/ (required)")
	cmd.Flags().StringVar(&runsRoot, "runs-root", "", "override runs directory (default <demo-root>/runs)")
	cmd.Flags().StringVar(&mode, "mode", "", "fresh|existing (default fresh)")
	cmd.Flags().StringVar(&provider, "provider", "", "blockfrost|utxorpc (default blockfrost)")
	cmd.Flags().StringVar(&tld, "tld", "", "top-level domain label")
	cmd.Flags().StringVar(&sld, "sld", "", "second-level domain label")
	cmd.Flags().BoolVar(&yes, "yes", false, "auto-approve install/default prompts (not submission confirm)")
	cmd.Flags().BoolVar(&skipInstall, "skip-install", false, "never install tools or write credentials; print guides only")
	cmd.Flags().BoolVar(&noClipboard, "no-clipboard", false, "do not copy bootstrap address to clipboard")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "quiet|normal|extensive")
	return cmd
}

func mapDemoRunErr(err error) error {
	msg := err.Error()
	switch {
	case contains(msg, "--demo-root", "required", "invalid mode", "invalid provider"):
		return WrapExit(ExitUsage, err)
	case contains(msg, "config", "template", "credential", "PROJECT_ID", "UTXORPC"):
		return WrapExit(ExitConfig, err)
	case contains(msg, "apply", "submit"):
		return WrapExit(ExitSubmit, err)
	case contains(msg, "prepare", "proof", "bind", "fund"):
		return WrapExit(ExitBuild, err)
	default:
		return WrapExit(ExitInternal, err)
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func newDemoHistoryCmd(g *GlobalFlags) *cobra.Command {
	var runsRoot string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Print read-only Preprod demo history (auto-finds demo/runs)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			resolved, err := demo.ResolveRunsRoot(runsRoot)
			if err != nil {
				return WrapExit(ExitUsage, err)
			}
			history, err := demo.ReadHistory(resolved)
			if err != nil {
				return WrapExit(ExitValidation, err)
			}
			if p.Mode == OutputHuman {
				_, err := fmt.Fprint(cmd.OutOrStdout(), demo.FormatHistoryHumanAt(history, resolved, p.Color))
				return err
			}
			msg := "demo history"
			if len(history.TLDs) == 0 {
				msg = "no demo history yet (run a fresh demo first)"
			}
			return p.Success(Result{
				Command:   "demo history",
				Operation: "demo.history",
				Message:   msg,
				Data: map[string]any{
					"runsRoot": resolved,
					"tlds":     history.TLDs,
				},
			})
		},
	}
	cmd.Flags().StringVar(&runsRoot, "runs-root", "", "optional override for demo runs directory (default: auto-detect)")
	return cmd
}
