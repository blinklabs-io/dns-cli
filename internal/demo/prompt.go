package demo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// Prompter handles interactive confirmations for the demo runner.
type Prompter interface {
	ConfirmYes(prompt string) bool     // default No [y/N]; AssumeYes → true
	ConfirmDefault(prompt string) bool // default Yes [Y/n]; AssumeYes → true
	ConfirmProceed(prompt string) bool // default No; NEVER auto-yes
	AskString(prompt, def string) string
	// AskChoice shows a numbered menu when allowed is non-empty; otherwise AskString.
	AskChoice(prompt, def string, allowed []string) string
}

type stdPrompter struct {
	in        *bufio.Reader
	out       io.Writer
	assumeYes bool
	th        *report.Theme
}

// NewPrompter creates a stdin/stdout prompter. color enables ANSI when the terminal supports it.
func NewPrompter(in io.Reader, out io.Writer, assumeYes bool, color bool) Prompter {
	return &stdPrompter{
		in:        bufio.NewReader(mustReader(in)),
		out:       mustWriter(out),
		assumeYes: assumeYes,
		th:        report.New(color),
	}
}

func (p *stdPrompter) ConfirmYes(prompt string) bool {
	if p.assumeYes {
		fmt.Fprint(p.out, p.th.PromptCursor(prompt+" [y/N]: "))
		fmt.Fprintln(p.out, "y (assume-yes)")
		return true
	}
	fmt.Fprint(p.out, p.th.PromptCursor(prompt+" [y/N]: "))
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) ConfirmDefault(prompt string) bool {
	if p.assumeYes {
		fmt.Fprint(p.out, p.th.PromptCursor(prompt+" [Y/n]: "))
		fmt.Fprintln(p.out, "Y (assume-yes)")
		return true
	}
	fmt.Fprint(p.out, p.th.PromptCursor(prompt+" [Y/n]: "))
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) ConfirmProceed(prompt string) bool {
	fmt.Fprint(p.out, p.th.PromptCursor(prompt+" [y/N]: "))
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) AskString(prompt, def string) string {
	label := fmt.Sprintf("%s [%s]: ", prompt, def)
	if p.assumeYes {
		fmt.Fprint(p.out, p.th.PromptCursor(label))
		fmt.Fprintf(p.out, "%s (assume-yes)\n", def)
		return def
	}
	fmt.Fprint(p.out, p.th.PromptCursor(label))
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func (p *stdPrompter) AskChoice(prompt, def string, allowed []string) string {
	if len(allowed) == 0 {
		return p.AskString(prompt, def)
	}
	if p.assumeYes {
		pick := resolveChoiceDefault(def, allowed)
		fmt.Fprint(p.out, p.th.PromptCursor(fmt.Sprintf("%s [%s]: ", prompt, pick)))
		fmt.Fprintf(p.out, "%s (assume-yes)\n", pick)
		return pick
	}

	defaultIndex := choiceDefaultIndex(def, allowed)
	fmt.Fprint(p.out, p.th.ChoiceMenu(prompt, def, allowed))
	fmt.Fprint(p.out, p.th.PromptCursor(fmt.Sprintf("Enter number [%d]: ", defaultIndex)))
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return allowed[defaultIndex-1]
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(allowed) {
		return allowed[n-1]
	}
	for _, opt := range allowed {
		if strings.EqualFold(line, opt) {
			return opt
		}
	}
	fmt.Fprint(p.out, p.th.Warn(fmt.Sprintf("Invalid choice %q; keeping default %q.", line, allowed[defaultIndex-1])))
	return allowed[defaultIndex-1]
}

func choiceDefaultIndex(def string, allowed []string) int {
	for i, opt := range allowed {
		if strings.EqualFold(def, opt) {
			return i + 1
		}
	}
	return 1
}

func resolveChoiceDefault(def string, allowed []string) string {
	return allowed[choiceDefaultIndex(def, allowed)-1]
}

// ReadSecret reads a credential line from in.
// Prefer a *bufio.Reader (or the demo Prompter's reader) so shared stdin is not double-buffered.
func ReadSecret(in io.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(mustWriter(out), report.New(false).PromptCursor(prompt+": "))
	br, ok := in.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(mustReader(in))
	}
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func envTruthy(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
