package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelp(t *testing.T) {
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("dns-cli")) {
		t.Fatal("expected help to mention dns-cli")
	}
}

func TestUnknownCommandExitCode(t *testing.T) {
	root := NewRoot()
	root.SetArgs([]string{"not-a-command"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitUsage, err)
	}
}

func TestVersionJSON(t *testing.T) {
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"version", "--output", "json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"ok": true`)) {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func TestVersionVerboseDebugLog(t *testing.T) {
	root := NewRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"version", "-v", "3", "--no-color"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("version")) {
		t.Fatalf("expected debug log on stderr, got: %q", stderr.String())
	}
}

func TestInvalidVerboseExitUsage(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version", "-v", "9"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d", ExitCode(err), ExitUsage)
	}
}

func TestConfigInitDry(t *testing.T) {
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init", "--network", "preview", "--provider", "utxorpc", "--config", t.TempDir() + "/dns-cli.json", "--force"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestWalletCreateJSON(t *testing.T) {
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	outDir := t.TempDir()
	root.SetArgs([]string{
		"wallet", "create",
		"--name", "bootstrap",
		"--network", "preprod",
		"--format", "key-envelope",
		"--out-dir", outDir,
		"--output", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"ok": true`)) {
		t.Fatalf("unexpected output: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"paymentKeyHash"`)) {
		t.Fatalf("expected paymentKeyHash in output: %s", buf.String())
	}
}

func TestWalletFundHelp(t *testing.T) {
	out, err := captureExecute(t, "wallet", "fund", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--from-actor", "--allocation", "--collateral", "--out"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("expected %s in help: %s", want, out)
		}
	}
}

func TestWalletFundRequiresAllocation(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"wallet", "fund", "--out", "artifacts/fund"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitUsage, err)
	}
}

func TestWalletFundInvalidAllocation(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"wallet", "fund",
		"--allocation", "not-valid",
		"--out", "artifacts/fund",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitValidation {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitValidation, err)
	}
}

func TestInvalidOutputMode(t *testing.T) {
	root := NewRoot()
	root.SetArgs([]string{"version", "--output", "xml"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d", ExitCode(err), ExitUsage)
	}
}

func captureExecute(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

func TestCobraSilence(t *testing.T) {
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return WrapExit(ExitUsage, errMarker) }}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestProofGenerateHelp(t *testing.T) {
	out, err := captureExecute(t, "proof", "generate", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("--tld")) {
		t.Fatalf("expected --tld in help: %s", out)
	}
}

func TestProofGenerateRequiresFlags(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"proof", "generate"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d", ExitCode(err), ExitUsage)
	}
}

func TestSystemPrepareHelp(t *testing.T) {
	out, err := captureExecute(t, "system", "prepare", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--blueprint", "--registrar-hns-key", "--stake-key", "--out-dir"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("expected %s in help: %s", want, out)
		}
	}
}

func TestSystemInitBindHelp(t *testing.T) {
	out, err := captureExecute(t, "system", "init", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("--deployment")) {
		t.Fatalf("expected --deployment: %s", out)
	}
	out, err = captureExecute(t, "system", "bind", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("--tx-id")) {
		t.Fatalf("expected --tx-id: %s", out)
	}
}

func TestWalletWaitFundsHelp(t *testing.T) {
	out, err := captureExecute(t, "wallet", "wait-funds", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--actor", "--min-lovelace", "--poll"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("expected %s in help: %s", want, out)
		}
	}
}

func TestWalletWaitFundsRequiresActor(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"wallet", "wait-funds"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitUsage, err)
	}
}

func TestTxApplyHelp(t *testing.T) {
	out, err := captureExecute(t, "tx", "apply", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--tx", "--actor", "--signed", "--manifest"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("expected %s in help: %s", want, out)
		}
	}
}

func TestTxApplyRequiresFlags(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"tx", "apply"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitUsage, err)
	}
}

func TestDemoHistoryHelp(t *testing.T) {
	out, err := captureExecute(t, "demo", "history", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("auto-detect")) && !bytes.Contains([]byte(out), []byte("--runs-root")) {
		t.Fatalf("expected auto-detect / --runs-root in help: %s", out)
	}
}

func TestDemoRunHelp(t *testing.T) {
	out, err := captureExecute(t, "demo", "run", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--demo-root", "--mode", "--provider", "--yes", "--skip-install"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("expected %s in help: %s", want, out)
		}
	}
}

func TestDemoRunRequiresDemoRoot(t *testing.T) {
	root := NewRoot()
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"demo", "run"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("got exit %d want %d (%v)", ExitCode(err), ExitUsage, err)
	}
}

func TestDemoHistoryEmptyJSON(t *testing.T) {
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"demo", "history", "--runs-root", t.TempDir(), "--output", "json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"ok": true`)) {
		t.Fatalf("unexpected output: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("no demo history yet")) {
		t.Fatalf("expected empty history message: %s", buf.String())
	}
}

var errMarker = bytes.ErrTooLarge
