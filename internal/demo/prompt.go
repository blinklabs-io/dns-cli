package demo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Prompter handles interactive confirmations for the demo runner.
type Prompter interface {
	ConfirmYes(prompt string) bool     // default No [y/N]; AssumeYes → true
	ConfirmDefault(prompt string) bool // default Yes [Y/n]; AssumeYes → true
	ConfirmProceed(prompt string) bool // default No; NEVER auto-yes
	AskString(prompt, def string) string
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
