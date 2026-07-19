package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/report"
	"github.com/spf13/cobra"
)

// loadReadyEffective loads config and blocks until provider readiness succeeds.
// The human readiness panel is written to stderr so JSON stdout stays clean.
func loadReadyEffective(cmd *cobra.Command, g *GlobalFlags) (*config.Effective, error) {
	eff, err := loadEffective(g)
	if err != nil {
		return nil, WrapExit(ExitConfig, err)
	}
	ctx := cmd.Context()
	if g.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.Timeout)
		defer cancel()
	}
	r, err := provider.CheckReadiness(ctx, eff)
	color := !g.NoColor && os.Getenv("NO_COLOR") == ""
	fmt.Fprint(cmd.ErrOrStderr(), formatReadinessHuman(r, color))
	if err != nil {
		if provider.IsConfigError(err) {
			return nil, WrapExit(ExitConfig, err)
		}
		return nil, WrapExit(ExitProvider, err)
	}
	return eff, nil
}

func formatReadinessHuman(r provider.Readiness, color bool) string {
	th := report.New(color)
	rows := []report.KV{
		{Key: "provider", Value: emptyDash(r.Provider)},
		{Key: "network", Value: emptyDash(r.Network)},
		{Key: "endpoint", Value: emptyDash(r.EndpointHost)},
		{Key: "endpointSrc", Value: emptyDash(r.EndpointSource)},
	}
	for _, c := range r.Credentials {
		status := "optional"
		switch {
		case c.Present:
			status = "present"
		case c.Required:
			status = "missing"
		}
		rows = append(rows, report.KV{Key: c.Name, Value: status})
	}
	health := "failed"
	if r.Healthy {
		health = "ready"
	}
	rows = append(rows, report.KV{Key: "health", Value: health})
	return th.Panel("Provider readiness", rows) + "\n"
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
