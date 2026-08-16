package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/ops"
	"github.com/blinklabs-io/dns-cli/internal/tui"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
)

func opsClient() *ops.Client {
	return ops.New(ContractRevision)
}

func runWalletFund(ctx context.Context, eff *config.Effective, fromActor string, allocations []txbuilder.FundAllocation, collateralLovelace int64, out string) (string, error) {
	path, err := opsClient().WalletFund(ctx, eff, fromActor, allocations, collateralLovelace, out)
	if err != nil {
		if ops.IsWalletFundValidation(err) {
			return "", WrapExit(ExitValidation, err)
		}
		return "", mapContextErr(err)
	}
	return path, nil
}

func runWalletBalance(ctx context.Context, eff *config.Effective, actor string) (int64, int, error) {
	total, count, err := opsClient().WalletBalance(ctx, eff, actor)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unknown actor") || strings.Contains(msg, "invalid address") {
			return 0, 0, WrapExit(ExitValidation, err)
		}
		return 0, 0, WrapExit(ExitProvider, err)
	}
	return total, count, nil
}

func runWalletWaitFunds(ctx context.Context, eff *config.Effective, actor string, minLovelace int64, poll, timeout time.Duration) (ops.WaitFundsResult, error) {
	result, err := opsClient().WalletWaitFunds(ctx, eff, actor, minLovelace, poll, timeout)
	if err != nil {
		msg := err.Error()
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			return ops.WaitFundsResult{}, WrapExit(ExitTimeout, err)
		case strings.Contains(msg, "unknown actor") || strings.Contains(msg, "invalid address") || strings.Contains(msg, "min lovelace") || strings.Contains(msg, "poll interval"):
			return ops.WaitFundsResult{}, WrapExit(ExitValidation, err)
		case strings.Contains(msg, "environment variable"):
			return ops.WaitFundsResult{}, WrapExit(ExitConfig, err)
		default:
			return ops.WaitFundsResult{}, WrapExit(ExitProvider, err)
		}
	}
	return result, nil
}

func runRegisterTLD(ctx context.Context, eff *config.Effective, tldRaw, proofPath, out string) (string, error) {
	path, err := opsClient().RegisterTLD(ctx, eff, tldRaw, proofPath, out)
	if err != nil {
		return "", mapProtocolBuildErr(err)
	}
	return path, nil
}

func runActivateTLD(ctx context.Context, eff *config.Effective, tldRaw, ownerKeyPath, out string) (string, error) {
	path, err := opsClient().ActivateTLD(ctx, eff, tldRaw, ownerKeyPath, out)
	if err != nil {
		return "", mapProtocolBuildErr(err)
	}
	return path, nil
}

func runMintSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, sldOwner, out string) (string, error) {
	path, err := opsClient().MintSLD(ctx, eff, tldRaw, sldRaw, sldOwner, out)
	if err != nil {
		return "", mapProtocolBuildErr(err)
	}
	return path, nil
}

func runUpdateSLD(ctx context.Context, eff *config.Effective, tldRaw, sldRaw, recordsPath, out string) (string, error) {
	path, err := opsClient().UpdateSLD(ctx, eff, tldRaw, sldRaw, recordsPath, out)
	if err != nil {
		return "", mapProtocolBuildErr(err)
	}
	return path, nil
}

func mapProtocolBuildErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "REPLACE_ME"),
		strings.Contains(msg, "GENERATED_BY"),
		strings.Contains(msg, "missing reference"):
		return WrapExit(ExitConfig, fmt.Errorf("%w\nhint: run system bind and config validate after deploy confirms", err))
	case strings.Contains(msg, "already registered"),
		strings.Contains(msg, "already activated"),
		strings.Contains(msg, "already exists"),
		strings.Contains(msg, "registration not found"),
		strings.Contains(msg, "no suitable collateral"),
		strings.Contains(msg, "insufficient"):
		return WrapExit(ExitValidation, fmt.Errorf("%w\nhint: check Preprod faucet funding, collateral (ADA-only), and prior register/activate steps", err))
	case strings.Contains(msg, "unknown actor"),
		strings.Contains(msg, "invalid"),
		strings.Contains(msg, "empty"),
		strings.Contains(msg, "too long"),
		strings.Contains(msg, "proof"),
		strings.Contains(msg, "records"),
		strings.Contains(msg, "label"):
		return WrapExit(ExitValidation, err)
	case strings.Contains(msg, "provider"),
		strings.Contains(msg, "contracts"):
		return mapContextErr(err)
	default:
		return WrapExit(ExitBuild, err)
	}
}

func runTxInspect(txPath string) (map[string]any, error) {
	data, err := opsClient().TxInspect(txPath)
	if err != nil {
		return nil, WrapExit(ExitValidation, err)
	}
	return data, nil
}

