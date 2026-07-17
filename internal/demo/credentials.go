package demo

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

func (r *Runner) ensureCredentials() error {
	switch r.provider {
	case "blockfrost":
		if strings.TrimSpace(os.Getenv("DNS_CLI_BLOCKFROST_PROJECT_ID")) != "" {
			return nil
		}
		fmt.Fprint(r.stdout, r.theme().Panel("Credentials required", []report.KV{
			{Key: "provider", Value: "blockfrost"},
			{Key: "env", Value: "DNS_CLI_BLOCKFROST_PROJECT_ID"},
			{Key: "dashboard", Value: "https://blockfrost.io/dashboard"},
		}))
		if r.opts.SkipInstall {
			return fmt.Errorf("DNS_CLI_BLOCKFROST_PROJECT_ID missing and --skip-install was set")
		}
		if !r.prompt.ConfirmYes("Enter Blockfrost project id now and save to runs/shared/.env?") {
			return fmt.Errorf("DNS_CLI_BLOCKFROST_PROJECT_ID is required")
		}
		val, err := ReadSecret(r.stdin, r.stdout, "Blockfrost Preprod project id")
		if err != nil {
			return err
		}
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("empty Blockfrost project id")
		}
		return SaveEnvVar(r.paths.EnvFile, "DNS_CLI_BLOCKFROST_PROJECT_ID", strings.TrimSpace(val))
	case "utxorpc":
		if strings.TrimSpace(os.Getenv("DNS_CLI_UTXORPC_URL")) == "" {
			fmt.Fprint(r.stdout, r.theme().Panel("Credentials required", []report.KV{
				{Key: "provider", Value: "utxorpc"},
				{Key: "env", Value: "DNS_CLI_UTXORPC_URL"},
			}))
			if r.opts.SkipInstall {
				return fmt.Errorf("DNS_CLI_UTXORPC_URL missing and --skip-install was set")
			}
			if !r.prompt.ConfirmYes("Enter UTxO RPC URL now and save to runs/shared/.env?") {
				return fmt.Errorf("DNS_CLI_UTXORPC_URL is required")
			}
			val, err := ReadSecret(r.stdin, r.stdout, "DNS_CLI_UTXORPC_URL")
			if err != nil {
				return err
			}
			if strings.TrimSpace(val) == "" {
				return fmt.Errorf("empty UTxO RPC URL")
			}
			if err := SaveEnvVar(r.paths.EnvFile, "DNS_CLI_UTXORPC_URL", strings.TrimSpace(val)); err != nil {
				return err
			}
		}
		if os.Getenv("DMTR_API_KEY") == "" && os.Getenv("DNS_CLI_UTXORPC_HEADERS") == "" {
			if !r.opts.SkipInstall && r.prompt.ConfirmYes("Also set optional DMTR_API_KEY (Demeter)?") {
				val, err := ReadSecret(r.stdin, r.stdout, "DMTR_API_KEY")
				if err != nil {
					return err
				}
				if strings.TrimSpace(val) != "" {
					if err := SaveEnvVar(r.paths.EnvFile, "DMTR_API_KEY", strings.TrimSpace(val)); err != nil {
						return err
					}
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported provider %q", r.provider)
	}
}

func (r *Runner) resolveSettings() error {
	priorTLD, priorSLD := runsDefaults(r.paths.RunsRoot)

	mode := firstNonEmpty(r.opts.Mode, os.Getenv("DEMO_MODE"), "fresh")
	provider := firstNonEmpty(r.opts.Provider, os.Getenv("DEMO_PROVIDER"), "blockfrost")
	sld := firstNonEmpty(r.opts.SLD, priorSLD, "www")
	tld := firstNonEmpty(r.opts.TLD, priorTLD, defaultTLDName())

	if r.opts.Mode == "" {
		mode = r.askSetting("mode", mode, []string{"fresh", "existing"})
	}
	if r.opts.Provider == "" && mode != "existing" {
		provider = r.askSetting("provider", provider, []string{"blockfrost", "utxorpc"})
	}
	if mode == "existing" {
		r.mode = "existing"
		r.provider = provider
		r.tld = tld
		r.sld = sld
		return nil
	}
	if r.opts.TLD == "" {
		tld = r.askSetting("tld", tld, nil)
	}
	if r.opts.SLD == "" {
		sld = r.askSetting("sld", sld, nil)
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	provider = strings.ToLower(strings.TrimSpace(provider))
	if mode != "fresh" && mode != "existing" {
		return fmt.Errorf("invalid mode %q", mode)
	}
	if provider != "blockfrost" && provider != "utxorpc" {
		return fmt.Errorf("invalid provider %q", provider)
	}
	if strings.TrimSpace(tld) == "" || strings.TrimSpace(sld) == "" {
		return fmt.Errorf("tld and sld are required")
	}
	r.mode = mode
	r.provider = provider
	r.tld = strings.TrimSpace(tld)
	r.sld = strings.TrimSpace(sld)
	slog.Info("Demo settings", "mode", r.mode, "provider", r.provider, "tld", r.tld, "sld", r.sld)
	return nil
}

func (r *Runner) askSetting(name, def string, allowed []string) string {
	if r.prompt.ConfirmDefault(fmt.Sprintf("Use this %s? (%s)", name, def)) {
		return def
	}
	val := r.prompt.AskString(fmt.Sprintf("Enter %s", name), def)
	if len(allowed) > 0 {
		ok := false
		for _, a := range allowed {
			if strings.EqualFold(val, a) {
				ok = true
				val = a
				break
			}
		}
		if !ok {
			slog.Warn("invalid value; keeping default", "name", name, "got", val, "default", def)
			return def
		}
	}
	return val
}
