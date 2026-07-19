package demo

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

const (
	blockfrostDashboardURL = "https://blockfrost.io/dashboard"
	blockfrostPreprodURL   = "https://cardano-preprod.blockfrost.io/api/v0"
	utxorpcDocsURL         = "https://docs.demeter.run/cardano"
)

func (r *Runner) ensureCredentials() error {
	switch r.provider {
	case "blockfrost":
		return r.ensureBlockfrostCredentials()
	case "utxorpc":
		return r.ensureUTxORPCCredentials()
	default:
		return fmt.Errorf("unsupported provider %q", r.provider)
	}
}

func (r *Runner) ensureBlockfrostCredentials() error {
	projectEnv := "DNS_CLI_BLOCKFROST_PROJECT_ID"
	urlEnv := "DNS_CLI_BLOCKFROST_URL"
	projectID := strings.TrimSpace(os.Getenv(projectEnv))
	customURL := strings.TrimSpace(os.Getenv(urlEnv))
	endpoint := blockfrostPreprodURL
	endpointSrc := "default"
	if customURL != "" {
		endpoint = customURL
		endpointSrc = urlEnv
	}

	fmt.Fprint(r.stdout, r.theme().Panel("Provider credentials · blockfrost", []report.KV{
		{Key: "dashboard", Value: blockfrostDashboardURL},
		{Key: "endpoint", Value: endpoint},
		{Key: "endpointSrc", Value: endpointSrc},
		{Key: "projectIdEnv", Value: projectEnv},
		{Key: "projectId", Value: maskSecret(projectID)},
		{Key: "envFile", Value: r.paths.EnvFile},
	}))

	if err := r.promptSecretVar(projectEnv, "Blockfrost Preprod project id", projectID, true); err != nil {
		return err
	}
	// Optional custom base URL — blank keeps the Preprod default.
	if err := r.promptSecretVar(urlEnv, "Blockfrost base URL (blank = Preprod default)", customURL, false); err != nil {
		return err
	}
	return nil
}

func (r *Runner) ensureUTxORPCCredentials() error {
	urlEnv := "DNS_CLI_UTXORPC_URL"
	headersEnv := "DNS_CLI_UTXORPC_HEADERS"
	keyEnv := "DMTR_API_KEY"
	url := strings.TrimSpace(os.Getenv(urlEnv))
	headers := strings.TrimSpace(os.Getenv(headersEnv))
	apiKey := strings.TrimSpace(os.Getenv(keyEnv))

	fmt.Fprint(r.stdout, r.theme().Panel("Provider credentials · utxorpc", []report.KV{
		{Key: "docs", Value: utxorpcDocsURL},
		{Key: "urlEnv", Value: urlEnv},
		{Key: "url", Value: displayOrUnset(url)},
		{Key: "headersEnv", Value: headersEnv + " (optional)"},
		{Key: "headers", Value: maskSecret(headers)},
		{Key: "apiKeyEnv", Value: keyEnv + " (optional)"},
		{Key: "apiKey", Value: maskSecret(apiKey)},
		{Key: "envFile", Value: r.paths.EnvFile},
	}))

	if err := r.promptSecretVar(urlEnv, "UTxO RPC URL (e.g. https://…demeter.run)", url, true); err != nil {
		return err
	}
	if headers == "" && apiKey == "" {
		if err := r.promptSecretVar(keyEnv, "DMTR_API_KEY (optional, blank to skip)", "", false); err != nil {
			return err
		}
	} else if apiKey != "" {
		if err := r.promptSecretVar(keyEnv, "DMTR_API_KEY", apiKey, false); err != nil {
			return err
		}
	} else if headers != "" {
		if err := r.promptSecretVar(headersEnv, "DNS_CLI_UTXORPC_HEADERS", headers, false); err != nil {
			return err
		}
	}
	return nil
}

