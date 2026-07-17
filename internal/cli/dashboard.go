package cli

import (
	"github.com/blinklabs-io/dns-cli/internal/tui"
	"github.com/spf13/cobra"
)

func newDashboardCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Interactive operator dashboard (Bubble Tea)",
		Long:  "Opens a Layout A TUI with identity header, actions, Huh forms for ceremony/bootstrap, status wall, and activity log. Prefer --config; offline actions (wallet create, proof, system prepare/bind) work without it.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v := ResolveVersion()
			opts := tui.DashboardOpts{
				ConfigPath:       g.ConfigPath,
				Network:          g.Network,
				Provider:         g.Provider,
				ContractRevision: ContractRevision,
				Timeout:          g.Timeout,
				Version: tui.VersionInfo{
					Version:          v.Version,
					GitCommit:        v.GitCommit,
					BuildDate:        v.BuildDate,
					GoVersion:        v.GoVersion,
					ApolloRevision:   v.ApolloRevision,
					ContractRevision: v.ContractRevision,
				},
			}
			return RunDashboard(opts)
		},
	}
	return cmd
}

// RunDashboard is separated for tests to stub if needed.
func RunDashboard(opts tui.DashboardOpts) error {
	return tui.RunDashboard(opts)
}
