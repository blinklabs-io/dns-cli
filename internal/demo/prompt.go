package demo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
}

// NewPrompter creates a stdin/stdout prompter.
func NewPrompter(in io.Reader, out io.Writer, assumeYes bool) Prompter {
	return &stdPrompter{
		in:        bufio.NewReader(mustReader(in)),
		out:       mustWriter(out),
		assumeYes: assumeYes,
	}
}

func (p *stdPrompter) ConfirmYes(prompt string) bool {
	if p.assumeYes {
		fmt.Fprintf(p.out, "%s [y/N]: y (assume-yes)\n", prompt)
		return true
	}
	fmt.Fprintf(p.out, "%s [y/N]: ", prompt)
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) ConfirmDefault(prompt string) bool {
	if p.assumeYes {
		fmt.Fprintf(p.out, "%s [Y/n]: Y (assume-yes)\n", prompt)
		return true
	}
	fmt.Fprintf(p.out, "%s [Y/n]: ", prompt)
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) ConfirmProceed(prompt string) bool {
	fmt.Fprintf(p.out, "%s [y/N]: ", prompt)
	line, _ := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

func (p *stdPrompter) AskString(prompt, def string) string {
	if p.assumeYes {
		fmt.Fprintf(p.out, "%s [%s]: %s (assume-yes)\n", prompt, def, def)
		return def
	}
	fmt.Fprintf(p.out, "%s [%s]: ", prompt, def)
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
		fmt.Fprintf(p.out, "%s [%s]: %s (assume-yes)\n", prompt, pick, pick)
		return pick
	}

	defaultIndex := choiceDefaultIndex(def, allowed)
	fmt.Fprintf(p.out, "%s\n", prompt)
	for i, opt := range allowed {
		mark := ""
		if i+1 == defaultIndex {
			mark = " (default)"
		}
		fmt.Fprintf(p.out, "  %d) %s%s\n", i+1, opt, mark)
	}
	fmt.Fprintf(p.out, "Enter number [%d]: ", defaultIndex)
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
	fmt.Fprintf(p.out, "Invalid choice %q; keeping default %q.\n", line, allowed[defaultIndex-1])
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

// ReadSecret reads a line without echo when possible; falls back to plain ReadString.
func ReadSecret(in io.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprintf(mustWriter(out), "%s: ", prompt)
	// Plain read — demo credentials are Preprod-only; avoid platform-specific terminal deps.
	line, err := bufio.NewReader(mustReader(in)).ReadString('\n')
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
