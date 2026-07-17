package prereq

import (
	"fmt"
	"io"
	"os"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

const (
	DNSContractsRepoURL = "https://github.com/blinklabs-io/dns-contracts.git"
	DNSContractsDirName = "dns-contracts"
	OnchainSubdir       = "onchain"
	MinAikenVersion     = "1.1.19"
)

// Options controls interactive prerequisite repair.
type Options struct {
	// StartDir is a path to walk up from when locating the workspace (cwd or demo-root).
	StartDir string
	// DemoRoot is the demo directory to validate/scaffold (optional for contracts-only).
	DemoRoot string
	// SkipInstall refuses clone/copy/write; prints guides instead.
	SkipInstall bool
	// AssumeYes auto-confirms repair prompts (not submission confirms).
	AssumeYes bool
	// NoColor disables ANSI in human prerequisite messages.
	NoColor bool
	// ConfirmYes asks [y/N]; if nil, AssumeYes decides.
	ConfirmYes func(prompt string) bool
	Stdout     io.Writer
	Stderr     io.Writer
}

func (o Options) out() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return os.Stdout
}

func (o Options) errOut() io.Writer {
	if o.Stderr != nil {
		return o.Stderr
	}
	return os.Stderr
}

func (o Options) theme() *report.Theme {
	return report.New(!o.NoColor)
}

func (o Options) askYes(prompt string) bool {
	if o.AssumeYes {
		fmt.Fprintf(o.out(), "%s [y/N]: y (assume-yes)\n", prompt)
		return true
	}
	if o.ConfirmYes != nil {
		return o.ConfirmYes(prompt)
	}
	return false
}
