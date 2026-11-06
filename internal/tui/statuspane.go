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
	fmt.Fprintf(&b, "TLD: %s\n", s.tld())
	fmt.Fprintf(&b, "SLD: %s\n", s.sld())
	b.WriteString("\nBalances:\n")
	if len(s.Balances) == 0 {
		b.WriteString(styleDim.Render("  (none yet)") + "\n")
	} else {
		for actor, bal := range s.Balances {
			fmt.Fprintf(&b, "  %s: %s\n", actor, bal)
		}
	}
	b.WriteString("\nChecklist:\n")
	fmt.Fprintf(&b, "  [%s] prepare\n", mark(s.Checklist.Prepare))
	fmt.Fprintf(&b, "  [%s] init/bind\n", mark(s.Checklist.InitBind))
	fmt.Fprintf(&b, "  [%s] register\n", mark(s.Checklist.Register))
	fmt.Fprintf(&b, "  [%s] activate\n", mark(s.Checklist.Activate))
	fmt.Fprintf(&b, "  [%s] mint\n", mark(s.Checklist.Mint))
	fmt.Fprintf(&b, "  [%s] update\n", mark(s.Checklist.Update))
	b.WriteString("\nWait:\n")
	if s.Waiting {
		fmt.Fprintf(&b, "  stage %s poll #%d\n", s.Wait.Stage, s.Wait.Poll)
		fmt.Fprintf(&b, "  txId %s\n", s.Wait.TxID)
	} else {
		b.WriteString(styleDim.Render("  idle") + "\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "artifact: %s\n", dash(s.LastArtifact))
	fmt.Fprintf(&b, "txId:     %s\n", dash(s.LastTxID))
	fmt.Fprintf(&b, "bodyHash: %s\n", dash(s.BodyHash))
	fmt.Fprintf(&b, "explorer: %s\n", dash(s.ExplorerURL))
	if s.LastError != "" {
		fmt.Fprintf(&b, "error:    %s\n", s.LastError)
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
