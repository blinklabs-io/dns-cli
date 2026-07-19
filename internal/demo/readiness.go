package demo

import (
	"fmt"
	"os"

	"github.com/blinklabs-io/dns-cli/internal/provider"
	"github.com/blinklabs-io/dns-cli/internal/report"
)

func formatProviderReadiness(r provider.Readiness, color bool) string {
	th := report.New(color)
	rows := []report.KV{
		{Key: "provider", Value: dash(r.Provider)},
		{Key: "network", Value: dash(r.Network)},
		{Key: "endpoint", Value: dash(r.EndpointHost)},
		{Key: "endpointSrc", Value: dash(r.EndpointSource)},
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

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (r *Runner) checkProviderReadiness(cfgPath string) error {
	eff, err := r.loadConfig(cfgPath)
	if err != nil {
		return err
	}
	ready, err := provider.CheckReadiness(r.ctx, eff)
	fmt.Fprint(r.stdout, formatProviderReadiness(ready, !r.opts.NoColor))
	return err
}

func (r *Runner) readinessConfigPath(stage ResumeStage) string {
	switch stage {
	case StageFund, StageDeploy, StageBind:
		return r.paths.BootstrapConfig
	default:
		return r.paths.BoundConfig
	}
}

// ensureReadinessConfig creates bind/bootstrap prerequisites needed before readiness.
func (r *Runner) ensureReadinessConfig(stage ResumeStage) error {
	switch stage {
	case StageFund, StageDeploy, StageBind:
		if _, err := os.Stat(r.paths.BootstrapConfig); err != nil {
			if err := r.writeBootstrapConfig(); err != nil {
				return err
			}
		}
		return nil
	default:
		return r.ensureBoundConfig()
	}
}
