package demo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// ErrResumeCancelled is returned when the user quits resume selection.
var ErrResumeCancelled = errors.New("resume selection cancelled")

// SelectResumeEntry prompts for a numbered resume entry.
// Blank/invalid/completed selections re-prompt. "q" or EOF cancels.
// --yes / assume-yes must not auto-select.
func SelectResumeEntry(in io.Reader, out io.Writer, entries []ResumeEntry, color bool) (ResumeEntry, error) {
	if len(entries) == 0 {
		return ResumeEntry{}, fmt.Errorf("no local TLD/SLD demo runs found")
	}
	th := report.New(color)
	reader := bufio.NewReader(mustReader(in))
	w := mustWriter(out)
	for {
		fmt.Fprint(w, th.PromptCursor(fmt.Sprintf("Select run number (1-%d, or q to cancel): ", len(entries))))
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				return ResumeEntry{}, ErrResumeCancelled
			}
			if !errors.Is(err, io.EOF) {
				return ResumeEntry{}, err
			}
		}
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Fprint(w, th.Warn("Enter a number to continue."))
			continue
		}
		if strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
			return ResumeEntry{}, ErrResumeCancelled
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(entries) {
			fmt.Fprint(w, th.Warn(fmt.Sprintf("Invalid choice %q; enter a number between 1 and %d.", line, len(entries))))
			continue
		}
		entry := entries[n-1]
		if !entry.Resumable {
			fmt.Fprint(w, th.Warn("Run is complete and cannot be resumed; choose another."))
			continue
		}
		return entry, nil
	}
}
