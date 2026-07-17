package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInitialViewContainsBrand(t *testing.T) {
	m := initialModel(DashboardOpts{
		Version: VersionInfo{Version: "1.2.3", GitCommit: "abcdef012345", BuildDate: "today"},
	})
	m.width = 120
	m.height = 40
	m.ready = true
	m.layoutSizes()
	s := m.View().Content
	if !strings.Contains(s, "dns-cli") {
		t.Fatalf("missing dns-cli brand: %q", s)
	}
	if !strings.Contains(s, "1.2.3") {
		t.Fatalf("missing version: %q", s)
	}
}

func TestConfigGateBlocksWithoutPath(t *testing.T) {
	m := initialModel(DashboardOpts{Version: VersionInfo{Version: "dev"}})
	if m.configLoaded() {
		t.Fatal("expected no config")
	}
	work := m.renderWorkbench()
	if !strings.Contains(work, "Config path") {
		t.Fatalf("expected config gate copy, got %q", work)
	}
}

func TestConfigPathInHeader(t *testing.T) {
	m := initialModel(DashboardOpts{
		ConfigPath: `E:\cfg\dns-cli.json`,
		Version:    VersionInfo{Version: "dev", GitCommit: "abc"},
	})
	m.width = 120
	m.height = 40
	m.ready = true
	m.layoutSizes()
	s := m.View().Content
	if !strings.Contains(s, `E:\cfg\dns-cli.json`) && !strings.Contains(s, "dns-cli.json") {
		t.Fatalf("header missing config path: %q", s)
	}
}

func TestWindowSizeNoPanic(t *testing.T) {
	m := initialModel(DashboardOpts{Version: VersionInfo{Version: "dev"}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mm := next.(model)
	_ = mm.View()
}

func TestNavSelectionChangesID(t *testing.T) {
	m := initialModel(DashboardOpts{ConfigPath: "x.json", Version: VersionInfo{Version: "dev"}})
	id1 := selectedActionID(m.nav)
	m.nav.CursorDown()
	id2 := selectedActionID(m.nav)
	if id1 == "" || id2 == "" {
		t.Fatalf("empty ids %q %q", id1, id2)
	}
	if id1 == id2 {
		t.Fatal("expected cursor down to change selection")
	}
}

func TestRedactForLog(t *testing.T) {
	in := "wallet create mnemonic=alpha beta gamma delta path=/tmp/w"
	out := RedactForLog(in)
	if strings.Contains(out, "alpha beta") {
		t.Fatalf("mnemonic not redacted: %q", out)
	}
	if !strings.Contains(out, "/tmp/w") && !strings.Contains(out, "path=") {
		t.Fatalf("path should remain: %q", out)
	}
}

func TestIdentityDetail(t *testing.T) {
	s := renderIdentityDetail(VersionInfo{
		Version: "v", GitCommit: "c", GoVersion: "go1", ApolloRevision: "a", ContractRevision: "r",
	})
	for _, want := range []string{"dns-cli", "apollo", "contracts", "go1"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %q", want, s)
		}
	}
}

func TestExitActionQuits(t *testing.T) {
	m := initialModel(DashboardOpts{Version: VersionInfo{Version: "dev"}, Runner: fakeRunner{}})
	cmd := m.openForm("app.exit")
	if cmd == nil {
		t.Fatal("expected Quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestLogHeightLargerShare(t *testing.T) {
	m := initialModel(DashboardOpts{Version: VersionInfo{Version: "dev"}, Runner: fakeRunner{}})
	m.width = 120
	m.height = 40
	m.layoutSizes()
	if m.logH < 16 {
		t.Fatalf("logH too small: %d", m.logH)
	}
	if m.logH <= m.mainH {
		t.Fatalf("expected log taller or equal share than main: log=%d main=%d", m.logH, m.mainH)
	}
}
