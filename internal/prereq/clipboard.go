package prereq

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

// CopyToClipboard best-effort copies text. Returns a warning string if it failed.
func CopyToClipboard(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("empty clipboard text")
	}
	if err := clipboard.WriteAll(text); err == nil {
		return nil
	}
	// Fallback platform tools
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "clip")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	default:
		if path, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command(path)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command(path, "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		return fmt.Errorf("no clipboard helper available")
	}
}
