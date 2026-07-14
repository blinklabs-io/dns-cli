// Package logging configures structured slog output for dns-cli.
package logging

import (
	"fmt"
	"log/slog"
)

// LevelTrace is below slog.LevelDebug for verbosity level 4.
const LevelTrace = slog.LevelDebug - 4

// LevelFromVerbose maps -v/--verbose (0-4) to a slog minimum level.
func LevelFromVerbose(n int) (slog.Level, error) {
	switch n {
	case 0:
		return slog.LevelError, nil
	case 1:
		return slog.LevelWarn, nil
	case 2:
		return slog.LevelInfo, nil
	case 3:
		return slog.LevelDebug, nil
	case 4:
		return LevelTrace, nil
	default:
		return 0, fmt.Errorf("invalid --verbose %d (want 0-4: error|warn|info|debug|trace)", n)
	}
}
