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

	modeFlag := strings.TrimSpace(r.opts.Mode)
	providerFlag := strings.TrimSpace(r.opts.Provider)
	tldFlag := strings.TrimSpace(r.opts.TLD)
	sldFlag := strings.TrimSpace(r.opts.SLD)
	modeEnv := strings.TrimSpace(os.Getenv("DEMO_MODE"))
	providerEnv := strings.TrimSpace(os.Getenv("DEMO_PROVIDER"))

	mode := firstNonEmpty(modeFlag, modeEnv, "fresh")
	provider := firstNonEmpty(providerFlag, providerEnv, "blockfrost")
	sld := firstNonEmpty(sldFlag, priorSLD, "www")
	tld := firstNonEmpty(tldFlag, priorTLD, defaultTLDName())

	logLevel := strings.TrimSpace(r.opts.LogLevel)
	if logLevel == "" && envTruthy("DEMO_EXTENSIVE_LOGGING") {
		logLevel = "extensive"
	}
	if logLevel == "" {
		logLevel = strings.TrimSpace(os.Getenv("DEMO_LOG_LEVEL"))
	}

	needAsk := modeFlag == "" && modeEnv == ""
	if mode != "existing" {
		if providerFlag == "" && providerEnv == "" {
			needAsk = true
		}
		if tldFlag == "" {
			needAsk = true
		}
		if sldFlag == "" {
			needAsk = true
		}
	}
	if logLevel == "" {
		needAsk = true
	}
	if !r.opts.SkipInstallSet {
		needAsk = true
	}
	if !r.opts.NoClipboardSet && mode != "existing" {
		needAsk = true
	}
	assumeYes := r.opts.Yes || envTruthy("DEMO_ASSUME_YES")
	if needAsk && !assumeYes {
		fmt.Fprintln(r.stdout, "══ Demo run options ══")
	}

	if modeFlag == "" && modeEnv == "" {
		mode = r.askSetting("Mode", mode, []string{"fresh", "existing"})
	}
	if mode != "existing" && providerFlag == "" && providerEnv == "" {
		provider = r.askSetting("Provider", provider, []string{"blockfrost", "utxorpc"})
	}
	if mode == "existing" {
		r.mode = "existing"
		r.provider = provider
		r.tld = tld
		r.sld = sld
		if err := r.resolveAuxOptions(logLevel, true); err != nil {
			return err
		}
		if needAsk && !assumeYes {
			fmt.Fprintln(r.stdout, "════════════════════════")
		}
		return nil
	}
	if tldFlag == "" {
		tld = r.askSetting("TLD (blank keeps default)", tld, nil)
	}
	if sldFlag == "" {
		sld = r.askSetting("SLD", sld, nil)
	}
	if err := r.resolveAuxOptions(logLevel, false); err != nil {
		return err
	}
	if needAsk && !assumeYes {
		fmt.Fprintln(r.stdout, "════════════════════════")
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
	slog.Info("Demo settings", "mode", r.mode, "provider", r.provider, "tld", r.tld, "sld", r.sld,
		"logLevel", r.opts.LogLevel, "skipInstall", r.opts.SkipInstall, "noClipboard", r.opts.NoClipboard)
	return nil
}

func (r *Runner) resolveAuxOptions(logLevel string, existingMode bool) error {
	assumeYes := r.opts.Yes || envTruthy("DEMO_ASSUME_YES")
	if logLevel == "" {
		logLevel = r.askSetting("Log level", "normal", []string{"quiet", "normal", "extensive"})
	}
	logLevel = strings.ToLower(strings.TrimSpace(logLevel))
	switch logLevel {
	case "quiet", "normal", "extensive":
		r.opts.LogLevel = logLevel
	default:
		return fmt.Errorf("invalid log level %q (want quiet|normal|extensive)", logLevel)
	}

	if !r.opts.SkipInstallSet {
		// Default No; --yes keeps that default (does not enable skip-install).
		if assumeYes {
			r.opts.SkipInstall = false
		} else {
			r.opts.SkipInstall = r.prompt.ConfirmYes("Skip tool installs / credential writes (guides only)?")
		}
		r.opts.SkipInstallSet = true
	}
	if !existingMode && !r.opts.NoClipboardSet {
		copyClip := r.prompt.ConfirmDefault("Copy bootstrap faucet address to clipboard?")
		r.opts.NoClipboard = !copyClip
		r.opts.NoClipboardSet = true
	}
	return nil
}

func (r *Runner) askSetting(name, def string, allowed []string) string {
	return r.prompt.AskChoice(name, def, allowed)
}
