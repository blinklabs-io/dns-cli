package prereq

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// ContractsOK reports whether path looks like a usable Aiken onchain project.
func ContractsOK(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "aiken.toml")); err != nil {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "validators")); err != nil || !st.IsDir() {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "lib")); err != nil || !st.IsDir() {
		return false
	}
	return true
}

// FindWorkspaceRoot walks up from start looking for dns-contracts or dns-cli go.mod.
func FindWorkspaceRoot(start string) (string, error) {
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = wd
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if ContractsOK(filepath.Join(dir, DNSContractsDirName, OnchainSubdir)) {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, DNSContractsDirName)); err == nil {
			return dir, nil
		}
		// dns-cli module root → workspace is parent
		if raw, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if strings.Contains(string(raw), "module github.com/blinklabs-io/dns-cli") {
				return filepath.Dir(dir), nil
			}
		}
		// demo/ inside dns-cli → go up to dns-cli then parent
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: if start is .../dns-cli/demo, workspace is ../..
	return filepath.Dir(filepath.Dir(abs)), nil
}

// OnchainPath returns workspace/dns-contracts/onchain.
func OnchainPath(workspace string) string {
	return filepath.Join(workspace, DNSContractsDirName, OnchainSubdir)
}

// EnsureDNSContracts ensures dns-contracts/onchain exists, cloning when missing.
// Returns the onchain directory path.
func EnsureDNSContracts(opts Options) (string, error) {
	workspace, err := FindWorkspaceRoot(firstNonEmpty(opts.DemoRoot, opts.StartDir))
	if err != nil {
		return "", err
	}
	onchain := OnchainPath(workspace)
	if ContractsOK(onchain) {
		slog.Info("dns-contracts onchain ready", "path", onchain)
		return onchain, nil
	}

	repoDir := filepath.Join(workspace, DNSContractsDirName)
	fmt.Fprint(opts.out(), opts.theme().Panel("Missing Handshake contracts", []report.KV{
		{Key: "path", Value: onchain},
		{Key: "clone", Value: DNSContractsRepoURL},
	}))

	if opts.SkipInstall {
		printContractsGuide(opts, workspace)
		return "", fmt.Errorf("dns-contracts missing and --skip-install was set")
	}
	if !opts.askYes("Clone dns-contracts now?") {
		printContractsGuide(opts, workspace)
		return "", fmt.Errorf("dns-contracts is required")
	}

	if _, err := os.Stat(repoDir); err == nil {
		// Repo dir exists but onchain incomplete
		return "", fmt.Errorf("dns-contracts exists at %s but onchain/ is incomplete; repair the checkout manually", repoDir)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", err
	}
	slog.Info("Cloning dns-contracts", "url", DNSContractsRepoURL, "dir", repoDir)
	cmd := exec.Command("git", "clone", "--depth", "1", DNSContractsRepoURL, repoDir)
	cmd.Stdout = opts.out()
	cmd.Stderr = opts.errOut()
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone dns-contracts: %w", err)
	}
	if !ContractsOK(onchain) {
		return "", fmt.Errorf("cloned dns-contracts but onchain project incomplete at %s", onchain)
	}
	slog.Info("dns-contracts ready", "path", onchain)
	return onchain, nil
}

// SyncDemoContracts copies onchain Aiken sources into demo/fixtures/contracts.
func SyncDemoContracts(onchainDir, fixturesDir string) error {
	if !ContractsOK(onchainDir) {
		return fmt.Errorf("invalid onchain contracts: %s", onchainDir)
	}
	if err := os.MkdirAll(fixturesDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"aiken.toml", "aiken.lock", "plutus.json"} {
		src := filepath.Join(onchainDir, name)
		dst := filepath.Join(fixturesDir, name)
		if _, err := os.Stat(src); err != nil {
			if name == "plutus.json" {
				continue // optional until aiken build
			}
			return fmt.Errorf("missing %s in onchain: %w", name, err)
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	for _, name := range []string{"validators", "lib"} {
		src := filepath.Join(onchainDir, name)
		dst := filepath.Join(fixturesDir, name)
		_ = os.RemoveAll(dst)
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	slog.Info("Synced demo fixtures/contracts from dns-contracts", "from", onchainDir, "to", fixturesDir)
	return nil
}

// EnsureBlueprintDir ensures an Aiken blueprint directory exists.
// Prefer the requested path when valid. Otherwise reuse an existing
// dns-contracts/onchain checkout (syncing into demo fixtures when that was requested).
// Only prompts to clone when no local contracts exist.
func EnsureBlueprintDir(blueprint string, opts Options) (string, error) {
	if ContractsOK(blueprint) {
		return blueprint, nil
	}

	workspace, err := FindWorkspaceRoot(firstNonEmpty(opts.DemoRoot, opts.StartDir, blueprint))
	if err != nil {
		return "", err
	}
	onchain := OnchainPath(workspace)

	// Existing checkout: use or sync without prompting.
	if ContractsOK(onchain) {
		if blueprint != "" && strings.Contains(filepath.ToSlash(blueprint), "fixtures/contracts") {
			if err := SyncDemoContracts(onchain, blueprint); err != nil {
				return "", err
			}
			return blueprint, nil
		}
		if blueprint != "" && blueprint != onchain {
			slog.Warn("Requested blueprint missing; using dns-contracts onchain", "requested", blueprint, "using", onchain)
		}
		return onchain, nil
	}

	// Need clone (interactive).
	onchain, err = EnsureDNSContracts(opts)
	if err != nil {
		return "", err
	}
	if blueprint != "" && strings.Contains(filepath.ToSlash(blueprint), "fixtures/contracts") {
		if opts.SkipInstall {
			printContractsGuide(opts, workspace)
			return "", fmt.Errorf("blueprint %s missing and --skip-install was set", blueprint)
		}
		if err := SyncDemoContracts(onchain, blueprint); err != nil {
			return "", err
		}
		return blueprint, nil
	}
	return onchain, nil
}

func printContractsGuide(opts Options, workspace string) {
	fmt.Fprint(opts.out(), opts.theme().Guide(
		"Self-serve: clone Handshake contracts",
		"cd "+workspace,
		"git clone "+DNSContractsRepoURL,
		"Then ensure demo fixtures (if using the demo):",
		"copy dns-contracts/onchain/{aiken.toml,aiken.lock,plutus.json,validators,lib}",
		"into dns-cli/demo/fixtures/contracts/",
	))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
