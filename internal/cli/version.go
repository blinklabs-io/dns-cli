package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Build metadata — overridden via -ldflags at release time.
var (
	Version          = "dev"
	GitCommit        = "unknown"
	BuildDate        = "unknown"
	ContractRevision = "unknown"
)

// VersionInfo holds runtime and dependency versions.
type VersionInfo struct {
	Version          string `json:"version"`
	GitCommit        string `json:"gitCommit"`
	BuildDate        string `json:"buildDate"`
	GoVersion        string `json:"goVersion"`
	ApolloRevision   string `json:"apolloRevision"`
	ContractRevision string `json:"contractRevision"`
}

// ResolveVersion gathers version metadata from ldflags and module build info.
func ResolveVersion() VersionInfo {
	info := VersionInfo{
		Version:          Version,
		GitCommit:        GitCommit,
		BuildDate:        BuildDate,
		GoVersion:        runtime.Version(),
		ApolloRevision:   "unknown",
		ContractRevision: ContractRevision,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/Salvionied/apollo/v2" {
				info.ApolloRevision = d.Version
				if d.Replace != nil && d.Replace.Version != "" {
					info.ApolloRevision = d.Replace.Version
				}
			}
		}
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && info.GitCommit == "unknown" {
				info.GitCommit = s.Value
			}
		}
	}
	return info
}

func formatVersionHuman(v VersionInfo) string {
	return fmt.Sprintf(
		"dns-cli %s\ncommit: %s\nbuilt: %s\ngo: %s\napollo: %s\ncontracts: %s\n",
		v.Version, v.GitCommit, v.BuildDate, v.GoVersion, v.ApolloRevision, v.ContractRevision,
	)
}
