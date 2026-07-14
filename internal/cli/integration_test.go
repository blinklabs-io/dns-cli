//go:build integration

package cli_test

import (
	"os"
	"testing"
)

// Live preview/preprod tests require DNS_CLI_RUN_LIVE=1 and a configured dns-cli.json.
func TestLiveRegisterTLDSkippedWithoutEnv(t *testing.T) {
	if os.Getenv("DNS_CLI_RUN_LIVE") != "1" {
		t.Skip("set DNS_CLI_RUN_LIVE=1 to run live integration tests")
	}
}
