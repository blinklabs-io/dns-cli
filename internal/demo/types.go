package demo

const (
	SchemaVersionState = 2
	ExplorerURLPrefix  = "https://preprod.cexplorer.io/tx/"
)

type StepResult struct {
	TxID     string `json:"txId"`
	Manifest string `json:"manifest"`
}

type TLDState struct {
	SchemaVersion int    `json:"schemaVersion"`
	Mode          string `json:"mode"`
	Network       string `json:"network"`
	Provider      string `json:"provider"`
	TLD           string `json:"tld"`
	Confirmed     struct {
		MintRegistrarToken StepResult `json:"mintRegistrarToken"`
		Fund               StepResult `json:"fund"`
		Deploy             StepResult `json:"deploy"`
		Register           StepResult `json:"register"`
		Activate           StepResult `json:"activate"`
	} `json:"confirmed"`
}

type SLDState struct {
	SchemaVersion int    `json:"schemaVersion"`
	Mode          string `json:"mode"`
	Network       string `json:"network"`
	Provider      string `json:"provider"`
	TLD           string `json:"tld"`
	SLD           string `json:"sld"`
	RunID         string `json:"runId"`
	Confirmed     struct {
		MintSld   StepResult `json:"mintSld"`
		UpdateSld StepResult `json:"updateSld"`
	} `json:"confirmed"`
}

type History struct {
	TLDs []HistoryTLD `json:"tlds"`
}

type HistoryTLD struct {
	TLD       string               `json:"tld"`
	Mode      string               `json:"mode"`
	Network   string               `json:"network"`
	Provider  string               `json:"provider"`
	Confirmed map[string]HistoryTx `json:"confirmed"`
	Runs      []HistoryRun         `json:"runs"`
}

type HistoryRun struct {
	TLD       string               `json:"tld"`
	SLD       string               `json:"sld"`
	RunID     string               `json:"runId"`
	Mode      string               `json:"mode"`
	Network   string               `json:"network"`
	Provider  string               `json:"provider"`
	Status    string               `json:"status"`
	Confirmed map[string]HistoryTx `json:"confirmed"`
}

type HistoryTx struct {
	TxID        string `json:"txId"`
	Manifest    string `json:"manifest,omitempty"`
	ExplorerURL string `json:"explorerUrl,omitempty"`
}
