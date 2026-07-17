package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindRunsRoot locates the demo runs directory by walking up from start
// (defaults to the process working directory).
//
// Candidates checked at each level:
//   - <dir>/runs           (cwd is already demo/)
//   - <dir>/demo/runs      (cwd is dns-cli/ or workspace)
//
// A candidate is accepted when it contains states/ or at least one TLD state.json.
// Explicit empty start uses os.Getwd().
func FindRunsRoot(start string) (string, error) {
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
			filepath.Join(dir, "runs"),
			filepath.Join(dir, "demo", "runs"),
		} {
			if looksLikeRunsRoot(cand) {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find demo runs/ (searched upward from %s); pass --runs-root or run from dns-cli/demo", abs)
}

func looksLikeRunsRoot(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "states")); err == nil {
		return true
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if !ent.IsDir() || isSkippedRunsChild(ent.Name()) {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, ent.Name(), "state.json")); err == nil {
			return true
		}
	}
	// Empty but intentional demo runs tree (only .gitkeep / shared).
	if _, err := os.Stat(filepath.Join(path, ".gitkeep")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "shared")); err == nil {
		return true
	}
	return false
}

// ResolveRunsRoot returns explicit if set, otherwise FindRunsRoot("").
func ResolveRunsRoot(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return FindRunsRoot("")
}
