package prereq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var reAikenVersion = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// AikenInfo describes a discovered aiken binary.
type AikenInfo struct {
	Path    string
	Version string
	OK      bool // meets MinAikenVersion
}

// FindAiken locates aiken on PATH and common install dirs.
func FindAiken(extraDirs ...string) (AikenInfo, error) {
	candidates := []string{}
	if p, err := exec.LookPath("aiken"); err == nil {
		candidates = append(candidates, p)
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".aiken", "bin", aikenBinName()))
	}
	for _, d := range extraDirs {
		if d == "" {
			continue
		}
		candidates = append(candidates, filepath.Join(d, aikenBinName()))
	}
	var lastErr error
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		ver, err := aikenVersion(c)
		if err != nil {
			lastErr = err
			continue
		}
		ok, err := versionAtLeast(ver, MinAikenVersion)
		if err != nil {
			lastErr = err
			continue
		}
		return AikenInfo{Path: c, Version: ver, OK: ok}, nil
	}
	if lastErr != nil {
		return AikenInfo{}, lastErr
	}
	return AikenInfo{}, fmt.Errorf("aiken not found (need >= %s)", MinAikenVersion)
}

// EnsureAiken checks Aiken and optionally offers install guidance / installer.
func EnsureAiken(opts Options, toolsDir string) (AikenInfo, error) {
	info, err := FindAiken(toolsDir)
	if err == nil && info.OK {
		return info, nil
	}
	th := opts.theme()
	if err == nil && !info.OK {
		fmt.Fprint(opts.out(), th.Warn(fmt.Sprintf("aiken %s is below required minimum %s (found %s)", info.Version, MinAikenVersion, info.Path)))
	} else {
		fmt.Fprint(opts.out(), th.Warn(fmt.Sprintf("aiken >= %s is required for system prepare", MinAikenVersion)))
	}
	printAikenGuide(opts)
	if opts.SkipInstall {
		return AikenInfo{}, fmt.Errorf("aiken missing/too-old and --skip-install was set")
	}
	if !opts.askYes("Download/install Aiken now?") {
		return AikenInfo{}, fmt.Errorf("aiken >= %s is required", MinAikenVersion)
	}
	if err := installAiken(opts); err != nil {
		return AikenInfo{}, err
	}
	info, err = FindAiken(toolsDir)
	if err != nil {
		return AikenInfo{}, err
	}
	if !info.OK {
		return AikenInfo{}, fmt.Errorf("aiken %s still below %s after install", info.Version, MinAikenVersion)
	}
	return info, nil
}

func installAiken(opts Options) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Official Windows installer
		ps := `irm https://windows.aiken-lang.org | iex`
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	default:
		cmd = exec.Command("bash", "-lc", `curl --proto "=https" --tlsv1.2 -LsSf https://install.aiken-lang.org | sh && command -v aikup >/dev/null && aikup || true`)
	}
	cmd.Stdout = opts.out()
	cmd.Stderr = opts.errOut()
	if err := cmd.Run(); err != nil {
		fmt.Fprint(opts.out(), opts.theme().Warn("Installer failed; try: cargo install aiken"))
		return fmt.Errorf("aiken install: %w", err)
	}
	return nil
}

func printAikenGuide(opts Options) {
	lines := []string{
		"Docs: https://aiken-lang.org/installation-instructions",
	}
	if runtime.GOOS == "windows" {
		lines = append(lines, `PowerShell: irm https://windows.aiken-lang.org | iex`)
	} else {
		lines = append(lines,
			`curl --proto "=https" --tlsv1.2 -LsSf https://install.aiken-lang.org | sh`,
			"then: aikup",
		)
	}
	lines = append(lines, "Fallback: cargo install aiken")
	fmt.Fprint(opts.out(), opts.theme().Guide("Self-serve: install Aiken >= "+MinAikenVersion, lines...))
}

func aikenBinName() string {
	if runtime.GOOS == "windows" {
		return "aiken.exe"
	}
	return "aiken"
}

func aikenVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("aiken --version: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	m := reAikenVersion.FindStringSubmatch(string(out))
	if len(m) != 4 {
		return "", fmt.Errorf("could not parse aiken version from: %s (need >= %s)", strings.TrimSpace(string(out)), MinAikenVersion)
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3]), nil
}

func versionAtLeast(got, want string) (bool, error) {
	g, err := parseSemver(got)
	if err != nil {
		return false, err
	}
	w, err := parseSemver(want)
	if err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		if g[i] > w[i] {
			return true, nil
		}
		if g[i] < w[i] {
			return false, nil
		}
	}
	return true, nil
}

func parseSemver(s string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid semver %q", s)
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, err
		}
		out[i] = n
	}
	return out, nil
}
