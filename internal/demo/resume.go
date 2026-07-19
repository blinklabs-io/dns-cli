package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// ResumeStage is the first incomplete demo lifecycle stage for a run.
type ResumeStage string

const (
	StageFund      ResumeStage = "fund"
	StageDeploy    ResumeStage = "deploy"
	StageBind      ResumeStage = "bind"
	StageRegister  ResumeStage = "register"
	StageActivate  ResumeStage = "activate"
	StageMintSLD   ResumeStage = "mint-sld"
	StageUpdateSLD ResumeStage = "update-sld"
	StageComplete  ResumeStage = "complete"
)

// ResumeEntry is one selectable (or completed) local TLD/SLD run.
type ResumeEntry struct {
	TLD       string
	SLD       string
	RunID     string
	Network   string
	Provider  string
	Stage     ResumeStage
	Resumable bool
}

// ReadResumeCatalog scans runsRoot for SLD run states and derives resume stages.
func ReadResumeCatalog(runsRoot string) ([]ResumeEntry, error) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ResumeEntry
	for _, ent := range entries {
		if !ent.IsDir() || isSkippedRunsChild(ent.Name()) {
			continue
		}
		tldDir := filepath.Join(runsRoot, ent.Name())
		tldStatePath := filepath.Join(tldDir, "state.json")
		raw, err := os.ReadFile(tldStatePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var tldState TLDState
		if err := json.Unmarshal(raw, &tldState); err != nil {
			return nil, fmt.Errorf("%s: %w", tldStatePath, err)
		}
		if tldState.SchemaVersion != SchemaVersionState {
			return nil, fmt.Errorf("%s: unsupported TLD state.schemaVersion (want %d)", tldStatePath, SchemaVersionState)
		}
		tldName := strings.TrimSpace(tldState.TLD)
		if tldName == "" {
			tldName = ent.Name()
		} else if tldName != ent.Name() {
			return nil, fmt.Errorf("%s: tld identity conflict (state=%q dir=%q)", tldStatePath, tldName, ent.Name())
		}
		runs, err := readResumeRuns(tldDir, tldName, &tldState)
		if err != nil {
			return nil, err
		}
		out = append(out, runs...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return resumeLess(out[i], out[j])
	})
	return out, nil
}