func runTxSign(eff *config.Effective, txPath, actor, out string, allowExtra bool) error {
	err := opsClient().TxSign(eff, txPath, actor, out, allowExtra)
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unknown actor"):
		return WrapExit(ExitValidation, err)
	case strings.Contains(msg, "wallet"), strings.Contains(msg, "mnemonic"), strings.Contains(msg, "signing key"), strings.Contains(msg, "LoadWallet"):
		return WrapExit(ExitWallet, err)
	default:
		// FromActor failures are wallet; SignWithWallet are sign
		if strings.Contains(msg, "actor") && strings.Contains(msg, "signing") {
			return WrapExit(ExitWallet, err)
		}
		return WrapExit(ExitSign, err)
	}
}

func runTxSubmit(ctx context.Context, eff *config.Effective, txPath string) (string, string, error) {
	txHex, explorer, err := opsClient().TxSubmit(ctx, eff, txPath)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not fully signed"),
			strings.Contains(msg, "no vkey"),
			strings.Contains(msg, "manifest network"),
			strings.Contains(msg, "manifest provider"):
			return "", "", WrapExit(ExitSubmit, err)
		case strings.Contains(msg, "provider"):
			return "", "", WrapExit(ExitProvider, err)
		case strings.Contains(msg, "ReadEnvelope"), strings.Contains(msg, "decode"), strings.Contains(msg, "Decode"):
			return "", "", WrapExit(ExitValidation, err)
		default:
			// envelope read errors often say "open" / "read"
			if strings.Contains(msg, "open ") || strings.Contains(msg, "no such file") {
				return "", "", WrapExit(ExitValidation, err)
			}
			return "", "", WrapExit(ExitSubmit, err)
		}
	}
	return txHex, explorer, nil
}

func runTxStatus(ctx context.Context, eff *config.Effective, txID, manifestPath string, wait bool, timeout time.Duration, outputMode string, color bool) (string, error) {
	forcePlain := strings.EqualFold(strings.TrimSpace(outputMode), "json")
	reporter := tui.NewWaitReporter(tui.WaitOptions{ForcePlain: forcePlain, Color: color && !forcePlain})
	status, err := opsClient().TxStatus(ctx, eff, txID, manifestPath, wait, timeout, reporter)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "provider"):
			return "", WrapExit(ExitProvider, err)
		case strings.Contains(msg, "--wait"), strings.Contains(msg, "manifest"), strings.Contains(msg, "invalid hex"), strings.Contains(msg, "32-byte"):
			return "", WrapExit(ExitValidation, err)
		default:
			if wait {
				return "", WrapExit(ExitTimeout, err)
			}
			return "", WrapExit(ExitProvider, err)
		}
	}
	return status, nil
}

func runTxApply(ctx context.Context, eff *config.Effective, txPath, actor, signedPath, manifestPath string, allowExtra bool, timeout time.Duration, outputMode string, color bool) (ops.TxApplyResult, error) {
	forcePlain := strings.EqualFold(strings.TrimSpace(outputMode), "json")
	reporter := tui.NewWaitReporter(tui.WaitOptions{ForcePlain: forcePlain, Color: color && !forcePlain})
	result, err := opsClient().TxApply(ctx, eff, txPath, actor, signedPath, manifestPath, allowExtra, timeout, reporter)
	if err != nil {
		msg := err.Error()
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			return ops.TxApplyResult{}, WrapExit(ExitTimeout, err)
		case strings.Contains(msg, "unknown actor"),
			strings.Contains(msg, "--tx"),
			strings.Contains(msg, "--actor"),
			strings.Contains(msg, "--signed"),
			strings.Contains(msg, "--manifest"),
			strings.Contains(msg, "open "),
			strings.Contains(msg, "no such file"),
			strings.Contains(msg, "manifest"),
			strings.Contains(msg, "invalid hex"),
			strings.Contains(msg, "32-byte"):
			return ops.TxApplyResult{}, WrapExit(ExitValidation, err)
		case strings.Contains(msg, "wallet"), strings.Contains(msg, "mnemonic"), strings.Contains(msg, "signing key"), strings.Contains(msg, "LoadWallet"):
			return ops.TxApplyResult{}, WrapExit(ExitWallet, err)
		case strings.Contains(msg, "not fully signed"), strings.Contains(msg, "no vkey"):
			return ops.TxApplyResult{}, WrapExit(ExitSubmit, err)
		case strings.Contains(msg, "provider"):
			return ops.TxApplyResult{}, WrapExit(ExitProvider, err)
		default:
			if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
				return ops.TxApplyResult{}, WrapExit(ExitTimeout, err)
			}
			return ops.TxApplyResult{}, WrapExit(ExitSubmit, err)
		}
	}
	return result, nil
}

// runConfigValidateOnline checks placeholders, provider health, and reference UTxOs.
func runConfigValidateOnline(ctx context.Context, eff *config.Effective) error {
	return opsClient().ValidateConfig(ctx, eff, true)
}

func mapContextErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "provider"):
		return WrapExit(ExitProvider, err)
	case strings.Contains(msg, "contracts"):
		return WrapExit(ExitConfig, err)
	default:
		return WrapExit(ExitBuild, err)
	}
}
