package demo

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func latestSLDRunID(sldRoot string) (string, error) {
	entries, err := os.ReadDir(sldRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names[0], nil
}

func sldRunComplete(runDir string) bool {
	st, err := loadSLDState(filepath.Join(runDir, "state.json"))
	if err != nil {
		return false
	}
	return st.complete()
}

// runsDefaults finds newest TLD state and a recent SLD name for interactive defaults.
func runsDefaults(runsRoot string) (tld, sld string) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return "", ""
	}
	type cand struct {
		name    string
		modTime time.Time
	}
	var tlds []cand
	for _, e := range entries {
		if !e.IsDir() || isSkippedRunsChild(e.Name()) {
			continue
		}
		stPath := filepath.Join(runsRoot, e.Name(), "state.json")
		info, err := os.Stat(stPath)
		if err != nil {
			continue
		}
		tlds = append(tlds, cand{name: e.Name(), modTime: info.ModTime()})
	}
	if len(tlds) == 0 {
		return "", ""
	}
	sort.Slice(tlds, func(i, j int) bool { return tlds[i].modTime.After(tlds[j].modTime) })
	tld = tlds[0].name
	tldDir := filepath.Join(runsRoot, tld)
	children, err := os.ReadDir(tldDir)
	if err != nil {
		return tld, ""
	}
	var sldCand []cand
	for _, e := range children {
		if !e.IsDir() || isSkippedTLDChild(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sldCand = append(sldCand, cand{name: e.Name(), modTime: info.ModTime()})
	}
	if len(sldCand) == 0 {
		return tld, ""
	}
	sort.Slice(sldCand, func(i, j int) bool { return sldCand[i].modTime.After(sldCand[j].modTime) })
	return tld, sldCand[0].name
}

func (r *Runner) initLayout() error {
	r.paths.applyTLD(r.tld, r.provider)
	for _, d := range []string{
		r.paths.SharedDir,
		r.paths.WalletsDir,
		filepath.Join(r.paths.SharedDir, "tools"),
		filepath.Join(r.paths.TldDir, "config"),
		filepath.Join(r.paths.TldDir, "proofs"),
		filepath.Join(r.paths.TldDir, "contracts"),
		r.paths.TldArtifacts,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(r.paths.TldStateFile); err == nil {
		st, err := loadTLDState(r.paths.TldStateFile)
		if err != nil {
			return err
		}
		r.tldState = st
	} else {
		r.tldState = newTLDState(r.tld, r.mode, r.provider)
	}
	r.tldState.Mode = r.mode
	r.tldState.Provider = r.provider
	r.tldState.TLD = r.tld
	r.tldState.Network = NetworkName
	if err := writeJSONAtomic(r.paths.TldStateFile, r.tldState); err != nil {
		return err
	}

	sldRoot := filepath.Join(r.paths.TldDir, r.sld)
	latest, err := latestSLDRunID(sldRoot)
	if err != nil {
		return err
	}
	if latest != "" && !sldRunComplete(filepath.Join(sldRoot, latest)) {
		r.runID = latest
		slog.Info("Resuming incomplete SLD run", "sld", r.sld, "runId", r.runID)
	} else {
		r.runID = newRunID()
		if latest != "" {
			slog.Debug("latest SLD run complete; starting new run", "previous", latest, "runId", r.runID)
		}
	}
	r.paths.applySLDRun(r.sld, r.runID)
	if err := os.MkdirAll(r.paths.SldArtifacts, 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(r.paths.SldStateFile); err == nil {
		st, err := loadSLDState(r.paths.SldStateFile)
		if err != nil {
			return err
		}
		r.sldState = st
	} else {
		r.sldState = newSLDState(r.tld, r.sld, r.runID, r.mode, r.provider)
	}
	r.sldState.Mode = r.mode
	r.sldState.Provider = r.provider
	r.sldState.TLD = r.tld
	r.sldState.SLD = r.sld
	r.sldState.RunID = r.runID
	r.sldState.Network = NetworkName
	if err := writeJSONAtomic(r.paths.SldStateFile, r.sldState); err != nil {
		return err
	}

	if _, err := os.Stat(r.paths.RecordsFile); os.IsNotExist(err) {
		if raw, err := os.ReadFile(r.paths.RecordsTemplate); err == nil {
			if err := os.WriteFile(r.paths.RecordsFile, raw, 0o644); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	meta := runMeta{
		SchemaVersion: SchemaVersionRun,
		Mode:          r.mode,
		Network:       NetworkName,
		Provider:      r.provider,
		TLD:           r.tld,
		SLD:           r.sld,
		RunID:         r.runID,
		CreatedAt:     time.Now().Format(time.RFC3339Nano),
		SharedDir:     r.paths.SharedDir,
		TldDir:        r.paths.TldDir,
		SldRunDir:     r.paths.SldRunDir,
	}
	if err := writeJSONAtomic(r.paths.RunJSONFile, meta); err != nil {
		return err
	}
	slog.Info("Demo layout ready", "tldState", r.paths.TldStateFile, "sldRun", r.paths.SldRunDir)
	return nil
}

func proofTLDMatches(path, want string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc struct {
		TLD string `json:"tld"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	return doc.TLD == want
}

func readPaymentAddr(walletDir string) (string, error) {
	path := filepath.Join(walletDir, "payment.addr")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("missing payment.addr: %s", path)
	}
	addr := strings.TrimSpace(string(raw))
	if addr == "" {
		return "", fmt.Errorf("empty payment.addr in %s", walletDir)
	}
	return addr, nil
}

func walletReady(walletsDir, name string) bool {
	dir := filepath.Join(walletsDir, name)
	_, errA := os.Stat(filepath.Join(dir, "payment.addr"))
	_, errS := os.Stat(filepath.Join(dir, "payment.skey"))
	return errA == nil && errS == nil
}
