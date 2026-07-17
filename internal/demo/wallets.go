package demo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/report"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
)

func (r *Runner) ensureWallets() error {
	readyCount := 0
	for _, name := range walletActors {
		if walletReady(r.paths.WalletsDir, name) {
			readyCount++
		}
	}
	allReady := readyCount == len(walletActors)
	if !allReady {
		if readyCount > 0 {
			slog.Info("Partial wallet set; creating missing actors", "ready", readyCount, "total", len(walletActors))
		}
		return r.createMissingWallets()
	}

	slog.Info("Found complete existing wallet set", "dir", r.paths.WalletsDir)
	if err := r.writeBootstrapConfig(); err != nil {
		return err
	}
	lovelace, err := r.bootstrapLovelace()
	if err != nil {
		return err
	}
	r.showWalletSummary(lovelace)

	var reuse bool
	if lovelace >= MinBootstrapLovelace {
		reuse = r.prompt.ConfirmDefault("Reuse these existing wallets?")
	} else {
		reuse = r.prompt.ConfirmYes("Bootstrap is underfunded. Still reuse these wallets (fund this address next)?")
	}
	if reuse {
		slog.Info("Reusing existing wallets")
		return nil
	}

	confirmed := r.confirmedStepKeys()
	if len(confirmed) > 0 {
		th := r.theme()
		fmt.Fprintln(r.stdout, "")
		fmt.Fprint(r.stdout, th.Warn(fmt.Sprintf("state.json already has confirmed steps (%v).", confirmed)))
		fmt.Fprintln(r.stdout, th.Dim("New wallets will not own those prior on-chain outputs."))
		if !r.prompt.ConfirmYes("Archive wallets and generate an entirely new set anyway?") {
			slog.Info("Keeping existing wallets after warning")
			return nil
		}
	}

	stamp := time.Now().Format("20060102150405")
	archive := filepath.Join(r.paths.SharedDir, "wallets-archive-"+stamp)
	if err := os.Rename(r.paths.WalletsDir, archive); err != nil {
		return err
	}
	if err := os.MkdirAll(r.paths.WalletsDir, 0o755); err != nil {
		return err
	}
	slog.Info("Archived previous wallets", "archive", archive)
	return r.createMissingWallets()
}

func (r *Runner) createMissingWallets() error {
	for _, name := range walletActors {
		if walletReady(r.paths.WalletsDir, name) {
			slog.Info("Wallet exists", "name", name)
			continue
		}
		outDir := filepath.Join(r.paths.WalletsDir, name)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		slog.Info("Creating wallet", "name", name)
		_, err := r.ops.WalletCreate(wallet.GenerateOptions{
			Name:    name,
			Network: NetworkName,
			Format:  wallet.FormatBoth,
			OutDir:  outDir,
		})
		if err != nil {
			return fmt.Errorf("wallet create %s: %w", name, err)
		}
	}
	return nil
}

func (r *Runner) bootstrapLovelace() (int64, error) {
	eff, err := r.loadConfig(r.paths.BootstrapConfig)
	if err != nil {
		return 0, err
	}
	total, _, err := r.ops.WalletBalance(r.ctx, eff, "bootstrap")
	if err != nil {
		// Match script soft-fail for unused address: treat as 0 when explained as empty.
		slog.Warn("Bootstrap balance query failed; treating as 0", "error", err)
		return 0, nil
	}
	return total, nil
}

func (r *Runner) showWalletSummary(lovelace int64) {
	th := r.theme()
	rows := make([]report.KV, 0, len(walletActors)+1)
	for _, name := range walletActors {
		addr, err := readPaymentAddr(filepath.Join(r.paths.WalletsDir, name))
		if err != nil {
			rows = append(rows, report.KV{Key: name, Value: "(missing)"})
			continue
		}
		rows = append(rows, report.KV{Key: name, Value: addr})
	}
	ada := float64(lovelace) / 1_000_000
	rows = append(rows, report.KV{Key: "balance", Value: fmt.Sprintf("%.6f ADA (%d lovelace)", ada, lovelace)})
	fmt.Fprintln(r.stdout, "")
	fmt.Fprint(r.stdout, th.Panel("Shared wallets", rows))
}

func (r *Runner) confirmedStepKeys() []string {
	var keys []string
	for _, k := range []string{"fund", "deploy", "register", "activate"} {
		if r.tldState != nil && r.tldState.stepTxID(k) != "" {
			keys = append(keys, k)
		}
	}
	for _, k := range []string{"mintSld", "updateSld"} {
		if r.sldState != nil && r.sldState.stepTxID(k) != "" {
			keys = append(keys, k)
		}
	}
	return keys
}
