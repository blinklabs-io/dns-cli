package tui

import (
	"fmt"
	"strings"
)

// VersionInfo is passed from the CLI so tui does not import cli.
type VersionInfo struct {
	Version          string
	GitCommit        string
	BuildDate        string
	GoVersion        string
	ApolloRevision   string
	ContractRevision string
}

func shortCommit(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func renderIdentity(v VersionInfo, configPath, network, provider, health, actor, waitBadge string) string {
	cfg := configPath
	if strings.TrimSpace(cfg) == "" {
		cfg = "no config"
	}
	if network == "" {
		network = "—"
	}
	if provider == "" {
		provider = "—"
	}
	if health == "" {
		health = "idle"
	}
	if actor == "" {
		actor = "—"
	}
	if waitBadge == "" {
		waitBadge = "idle"
	}
	line := fmt.Sprintf(
		"dns-cli %s · commit %s · built %s | config: %s | %s · %s · %s | actor: %s | wait: %s",
		v.Version,
		shortCommit(v.GitCommit),
		v.BuildDate,
		cfg,
		network,
		provider,
		health,
		actor,
		waitBadge,
	)
	return styleHeader.Render(line)
}

func renderIdentityDetail(v VersionInfo) string {
	return strings.Join([]string{
		styleTitle.Render("dns-cli identity"),
		fmt.Sprintf("version:   %s", v.Version),
		fmt.Sprintf("commit:    %s", v.GitCommit),
		fmt.Sprintf("built:     %s", v.BuildDate),
		fmt.Sprintf("go:        %s", v.GoVersion),
		fmt.Sprintf("apollo:    %s", v.ApolloRevision),
		fmt.Sprintf("contracts: %s", v.ContractRevision),
		"",
		styleDim.Render("press i or esc to close"),
	}, "\n")
}
