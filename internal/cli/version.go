package cli

import (
	"runtime"
	"runtime/debug"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// Build metadata — overridden via -ldflags at release time.
var (
	Version          = "1.0.0"
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
			switch s.Key {
			case "vcs.revision":
				if info.GitCommit == "unknown" || info.GitCommit == "" {
					info.GitCommit = s.Value
				}
			case "vcs.time":
				if info.BuildDate == "unknown" || info.BuildDate == "" {
					info.BuildDate = s.Value
				}
			}
		}
	}
	return info
}

func formatVersionHuman(v VersionInfo, color bool) string {
	th := report.New(color)
	return th.Panel("dns-cli "+v.Version, []report.KV{
		{Key: "commit", Value: v.GitCommit},
		{Key: "built", Value: v.BuildDate},
		{Key: "go", Value: v.GoVersion},
		{Key: "apollo", Value: v.ApolloRevision},
		{Key: "contracts", Value: v.ContractRevision},
	})
}