func readResumeRuns(tldDir, tldName string, tldState *TLDState) ([]ResumeEntry, error) {
	entries, err := os.ReadDir(tldDir)
	if err != nil {
		return nil, err
	}
	var out []ResumeEntry
	for _, ent := range entries {
		if !ent.IsDir() || isSkippedTLDChild(ent.Name()) {
			continue
		}
		sldDir := filepath.Join(tldDir, ent.Name())
		runEntries, err := os.ReadDir(sldDir)
		if err != nil {
			return nil, err
		}
		for _, runEnt := range runEntries {
			if !runEnt.IsDir() {
				continue
			}
			statePath := filepath.Join(sldDir, runEnt.Name(), "state.json")
			raw, err := os.ReadFile(statePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			var sldState SLDState
			if err := json.Unmarshal(raw, &sldState); err != nil {
				return nil, fmt.Errorf("%s: %w", statePath, err)
			}
			if sldState.SchemaVersion != SchemaVersionState {
				return nil, fmt.Errorf("%s: unsupported SLD state.schemaVersion (want %d)", statePath, SchemaVersionState)
			}
			entry, err := buildResumeEntry(tldDir, tldName, ent.Name(), runEnt.Name(), tldState, &sldState, statePath)
			if err != nil {
				return nil, err
			}
			out = append(out, entry)
		}
	}
	return out, nil
}

func buildResumeEntry(tldDir, tldName, sldDirName, runDirName string, tldState *TLDState, sldState *SLDState, statePath string) (ResumeEntry, error) {
	sldName := strings.TrimSpace(sldState.SLD)
	if sldName == "" {
		sldName = sldDirName
	}
	runID := strings.TrimSpace(sldState.RunID)
	if runID == "" {
		runID = runDirName
	}
	if strings.TrimSpace(sldState.TLD) != "" && sldState.TLD != tldName {
		return ResumeEntry{}, fmt.Errorf("%s: tld identity conflict (sld state=%q tld state=%q)", statePath, sldState.TLD, tldName)
	}
	if strings.TrimSpace(sldState.SLD) != "" && sldState.SLD != sldDirName {
		return ResumeEntry{}, fmt.Errorf("%s: sld identity conflict (state=%q dir=%q)", statePath, sldState.SLD, sldDirName)
	}
	if strings.TrimSpace(sldState.RunID) != "" && sldState.RunID != runDirName {
		return ResumeEntry{}, fmt.Errorf("%s: runId identity conflict (state=%q dir=%q)", statePath, sldState.RunID, runDirName)
	}

	provider := strings.TrimSpace(sldState.Provider)
	if provider == "" {
		provider = strings.TrimSpace(tldState.Provider)
	}
	if pSLD, pTLD := strings.TrimSpace(sldState.Provider), strings.TrimSpace(tldState.Provider); pSLD != "" && pTLD != "" && !strings.EqualFold(pSLD, pTLD) {
		return ResumeEntry{}, fmt.Errorf("%s: provider conflict (sld=%q tld=%q)", statePath, pSLD, pTLD)
	}
	network := strings.TrimSpace(sldState.Network)
	if network == "" {
		network = strings.TrimSpace(tldState.Network)
	}
	if nSLD, nTLD := strings.TrimSpace(sldState.Network), strings.TrimSpace(tldState.Network); nSLD != "" && nTLD != "" && !strings.EqualFold(nSLD, nTLD) {
		return ResumeEntry{}, fmt.Errorf("%s: network conflict (sld=%q tld=%q)", statePath, nSLD, nTLD)
	}
	if provider == "" {
		return ResumeEntry{}, fmt.Errorf("%s: provider is required", statePath)
	}

	stage := deriveResumeStage(tldDir, provider, tldState, sldState)
	return ResumeEntry{
		TLD:       tldName,
		SLD:       sldName,
		RunID:     runID,
		Network:   network,
		Provider:  strings.ToLower(provider),
		Stage:     stage,
		Resumable: stage != StageComplete,
	}, nil
}

func deriveResumeStage(tldDir, provider string, tldState *TLDState, sldState *SLDState) ResumeStage {
	if strings.TrimSpace(tldState.Confirmed.Fund.TxID) == "" {
		return StageFund
	}
	if strings.TrimSpace(tldState.Confirmed.Deploy.TxID) == "" {
		return StageDeploy
	}
	bound := filepath.Join(tldDir, "config", provider+".json")
	if _, err := os.Stat(bound); err != nil {
		return StageBind
	}
	if strings.TrimSpace(tldState.Confirmed.Register.TxID) == "" {
		return StageRegister
	}
	if strings.TrimSpace(tldState.Confirmed.Activate.TxID) == "" {
		return StageActivate
	}
	if strings.TrimSpace(sldState.Confirmed.MintSld.TxID) == "" {
		return StageMintSLD
	}
	if strings.TrimSpace(sldState.Confirmed.UpdateSld.TxID) == "" {
		return StageUpdateSLD
	}
	return StageComplete
}

func resumeLess(a, b ResumeEntry) bool {
	if c := strings.Compare(strings.ToLower(a.TLD), strings.ToLower(b.TLD)); c != 0 {
		return c < 0
	}
	if a.TLD != b.TLD {
		return a.TLD < b.TLD
	}
	if c := strings.Compare(strings.ToLower(a.SLD), strings.ToLower(b.SLD)); c != 0 {
		return c < 0
	}
	if a.SLD != b.SLD {
		return a.SLD < b.SLD
	}
	if c := strings.Compare(strings.ToLower(a.RunID), strings.ToLower(b.RunID)); c != 0 {
		return c < 0
	}
	return a.RunID < b.RunID
}

// FormatResumeCatalog renders a numbered resume list without tx/explorer details.
func FormatResumeCatalog(entries []ResumeEntry, color bool) string {
	th := report.New(color)
	if len(entries) == 0 {
		return th.Dim("no local TLD/SLD demo runs found under demo/runs (run a fresh demo first)") + "\n"
	}
	var b strings.Builder
	b.WriteString(th.Title("Resume demo runs"))
	b.WriteByte('\n')
	b.WriteString(th.Dim("  #  domain              runId              provider     stage"))
	b.WriteByte('\n')
	for i, e := range entries {
		stage := string(e.Stage)
		if !e.Resumable {
			stage = "complete (not resumable)"
		}
		line := fmt.Sprintf("  %-2d %-19s %-18s %-12s %s",
			i+1,
			e.SLD+"."+e.TLD,
			e.RunID,
			e.Provider,
			stage,
		)
		if e.Resumable {
			b.WriteString(line)
		} else {
			b.WriteString(th.Dim(line))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
