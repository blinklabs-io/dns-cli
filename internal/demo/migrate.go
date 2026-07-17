package demo

import (
	"log/slog"
	"os"
	"path/filepath"
)

// migrateLegacyRuntime is a no-op placeholder for the one-time demo/runtime → runs/
// migration. The PowerShell runner performed this; new Go runs start from runs/.
// If a legacy runtime tree exists without a migrated marker, we simply log a hint.
func migrateLegacyRuntime(paths Paths) error {
	if _, err := os.Stat(paths.LegacyRuntime); err != nil {
		return nil
	}
	// If runs/ already has any TLD state, assume migration already happened.
	entries, err := os.ReadDir(paths.RunsRoot)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !isSkippedRunsChild(e.Name()) {
				if _, err := os.Stat(filepath.Join(paths.RunsRoot, e.Name(), "state.json")); err == nil {
					return nil
				}
			}
		}
	}
	slog.Warn("legacy demo/runtime detected; automatic migration is not performed by the Go runner",
		"runtime", paths.LegacyRuntime,
		"hint", "use a previous run-demo script version to migrate, or start fresh under runs/")
	return nil
}
