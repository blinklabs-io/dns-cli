package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindDemoRoot locates the demo directory by walking up from start
// (defaults to the process working directory).
//
// Candidates checked at each level:
//   - <dir>           (cwd is already demo/)
//   - <dir>/demo      (cwd is dns-cli/ or workspace)
//
// A candidate is accepted when it contains config/, fixtures/, and runs/ directories.
func FindDemoRoot(start string) (string, error) {
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
	seen := map[string]struct{}{}
	for {
		if _, ok := seen[dir]; ok {
			break
		}
		seen[dir] = struct{}{}
		for _, cand := range []string{
			dir,
			filepath.Join(dir, "demo"),
		} {
			if looksLikeDemoRoot(cand) {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find demo/ (searched upward from %s); pass --demo-root or run from dns-cli/", abs)
}

func looksLikeDemoRoot(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	for _, rel := range []string{"config", "fixtures", "runs"} {
		st, err := os.Stat(filepath.Join(path, rel))
		if err != nil || !st.IsDir() {
			return false
		}
	}
	return true
}

// ResolveDemoRoot returns explicit if set, otherwise FindDemoRoot("").
func ResolveDemoRoot(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return FindDemoRoot("")
}
