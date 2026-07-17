package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
)

var (
	mnemonicKeyRe = regexp.MustCompile(`(?i)(mnemonic|seed|seedphrase|private[_ ]?key)\s*[:=]\s*\S+`)
	hexKeyishRe   = regexp.MustCompile(`(?i)\b[0-9a-f]{64,}\b`)
)

// RedactForLog removes secret-looking material from activity log lines.
func RedactForLog(s string) string {
	s = mnemonicKeyRe.ReplaceAllString(s, "$1=[REDACTED]")
	// Avoid redacting short tx ids (64 hex is common for tx hashes — keep those).
	// Only redact longer blobs that look like raw keys.
	s = regexp.MustCompile(`(?i)\b[0-9a-f]{96,}\b`).ReplaceAllString(s, "[REDACTED_HEX]")
	_ = hexKeyishRe
	return s
}

type activityLog struct {
	lines []string
	vp    viewport.Model
}

func newActivityLog(width, height int) activityLog {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	return activityLog{vp: vp}
}

func (l *activityLog) Append(format string, args ...any) {
	msg := RedactForLog(fmt.Sprintf(format, args...))
	line := fmt.Sprintf("%s  %s", time.Now().Format("15:04:05"), msg)
	l.lines = append(l.lines, line)
	if len(l.lines) > 500 {
		l.lines = l.lines[len(l.lines)-500:]
	}
	l.vp.SetContent(strings.Join(l.lines, "\n"))
	l.vp.GotoBottom()
}

func (l *activityLog) SetSize(width, height int) {
	l.vp.SetWidth(width)
	l.vp.SetHeight(height)
	l.vp.SetContent(strings.Join(l.lines, "\n"))
}

func (l activityLog) View() string {
	return l.vp.View()
}
