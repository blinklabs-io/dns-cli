package demo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	MinBootstrapLovelace int64 = 150000000
	PollSeconds                = 20
	FaucetURL                  = "https://docs.cardano.org/cardano-testnets/tools/faucet/"
	SchemaVersionRun           = 1
	NetworkName                = "preprod"
)

var walletActors = []string{"bootstrap", "registrar", "tld-owner", "sld-owner"}

// Options configures demo run.
type Options struct {
	DemoRoot    string
	RunsRoot    string // optional override; default DemoRoot/runs
	Mode        string // fresh|existing
	Provider    string // blockfrost|utxorpc
	TLD         string
	SLD         string
	Yes         bool
	SkipInstall bool
	// SkipInstallSet is true when --skip-install was explicitly passed on the CLI.
	SkipInstallSet bool
	NoClipboard    bool
	// NoClipboardSet is true when --no-clipboard was explicitly passed on the CLI.
	NoClipboardSet bool
	NoColor        bool
	LogLevel       string // quiet|normal|extensive
	ContractRev    string
	// ApplyVerbose, when set, is called after log level is resolved (prompt or flag).
	// Used by the CLI to reconfigure slog when -v was not explicitly set.
	ApplyVerbose func(verbose int) error

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Paths holds resolved demo filesystem locations.
type Paths struct {
	DemoRoot        string
	RunsRoot        string
	SharedDir       string
	StatesDir       string
	EnvFile         string
	RecordsTemplate string
	LegacyRuntime   string
	TldDir          string
	SldRunDir       string
	TldStateFile    string
	SldStateFile    string
	RunJSONFile     string
	RecordsFile     string
	BootstrapConfig string
	BoundConfig     string
	DeploymentJSON  string
	ProofBundle     string
	TldArtifacts    string
	SldArtifacts    string
	WalletsDir      string
}

func resolvePaths(demoRoot, runsRoot string) (Paths, error) {
	demoRoot, err := filepath.Abs(demoRoot)
	if err != nil {
		return Paths{}, err
	}
	if runsRoot == "" {
		runsRoot = filepath.Join(demoRoot, "runs")
	} else {
		runsRoot, err = filepath.Abs(runsRoot)
		if err != nil {
			return Paths{}, err
		}
	}
	shared := filepath.Join(runsRoot, "shared")
	return Paths{
		DemoRoot:        demoRoot,
		RunsRoot:        runsRoot,
		SharedDir:       shared,
		StatesDir:       filepath.Join(runsRoot, "states"),
		EnvFile:         filepath.Join(shared, ".env"),
		RecordsTemplate: filepath.Join(demoRoot, "config", "records.json"),
		LegacyRuntime:   filepath.Join(demoRoot, "runtime"),
		WalletsDir:      filepath.Join(shared, "wallets"),
	}, nil
}

func (p *Paths) applyTLD(tld, provider string) {
	p.TldDir = filepath.Join(p.RunsRoot, tld)
	p.TldStateFile = filepath.Join(p.TldDir, "state.json")
	p.BootstrapConfig = filepath.Join(p.TldDir, "config", "bootstrap.json")
	p.BoundConfig = filepath.Join(p.TldDir, "config", provider+".json")
	p.DeploymentJSON = filepath.Join(p.TldDir, "contracts", "deployment.json")
	p.ProofBundle = filepath.Join(p.TldDir, "proofs", "proof-bundle.json")
	p.TldArtifacts = filepath.Join(p.TldDir, "artifacts")
}

func (p *Paths) applySLDRun(sld, runID string) {
	p.SldRunDir = filepath.Join(p.TldDir, sld, runID)
	p.SldStateFile = filepath.Join(p.SldRunDir, "state.json")
	p.RunJSONFile = filepath.Join(p.SldRunDir, "run.json")
	p.RecordsFile = filepath.Join(p.SldRunDir, "records.json")
	p.SldArtifacts = filepath.Join(p.SldRunDir, "artifacts")
}

func defaultTLDName() string {
	return fmt.Sprintf("demo-%s", time.Now().Format("20060102150405"))
}

func newRunID() string {
	return time.Now().Format("20060102-150405")
}

func mustWriter(w io.Writer) io.Writer {
	if w == nil {
		return os.Stdout
	}
	return w
}

func mustReader(r io.Reader) io.Reader {
	if r == nil {
		return os.Stdin
	}
	return r
}
