package demo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func (r *Runner) runExisting() error {
	catalog, err := ReadResumeCatalog(r.paths.RunsRoot)
	if err != nil {
		return err
	}
	fmt.Fprint(r.stdout, FormatResumeCatalog(catalog, !r.opts.NoColor))
	if len(catalog) == 0 {
		return fmt.Errorf("no local TLD/SLD demo runs found under %s (run a fresh demo first)", r.paths.RunsRoot)
	}

	entry, err := SelectResumeEntry(r.stdin, r.stdout, catalog, !r.opts.NoColor)
	if err != nil {
		if errors.Is(err, ErrResumeCancelled) {
			r.guide().Note("Resume cancelled; no changes made.")
			return nil
		}
		return err
	}

	r.mode = "existing"
	r.provider = entry.Provider
	r.tld = entry.TLD
	r.sld = entry.SLD
	r.runID = entry.RunID
	slog.Info("Resuming selected demo run",
		"tld", r.tld, "sld", r.sld, "runId", r.runID, "provider", r.provider, "stage", entry.Stage)

	if err := r.loadSelectedLayout(entry); err != nil {
		return err
	}

	name := r.sld + "." + r.tld
	fmt.Fprint(r.stdout, r.theme().Splash(
		"Handshake DNS on Cardano · Preprod demo",
		"existing",
		r.provider,
		name,
		string(entry.Stage),
	))
	fmt.Fprintln(r.stdout, "")

	g := r.guide()
	g.Step("Prerequisites",
		"Check demo layout, dns-contracts fixtures, and Aiken ≥ 1.1.19.")
	if err := r.ensureFreshPrereqs(); err != nil {
		return err
	}
	g.Step("Provider credentials",
		"Confirm endpoint + API credentials (defaults and existing values are shown; blank keeps them).")
	if err := r.ensureCredentials(); err != nil {
		return err
	}

	if entry.Stage == StageFund || entry.Stage == StageDeploy {
		g.Step("Layout & wallets",
			"Reuse runs/shared wallets and ensure bootstrap.json exists.")
		if err := r.ensureWallets(); err != nil {
			return err
		}
		if _, err := os.Stat(r.paths.BootstrapConfig); err != nil {
			if err := r.writeBootstrapConfig(); err != nil {
				return err
			}
		}
	} else {
		// Later stages still need wallets for signing; create only if missing.
		if err := r.ensureWallets(); err != nil {
			return err
		}
		if _, err := os.Stat(r.paths.BootstrapConfig); err != nil {
			if err := r.writeBootstrapConfig(); err != nil {
				return err
			}
		}
	}

	if entry.Stage == StageFund {
		g.Step("Faucet funding",
			"Bootstrap needs ≥ 150 ADA on Preprod before fund/deploy.")
		if err := r.waitForFunds(); err != nil {
			return err
		}
	} else {
		g.Note("Skip faucet wait — fund already confirmed in TLD state.")
	}

	g.Step("Proof",
		"Ensure the owner's Handshake proof keys exist.")
	if err := r.ensureProof(); err != nil {
		return err
	}

	if err := r.ensureReadinessConfig(entry.Stage); err != nil {
		return err
	}
	cfgPath := r.readinessConfigPath(entry.Stage)
	if err := r.checkProviderReadiness(cfgPath); err != nil {
		return err
	}

	r.showPreflight()
	if !r.prompt.ConfirmProceed("Proceed with Preprod submissions?") {
		slog.Info("Aborted before submissions")
		g.Note("Aborted before on-chain submissions (no changes since preflight).")
		return nil
	}
	if err := r.freshSubmissions(); err != nil {
		return err
	}
	r.showSuccess()
	return nil
}

func (r *Runner) loadSelectedLayout(entry ResumeEntry) error {
	r.paths.applyTLD(entry.TLD, entry.Provider)
	r.paths.applySLDRun(entry.SLD, entry.RunID)

	tldState, err := loadTLDState(r.paths.TldStateFile)
	if err != nil {
		return err
	}
	sldState, err := loadSLDState(r.paths.SldStateFile)
	if err != nil {
		return err
	}
	if err := verifyResumeIdentity(entry, tldState, sldState); err != nil {
		return err
	}
	r.tldState = tldState
	r.sldState = sldState
	r.provider = entry.Provider
	r.tld = entry.TLD
	r.sld = entry.SLD
	r.runID = entry.RunID
	// Do not rewrite state.json or run.json — resume is read-only until a new step confirms.
	return nil
}

func verifyResumeIdentity(entry ResumeEntry, tldState *TLDState, sldState *SLDState) error {
	if strings.TrimSpace(tldState.TLD) != "" && tldState.TLD != entry.TLD {
		return fmt.Errorf("tld state tld mismatch (state=%q selected=%q)", tldState.TLD, entry.TLD)
	}
	if strings.TrimSpace(sldState.TLD) != "" && sldState.TLD != entry.TLD {
		return fmt.Errorf("sld state tld mismatch (state=%q selected=%q)", sldState.TLD, entry.TLD)
	}
	if strings.TrimSpace(sldState.SLD) != "" && sldState.SLD != entry.SLD {
		return fmt.Errorf("sld state sld mismatch (state=%q selected=%q)", sldState.SLD, entry.SLD)
	}
	if strings.TrimSpace(sldState.RunID) != "" && sldState.RunID != entry.RunID {
		return fmt.Errorf("sld state runId mismatch (state=%q selected=%q)", sldState.RunID, entry.RunID)
	}
	if p := strings.TrimSpace(sldState.Provider); p != "" && !strings.EqualFold(p, entry.Provider) {
		return fmt.Errorf("sld state provider mismatch (state=%q selected=%q)", p, entry.Provider)
	}
	if p := strings.TrimSpace(tldState.Provider); p != "" && !strings.EqualFold(p, entry.Provider) {
		return fmt.Errorf("tld state provider mismatch (state=%q selected=%q)", p, entry.Provider)
	}
	return nil
}
