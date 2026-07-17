package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// ReadHistory scans runsRoot for TLD/SLD state files and returns a read-only history.
func ReadHistory(runsRoot string) (History, error) {
	out := History{TLDs: []HistoryTLD{}}
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return History{}, err
	}
	for _, ent := range entries {
		if !ent.IsDir() || isSkippedRunsChild(ent.Name()) {
			continue
		}
		tldDir := filepath.Join(runsRoot, ent.Name())
		statePath := filepath.Join(tldDir, "state.json")
		raw, err := os.ReadFile(statePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return History{}, err
		}
		var tldState TLDState
		if err := json.Unmarshal(raw, &tldState); err != nil {
			return History{}, fmt.Errorf("%s: %w", statePath, err)
		}
		if tldState.SchemaVersion != SchemaVersionState {
			return History{}, fmt.Errorf("%s: unsupported TLD state.schemaVersion (want %d)", statePath, SchemaVersionState)
		}
		tldName := strings.TrimSpace(tldState.TLD)
		if tldName == "" {
			tldName = ent.Name()
		}
		item := HistoryTLD{
			TLD:      tldName,
			Mode:     tldState.Mode,
			Network:  tldState.Network,
			Provider: tldState.Provider,
			Confirmed: map[string]HistoryTx{
				"fund":     historyTx(tldState.Confirmed.Fund),
				"deploy":   historyTx(tldState.Confirmed.Deploy),
				"register": historyTx(tldState.Confirmed.Register),
				"activate": historyTx(tldState.Confirmed.Activate),
			},
			Runs: []HistoryRun{},
		}
		runs, err := readSLDRuns(tldDir, tldName)
		if err != nil {
			return History{}, err
		}
		item.Runs = runs
		out.TLDs = append(out.TLDs, item)
	}
	return out, nil
}

func readSLDRuns(tldDir, tldName string) ([]HistoryRun, error) {
	entries, err := os.ReadDir(tldDir)
	if err != nil {
		return nil, err
	}
	var runs []HistoryRun
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
			sldName := strings.TrimSpace(sldState.SLD)
			if sldName == "" {
				sldName = ent.Name()
			}
			runID := strings.TrimSpace(sldState.RunID)
			if runID == "" {
				runID = runEnt.Name()
			}
			status := "incomplete"
			if strings.TrimSpace(sldState.Confirmed.MintSld.TxID) != "" && strings.TrimSpace(sldState.Confirmed.UpdateSld.TxID) != "" {
				status = "complete"
			}
			runs = append(runs, HistoryRun{
				TLD:      firstNonEmpty(sldState.TLD, tldName),
				SLD:      sldName,
				RunID:    runID,
				Mode:     sldState.Mode,
				Network:  sldState.Network,
				Provider: sldState.Provider,
				Status:   status,
				Confirmed: map[string]HistoryTx{
					"mintSld":   historyTx(sldState.Confirmed.MintSld),
					"updateSld": historyTx(sldState.Confirmed.UpdateSld),
				},
			})
		}
	}
	return runs, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// FormatHistoryHuman renders a clear aligned history report.
// When color is true, ANSI styling is applied (honor --no-color / NO_COLOR at call sites).
func FormatHistoryHuman(h History, color bool) string {
	return FormatHistoryHumanAt(h, "", color)
}

// FormatHistoryHumanAt is like FormatHistoryHuman but includes the runs root path in the header.
func FormatHistoryHumanAt(h History, runsRoot string, color bool) string {
	th := report.New(color)
	if len(h.TLDs) == 0 {
		msg := "no demo history yet (run a fresh demo first)"
		if runsRoot != "" {
			msg = fmt.Sprintf("no demo history yet under %s (run a fresh demo first)", runsRoot)
		}
		return th.Dim(msg) + "\n"
	}
	var b strings.Builder
	title := "Demo history"
	if runsRoot != "" {
		title = "Demo history · " + runsRoot
	}
	b.WriteString(th.Title(title))
	b.WriteByte('\n')
	for i, tld := range h.TLDs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(th.Title(fmt.Sprintf("TLD %s", tld.TLD)))
		b.WriteByte('\n')
		b.WriteString(historyKV(th, "provider", tld.Provider))
		b.WriteString(historyKV(th, "network", tld.Network))
		b.WriteString(historyKV(th, "mode", tld.Mode))
		for _, key := range []string{"fund", "deploy", "register", "activate"} {
			b.WriteString(historyStepLines(th, key, tld.Confirmed[key], "  "))
		}
		for _, run := range tld.Runs {
			b.WriteByte('\n')
			b.WriteString(th.Title(fmt.Sprintf("SLD %s.%s", run.SLD, run.TLD)))
			b.WriteByte('\n')
			b.WriteString(historyKV(th, "runId", run.RunID))
			b.WriteString(historyKV(th, "status", run.Status))
			for _, key := range []string{"mintSld", "updateSld"} {
				label := key
				if key == "mintSld" {
					label = "mint-sld"
				}
				if key == "updateSld" {
					label = "update-sld"
				}
				b.WriteString(historyStepLines(th, label, run.Confirmed[key], "  "))
			}
		}
	}
	return b.String()
}

func historyKV(th *report.Theme, key, value string) string {
	if strings.TrimSpace(value) == "" {
		value = "-"
	}
	return th.Dim(fmt.Sprintf("  %-10s", key)) + "  " + value + "\n"
}

func historyStepLines(th *report.Theme, label string, tx HistoryTx, indent string) string {
	txID := strings.TrimSpace(tx.TxID)
	if txID == "" {
		return th.Dim(fmt.Sprintf("%s%-10s", indent, label)) + "  " + th.Dim("(empty)") + "\n"
	}
	var b strings.Builder
	b.WriteString(th.Dim(fmt.Sprintf("%s%-10s", indent, label)))
	b.WriteString("  ")
	b.WriteString(txID)
	b.WriteByte('\n')
	explorer := strings.TrimSpace(tx.ExplorerURL)
	if explorer == "" {
		explorer = explorerURL(txID)
	}
	if explorer != "" {
		b.WriteString(th.Dim(fmt.Sprintf("%s%-10s", indent, "explorer")))
		b.WriteString("  ")
		b.WriteString(explorer)
		b.WriteByte('\n')
	}
	return b.String()
}
