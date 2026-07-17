package cli

import (
	"errors"
	"strings"
)

// Exit codes used by dns-cli. Scripts may rely on these values.
const (
	ExitOK         = 0
	ExitUsage      = 2
	ExitConfig     = 3
	ExitValidation = 4
	ExitProvider   = 5
	ExitWallet     = 6
	ExitBuild      = 7
	ExitSign       = 8
	ExitSubmit     = 9
	ExitTimeout    = 10
	ExitInternal   = 1
)

// ExitCoder is an error that maps to a process exit code.
type ExitCoder interface {
	error
	ExitCode() int
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return "error"
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }
func (e *exitError) ExitCode() int { return e.code }

// WrapExit associates an exit code with an error.
func WrapExit(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

// ExitCode returns the exit code for an error.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ec ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	msg := err.Error()
	if strings.Contains(msg, "required flag") ||
		strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "invalid argument") {
		return ExitUsage
	}
	return ExitInternal
}
