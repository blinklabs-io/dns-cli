package tui

import (
	"fmt"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/logging"
)

type checklist struct {
	Prepare  bool
	InitBind bool
	Register bool
	Activate bool
	Mint     bool
	Update   bool
}

type statusState struct {
	TLD          string
	SLD          string
	Balances     map[string]string
	Checklist    checklist
	Wait         logging.WaitProgress
	Waiting      bool
	LastArtifact string
	LastTxID     string
	BodyHash     string
	LastError    string
	ExplorerURL  string
}

func (s statusState) tld() string {
	if strings.TrimSpace(s.TLD) == "" {
		return "—"
	}
	return s.TLD
}

func (s statusState) sld() string {
	if strings.TrimSpace(s.SLD) == "" {
		return "—"
	}
	return s.SLD
}

func renderStatusPane(s statusState) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Status"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("TLD: %s\n", s.tld()))
	b.WriteString(fmt.Sprintf("SLD: %s\n", s.sld()))
	b.WriteString("\nBalances:\n")
	if len(s.Balances) == 0 {
		b.WriteString(styleDim.Render("  (none yet)") + "\n")
	} else {
		for actor, bal := range s.Balances {
			b.WriteString(fmt.Sprintf("  %s: %s\n", actor, bal))
		}
	}
	b.WriteString("\nChecklist:\n")
	b.WriteString(fmt.Sprintf("  [%s] prepare\n", mark(s.Checklist.Prepare)))
	b.WriteString(fmt.Sprintf("  [%s] init/bind\n", mark(s.Checklist.InitBind)))
	b.WriteString(fmt.Sprintf("  [%s] register\n", mark(s.Checklist.Register)))
	b.WriteString(fmt.Sprintf("  [%s] activate\n", mark(s.Checklist.Activate)))
	b.WriteString(fmt.Sprintf("  [%s] mint\n", mark(s.Checklist.Mint)))
	b.WriteString(fmt.Sprintf("  [%s] update\n", mark(s.Checklist.Update)))
	b.WriteString("\nWait:\n")
	if s.Waiting {
		b.WriteString(fmt.Sprintf("  stage %s poll #%d\n", s.Wait.Stage, s.Wait.Poll))
		b.WriteString(fmt.Sprintf("  txId %s\n", s.Wait.TxID))
	} else {
		b.WriteString(styleDim.Render("  idle") + "\n")
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("artifact: %s\n", dash(s.LastArtifact)))
	b.WriteString(fmt.Sprintf("txId:     %s\n", dash(s.LastTxID)))
	b.WriteString(fmt.Sprintf("bodyHash: %s\n", dash(s.BodyHash)))
	b.WriteString(fmt.Sprintf("explorer: %s\n", dash(s.ExplorerURL)))
	if s.LastError != "" {
		b.WriteString(fmt.Sprintf("error:    %s\n", s.LastError))
	}
	return b.String()
}

func mark(ok bool) string {
	if ok {
		return "x"
	}
	return " "
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