// promptSecretVar shows the current (masked) value and offers keep / replace.
// required=true fails when the final value is empty.
func (r *Runner) promptSecretVar(envKey, label, current string, required bool) error {
	assumeYes := r.opts.Yes || envTruthy("DEMO_ASSUME_YES")
	current = strings.TrimSpace(current)

	if current != "" {
		if assumeYes {
			slog.Info("Keeping existing credential", "env", envKey)
			return nil
		}
		keep := r.prompt.ConfirmDefault(fmt.Sprintf("Keep existing %s (%s)?", envKey, maskSecret(current)))
		if keep {
			return nil
		}
	} else if assumeYes {
		if required {
			return fmt.Errorf("%s missing (set it in %s or unset --yes to enter interactively)", envKey, r.paths.EnvFile)
		}
		return nil
	}

	if r.opts.SkipInstall {
		if required && current == "" {
			return fmt.Errorf("%s missing and skip-install was set (set it in %s or re-run without skip)", envKey, r.paths.EnvFile)
		}
		// Guides-only: do not write new secrets; keep whatever is already loaded.
		if current != "" {
			fmt.Fprint(r.stdout, r.theme().Note(fmt.Sprintf("skip-install set — keeping existing %s", envKey)))
		}
		return nil
	}

	defHint := ""
	if current != "" {
		defHint = "leave blank to keep"
	} else if !required {
		defHint = "optional, blank to skip"
	}
	prompt := label
	if defHint != "" {
		prompt = fmt.Sprintf("%s (%s)", label, defHint)
	}
	val, err := r.readCredential(prompt)
	if err != nil {
		return err
	}
	val = strings.TrimSpace(val)
	if val == "" {
		if current != "" {
			return nil
		}
		if required {
			return fmt.Errorf("%s is required", envKey)
		}
		return nil
	}
	return SaveEnvVar(r.paths.EnvFile, envKey, val)
}

func maskSecret(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(not set)"
	}
	if len(v) <= 8 {
		return v[:1] + strings.Repeat("•", len(v)-1)
	}
	return v[:4] + "…" + v[len(v)-4:]
}

func displayOrUnset(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(not set)"
	}
	return v
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
	builtinTLD := defaultTLDName()
	builtinSLD := "www"
	sld := firstNonEmpty(sldFlag, priorSLD, builtinSLD)
	tld := firstNonEmpty(tldFlag, priorTLD, builtinTLD)

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
		fmt.Fprint(r.stdout, r.theme().SectionOpen("Demo run options"))
	}

	if modeFlag == "" && modeEnv == "" {
		mode = r.askSetting("Mode", mode, []string{"fresh", "existing"})
	}
	if mode != "existing" && providerFlag == "" && providerEnv == "" {
		provider = r.askSetting("Provider", provider, []string{"blockfrost", "utxorpc"})
	}
	if mode == "existing" {
		// Provider/TLD/SLD come from the resume catalog selection, not prompts.
		r.mode = "existing"
		r.provider = ""
		r.tld = ""
		r.sld = ""
		if err := r.resolveAuxOptions(logLevel, true); err != nil {
			return err
		}
		if needAsk && !assumeYes {
			fmt.Fprint(r.stdout, r.theme().SectionClose())
		}
		slog.Info("Demo settings", "mode", r.mode,
			"logLevel", r.opts.LogLevel, "skipInstall", r.opts.SkipInstall)
		return nil
	}
	if tldFlag == "" {
		tld = r.askSetting(settingPrompt("TLD", tld, priorTLD, builtinTLD), tld, nil)
	}
	if sldFlag == "" {
		sld = r.askSetting(settingPrompt("SLD", sld, priorSLD, builtinSLD), sld, nil)
	}
	if err := r.resolveAuxOptions(logLevel, false); err != nil {
		return err
	}
	if needAsk && !assumeYes {
		fmt.Fprint(r.stdout, r.theme().SectionClose())
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

func settingPrompt(name, value, prior, builtin string) string {
	src := "default"
	switch {
	case prior != "" && value == prior:
		src = "last run"
	case value == builtin:
		src = "default"
	default:
		src = "default"
	}
	return fmt.Sprintf("%s · %s", name, src)
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

func (r *Runner) readCredential(prompt string) (string, error) {
	// Reuse the prompter's bufio.Reader when available so confirm+secret share one stdin cursor.
	if sp, ok := r.prompt.(*stdPrompter); ok {
		return ReadSecret(sp.in, r.stdout, prompt)
	}
	return ReadSecret(r.stdin, r.stdout, prompt)
}
