// Command dns-cli is the Go-native CLI for Cardano-based Handshake DNS flows.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/blinklabs-io/dns-cli/internal/cli"
)

func main() {
	code := run()
	os.Exit(code)
}

func run() int {
	root := cli.NewRoot()
	if err := root.ExecuteContext(context.Background()); err != nil {
		slog.Error("command failed", "error", err)
		fmt.Fprintln(os.Stderr, err.Error())
		return cli.ExitCode(err)
	}
	return 0
}
