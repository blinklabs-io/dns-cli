package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			return err2
		}
	}
	return nil
}

func loadTLDState(path string) (*TLDState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st TLDState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if st.SchemaVersion != SchemaVersionState {
		return nil, fmt.Errorf("%s: unsupported TLD state.schemaVersion (want %d)", path, SchemaVersionState)
	}
	return &st, nil
}

func loadSLDState(path string) (*SLDState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st SLDState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if st.SchemaVersion != SchemaVersionState {
		return nil, fmt.Errorf("%s: unsupported SLD state.schemaVersion (want %d)", path, SchemaVersionState)
	}
	return &st, nil
}

func emptyStep() StepResult { return StepResult{TxID: "", Manifest: ""} }

func newTLDState(tld, mode, provider string) *TLDState {
	st := &TLDState{
		SchemaVersion: SchemaVersionState,
		Mode:          mode,
		Network:       NetworkName,
		Provider:      provider,
		TLD:           tld,
	}
	st.Confirmed.Fund = emptyStep()
	st.Confirmed.Deploy = emptyStep()
	st.Confirmed.Register = emptyStep()
	st.Confirmed.Activate = emptyStep()
	return st
}

func newSLDState(tld, sld, runID, mode, provider string) *SLDState {
	st := &SLDState{
		SchemaVersion: SchemaVersionState,
		Mode:          mode,
		Network:       NetworkName,
		Provider:      provider,
		TLD:           tld,
		SLD:           sld,
		RunID:         runID,
	}
	st.Confirmed.MintSld = emptyStep()
	st.Confirmed.UpdateSld = emptyStep()
	return st
}

func (st *TLDState) stepTxID(key string) string {
	switch key {
	case "fund":
		return strings.TrimSpace(st.Confirmed.Fund.TxID)
	case "deploy":
		return strings.TrimSpace(st.Confirmed.Deploy.TxID)
	case "register":
		return strings.TrimSpace(st.Confirmed.Register.TxID)
	case "activate":
		return strings.TrimSpace(st.Confirmed.Activate.TxID)
	default:
		return ""
	}
}

func (st *TLDState) setStep(key string, step StepResult) {
	switch key {
	case "fund":
		st.Confirmed.Fund = step
	case "deploy":
		st.Confirmed.Deploy = step
	case "register":
		st.Confirmed.Register = step
	case "activate":
		st.Confirmed.Activate = step
	}
}

func (st *SLDState) stepTxID(key string) string {
	switch key {
	case "mintSld":
		return strings.TrimSpace(st.Confirmed.MintSld.TxID)
	case "updateSld":
		return strings.TrimSpace(st.Confirmed.UpdateSld.TxID)
	default:
		return ""
	}
}

func (st *SLDState) setStep(key string, step StepResult) {
	switch key {
	case "mintSld":
		st.Confirmed.MintSld = step
	case "updateSld":
		st.Confirmed.UpdateSld = step
	}
}

func (st *SLDState) complete() bool {
	return st.stepTxID("mintSld") != "" && st.stepTxID("updateSld") != ""
}

type runMeta struct {
	SchemaVersion int    `json:"schemaVersion"`
	Mode          string `json:"mode"`
	Network       string `json:"network"`
	Provider      string `json:"provider"`
	TLD           string `json:"tld"`
	SLD           string `json:"sld"`
	RunID         string `json:"runId"`
	CreatedAt     string `json:"createdAt"`
	SharedDir     string `json:"sharedDir"`
	TldDir        string `json:"tldDir"`
	SldRunDir     string `json:"sldRunDir"`
}
