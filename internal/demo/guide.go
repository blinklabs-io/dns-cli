package demo

import (
	"fmt"
	"io"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/report"
)

// guide prints operator-facing demo narration to stdout.
// Deliberately separate from slog (stderr) so explanations stay readable.
type guide struct {
	out io.Writer
	th  *report.Theme
}

func (r *Runner) guide() guide {
	return guide{out: mustWriter(r.stdout), th: r.theme()}
}

func (r *Runner) theme() *report.Theme {
	return report.New(!r.opts.NoColor)
}

// Step announces a major demo phase with a short explanation and optional CLI hints.
func (g guide) Step(title, what string, cliLines ...string) {
	fmt.Fprint(g.out, g.th.Step(title, what, cliLines...))
}

// Note prints a short DEMO note (skip/resume/hint) without a full step banner.
func (g guide) Note(msg string) {
	fmt.Fprint(g.out, g.th.Note(msg))
}

// CompletionReport is the final demo summary with every confirmed tx + explorer link.
type CompletionReport struct {
	TLD         string
	SLD         string
	RunID       string
	Provider    string
	RecordsPath string
	BoundConfig string
	TLDState    string
	SLDRunDir   string
	Explorer    string // URL prefix
	// Ordered steps: label + txId (empty = missing)
	Steps []ReportStep
}

// ReportStep is one line in the completion tx table.
type ReportStep struct {
	Label string
	TxID  string
}

// Done prints a full completion report: all txs with explorer links, paths, and next steps.
func (g guide) Done(rep CompletionReport) {
	prefix := rep.Explorer
	if prefix == "" {
		prefix = ExplorerURLPrefix
	}

	var txBlock strings.Builder
	for _, step := range rep.Steps {
		txBlock.WriteString(g.th.TxLine(step.Label, step.TxID, prefix))
	}

	var paths strings.Builder
	if rep.RecordsPath != "" {
		paths.WriteString(g.th.Kv("    records", rep.RecordsPath))
	}
	if rep.BoundConfig != "" {
		paths.WriteString(g.th.Kv("    bound cfg", rep.BoundConfig))
	}
	if rep.TLDState != "" {
		paths.WriteString(g.th.Kv("    TLD state", rep.TLDState))
	}
	if rep.SLDRunDir != "" {
		paths.WriteString(g.th.Kv("    SLD run", rep.SLDRunDir))
	}

	var next strings.Builder
	next.WriteString(g.th.Dim("  Change DNS later: edit the records file, then:"))
	next.WriteByte('\n')
	next.WriteString("    dns-cli owner update-sld --config <bound.json> \\\n")
	fmt.Fprintf(&next, "      --tld %s --sld %s --records <records.json> --out <prefix>\n", rep.TLD, rep.SLD)
	next.WriteString("    dns-cli tx apply --config <bound.json> --tx <prefix>.unsigned.json \\\n")
	next.WriteString("      --actor sldOwner --signed <prefix>.signed.json --manifest <prefix>.manifest.json\n")

	title := fmt.Sprintf("DEMO COMPLETE · %s.%s", rep.SLD, rep.TLD)
	meta := []report.KV{
		{Key: "Name:", Value: fmt.Sprintf("%s.%s", rep.SLD, rep.TLD)},
		{Key: "Provider:", Value: rep.Provider},
	}
	if rep.RunID != "" {
		meta = append(meta, report.KV{Key: "Run ID:", Value: rep.RunID})
	}

	fmt.Fprint(g.out, g.th.Completion(title, meta, txBlock.String(), paths.String(), next.String()))
}

func quotePath(p string) string {
	if p == "" {
		return `""`
	}
	if strings.ContainsAny(p, " \t\"") {
		return `"` + p + `"`
	}
	return p
}
