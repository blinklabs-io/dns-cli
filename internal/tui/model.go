package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/tui/forms"
)

type paneFocus int

const (
	focusNav paneFocus = iota
	focusWorkbench
	focusStatus
	focusLog
)

// DashboardOpts configures the dashboard program.
type DashboardOpts struct {
	ConfigPath       string
	Version          VersionInfo
	Network          string
	Provider         string
	ContractRevision string
	Timeout          time.Duration
	Runner           Runner // optional; defaults to ops-backed runner
}

type model struct {
	opts         DashboardOpts
	runner       Runner
	eff          *config.Effective
	width        int
	height       int
	focus        paneFocus
	nav          list.Model
	configPath   string
	network      string
	provider     string
	health       string
	actor        string
	status       statusState
	log          activityLog
	picker       textinput.Model
	values       forms.ActionValues
	form         *huh.Form
	activeAction string
	confirm      forms.ConfirmState
	showHelp     bool
	showIdent    bool
	ready        bool
	busy         bool
	timeout      time.Duration
	mainH        int
	logH         int
	zones        *zone.Manager
}

func initialModel(opts DashboardOpts) model {
	nav := newActionList(24, 16)
	picker := textinput.New()
	picker.Placeholder = "path/to/dns-cli.json"
	picker.CharLimit = 512
	picker.SetWidth(48)
	if opts.ConfigPath != "" {
		picker.SetValue(opts.ConfigPath)
	}
	runner := opts.Runner
	if runner == nil {
		runner = newOpsRunner(opts.ContractRevision)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	m := model{
		opts:       opts,
		runner:     runner,
		nav:        nav,
		configPath: opts.ConfigPath,
		network:    opts.Network,
		provider:   opts.Provider,
		health:     "idle",
		actor:      "bootstrap",
		status:     statusState{Balances: map[string]string{}},
		log:        newActivityLog(80, 6),
		picker:     picker,
		values:     forms.ActionValues{Network: "preprod", Actor: "bootstrap", FromActor: "bootstrap", SLDOwner: "sldOwner", Provider: "blockfrost", Format: "both", Collateral: "5000000"},
		timeout:    timeout,
		zones:      zone.New(),
	}
	m.log.Append("dashboard started")
	if m.configPath == "" {
		m.log.Append("config required for most actions — enter path or pick an offline action")
		m.focus = focusWorkbench
		m.picker.Focus()
	} else {
		m.log.Append("config path set: %s", m.configPath)
		m.focus = focusNav
		if eff, err := m.runner.Load(m.configPath); err == nil {
			m.eff = eff
			m.network = eff.Profile.Network.Name
			m.provider = eff.Profile.Provider.Type
			m.health = "loaded"
		} else {
			m.log.Append("config load failed: %v", err)
			m.health = "error"
		}
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layoutSizes()
		return m, nil
	case OpResultMsg:
		return m.applyOpResult(msg), nil
	case tea.KeyPressMsg:
		key := msg.String()
		if m.showHelp || m.showIdent {
			if key == "esc" || key == "i" || key == "?" || key == "q" {
				m.showHelp = false
				m.showIdent = false
				if key == "q" {
					return m, tea.Quit
				}
				return m, nil
			}
			return m, nil
		}
		if m.confirm.Active {
			switch key {
			case "y", "enter":
				action := m.confirm.Action
				m.confirm.Clear()
				return m, m.runAction(action)
			case "esc", "n":
				m.log.Append("cancelled %s", m.confirm.Action)
				m.confirm.Clear()
				return m, nil
			}
			return m, nil
		}
		// Global keys when form is not capturing exclusively — still allow quit/help.
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.form == nil {
				return m, tea.Quit
			}
		case "?":
			if m.form == nil {
				m.showHelp = true
				return m, nil
			}
		case "i":
			if m.form == nil {
				m.showIdent = true
				return m, nil
			}
		case "y":
			if m.form == nil {
				target := CopyTarget(m.status.LastTxID, m.status.ExplorerURL, m.status.LastArtifact)
				if target == "" {
					m.log.Append("nothing to copy")
				} else if err := copyToClipboard(target); err != nil {
					m.log.Append("copy failed: %v", err)
				} else {
					m.log.Append("copied %s", target)
				}
				return m, nil
			}
		case "r":
			if m.form == nil {
				if !m.configLoaded() {
					m.log.Append("refresh requires config")
					return m, nil
				}
				m.busy = true
				m.log.Append("refreshing…")
				return m, runRefreshCmd(m.runner, m.configPath)
			}
		case "tab":
			if m.form == nil {
				m.focus = (m.focus + 1) % 4
				m.applyFocus()
				return m, nil
			}
		case "shift+tab":
			if m.form == nil {
				m.focus = (m.focus + 3) % 4
				m.applyFocus()
				return m, nil
			}
		case "esc":
			if m.form != nil {
				m.form = nil
				m.activeAction = ""
				m.focus = focusNav
				m.log.Append("form cancelled")
				return m, nil
			}
		}
		if m.focus == focusWorkbench && !m.configLoaded() && m.form == nil {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			if key == "enter" {
				path := strings.TrimSpace(m.picker.Value())
				if path != "" {
					m.configPath = filepath.Clean(path)
					m.log.Append("config selected: %s", m.configPath)
					if eff, err := m.runner.Load(m.configPath); err != nil {
						m.log.Append("config load failed: %v", err)
						m.health = "error"
					} else {
						m.eff = eff
						m.network = eff.Profile.Network.Name
						m.provider = eff.Profile.Provider.Type
						m.health = "loaded"
						m.focus = focusNav
						m.picker.Blur()
					}
				}
			}
			return m, cmd
		}
		if m.focus == focusWorkbench && m.form != nil {
			formModel, cmd := m.form.Update(msg)
			if f, ok := formModel.(*huh.Form); ok {
				m.form = f
			}
			switch m.form.State {
			case huh.StateCompleted:
				action := m.activeAction
				m.form = nil
				if forms.Mutating(action) {
					m.confirm.Ask("Confirm "+action+"?", m.confirmSummary(action), action)
					return m, nil
				}
				return m, m.runAction(action)
			case huh.StateAborted:
				m.form = nil
				m.activeAction = ""
				m.focus = focusNav
				m.log.Append("form aborted")
				return m, nil
			}
			return m, cmd
		}
		if m.focus == focusNav {
			var cmd tea.Cmd
			m.nav, cmd = updateNav(m.nav, msg)
			if key == "enter" {
				id := selectedActionID(m.nav)
				return m, m.openForm(id)
			}
			return m, cmd
		}
		if m.focus == focusLog {
			var cmd tea.Cmd
			m.log.vp, cmd = m.log.vp.Update(msg)
			return m, cmd
		}
	}
	// Forward other msgs (blink, etc.) into active form.
	if m.form != nil && m.focus == focusWorkbench {
		formModel, cmd := m.form.Update(msg)
		if f, ok := formModel.(*huh.Form); ok {
			m.form = f
		}
		return m, cmd
	}
	return m, nil
}

func (m model) applyOpResult(msg OpResultMsg) model {
	m.busy = false
	if msg.Err != nil {
		m.status.LastError = msg.Err.Error()
		m.log.Append("%s failed: %v", msg.Action, msg.Err)
		m.health = "error"
		return m
	}
	m.status.LastError = ""
	if msg.Message != "" {
		m.log.Append("%s", msg.Message)
	}
	if msg.Artifact != "" {
		m.status.LastArtifact = msg.Artifact
	}
	if msg.TxID != "" {
		m.status.LastTxID = msg.TxID
	}
	if msg.BodyHash != "" {
		m.status.BodyHash = msg.BodyHash
	}
	if msg.Explorer != "" {
		m.status.ExplorerURL = msg.Explorer
	}
	if msg.TLD != "" {
		m.status.TLD = msg.TLD
	}
	if msg.SLD != "" {
		m.status.SLD = msg.SLD
	} else if msg.Action == "registrar.register" || msg.Action == "owner.activate" {
		m.status.SLD = ""
	}
	for k, v := range msg.Balances {
		m.status.Balances[k] = v
	}
	if msg.Checklist != nil {
		if msg.Checklist.Prepare != nil {
			m.status.Checklist.Prepare = *msg.Checklist.Prepare
		}
		if msg.Checklist.InitBind != nil {
			m.status.Checklist.InitBind = *msg.Checklist.InitBind
		}
		if msg.Checklist.Register != nil {
			m.status.Checklist.Register = *msg.Checklist.Register
		}
		if msg.Checklist.Activate != nil {
			m.status.Checklist.Activate = *msg.Checklist.Activate
		}
		if msg.Checklist.Mint != nil {
			m.status.Checklist.Mint = *msg.Checklist.Mint
		}
		if msg.Checklist.Update != nil {
			m.status.Checklist.Update = *msg.Checklist.Update
		}
	}
	if msg.Action == "refresh" {
		m.health = "ok"
	}
	return m
}

func (m *model) openForm(action string) tea.Cmd {
	if action == "app.exit" {
		m.log.Append("exiting dashboard")
		return tea.Quit
	}
	if forms.NeedsConfig(action) && (m.eff == nil || !m.configLoaded()) {
		m.log.Append("%s requires a loaded config — set path first", action)
		m.focus = focusWorkbench
		m.picker.Focus()
		return nil
	}
	m.activeAction = action
	m.values.Actor = m.actor
	if m.network != "" {
		m.values.Network = m.network
	}
	if m.provider != "" {
		m.values.Provider = m.provider
	}
	m.form = forms.NewActionForm(action, &m.values)
	m.focus = focusWorkbench
	m.log.Append("form: %s — complete fields, ctrl+s or last enter to submit · esc cancel", action)
	return m.form.Init()
}

func (m *model) runAction(action string) tea.Cmd {
	if m.busy {
		m.log.Append("busy — wait for current op")
		return nil
	}
	if forms.NeedsConfig(action) && m.eff == nil {
		m.log.Append("load config first")
		return nil
	}
	m.busy = true
	m.actor = m.values.Actor
	m.log.Append("running %s…", action)
	return runActionCmd(m.runner, m.eff, action, m.values, m.timeout)
}

func (m model) confirmSummary(action string) string {
	v := m.values
	switch action {
	case "wallet.create":
		return fmt.Sprintf("name=%s out=%s", v.Name, v.OutDir)
	case "wallet.fund":
		return fmt.Sprintf("from=%s out=%s", v.FromActor, v.Out)
	case "registrar.register", "owner.activate":
		return fmt.Sprintf("tld=%s out=%s", v.TLD, v.Out)
	case "owner.mint", "owner.update":
		return fmt.Sprintf("tld=%s sld=%s out=%s", v.TLD, v.SLD, v.Out)
	case "tx.sign":
		return fmt.Sprintf("tx=%s actor=%s out=%s", v.TxPath, v.Actor, v.Out)
	case "tx.submit":
		return fmt.Sprintf("tx=%s", v.TxPath)
	default:
		return "This may write artifacts or touch the chain. Continue?"
	}
}

func (m *model) configLoaded() bool {
	return strings.TrimSpace(m.configPath) != ""
}

func (m *model) applyFocus() {
	m.picker.Blur()
	if m.focus == focusWorkbench && !m.configLoaded() && m.form == nil {
		m.picker.Focus()
	}
}

func (m *model) layoutSizes() {
	if m.width < 40 || m.height < 12 {
		return
	}
	navW := 28
	statusW := 34
	midW := m.width - navW - statusW - 4
	if midW < 20 {
		midW = 20
	}
	// header(~1) + help(~1) + borders/padding(~4)
	bodyH := m.height - 6
	if bodyH < 10 {
		bodyH = 10
	}
	// Activity log: majority of the body (~55%, min 16 lines).
	logH := (bodyH * 11) / 20
	if logH < 16 {
		logH = 16
	}
	if logH > bodyH-6 {
		logH = bodyH - 6
	}
	mainH := bodyH - logH
	if mainH < 6 {
		mainH = 6
	}
	m.mainH = mainH
	m.logH = logH
	m.nav.SetSize(navW-2, mainH-2)
	m.log.SetSize(m.width-4, logH-2)
	m.picker.SetWidth(midW - 6)
	if m.form != nil {
		m.form.WithWidth(midW - 4)
	}
}

func (m model) View() tea.View {
	if !m.ready {
		return tea.NewView("initializing dns-cli dashboard…")
	}
	if m.showIdent {
		v := tea.NewView(renderIdentityDetail(m.opts.Version))
		v.AltScreen = true
		return v
	}
	if m.showHelp {
		v := tea.NewView(renderHelp())
		v.AltScreen = true
		return v
	}
	waitBadge := "idle"
	if m.status.Waiting {
		waitBadge = fmt.Sprintf("poll #%d", m.status.Wait.Poll)
	}
	header := renderIdentity(m.opts.Version, m.configPath, m.network, m.provider, m.health, m.actor, waitBadge)

	mainH := m.mainH
	if mainH < 6 {
		mainH = max(6, m.height/2)
	}
	logH := m.logH
	if logH < 16 {
		logH = 16
	}
	navView := stylePane.Width(28).Height(mainH).Render(m.nav.View())
	work := m.renderWorkbench()
	status := stylePane.Width(34).Height(mainH).Render(renderStatusPane(m.status))
	mid := stylePane.Width(max(20, m.width-28-34-6)).Height(mainH).Render(work)
	row := lipgloss.JoinHorizontal(lipgloss.Top, navView, mid, status)
	logPane := stylePane.Width(m.width - 2).Height(logH).Render(styleTitle.Render("Activity") + "\n" + m.log.View())
	help := styleHelp.Render("tab focus · enter open form · Exit to quit · y copy · r refresh · i identity · ? help · q quit")
	content := lipgloss.JoinVertical(lipgloss.Left, header, row, logPane, help)
	v := tea.NewView(m.zones.Scan(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) renderWorkbench() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Workbench"))
	b.WriteString("\n\n")
	if m.confirm.Active {
		b.WriteString(m.confirm.View())
		return b.String()
	}
	if m.form != nil {
		b.WriteString(fmt.Sprintf("Action: %s\n\n", m.activeAction))
		if m.busy {
			b.WriteString(styleDim.Render("running…") + "\n\n")
		}
		b.WriteString(m.form.View())
		b.WriteString("\n")
		b.WriteString(styleDim.Render("esc cancel form"))
		return b.String()
	}
	if !m.configLoaded() {
		b.WriteString("Config path (required for most actions):\n\n")
		b.WriteString(m.picker.View())
		b.WriteString("\n\n")
		b.WriteString(styleDim.Render("enter to load · or open wallet.create / proof.generate / system.prepare|bind without config"))
		return b.String()
	}
	b.WriteString(fmt.Sprintf("Config: %s\n", m.configPath))
	b.WriteString(fmt.Sprintf("Selected: %s\n\n", selectedActionID(m.nav)))
	if m.busy {
		b.WriteString(styleDim.Render("running…") + "\n\n")
	}
	b.WriteString(styleDim.Render("press enter on an Action to open its form"))
	return b.String()
}

func renderHelp() string {
	return strings.Join([]string{
		styleTitle.Render("dns-cli dashboard help"),
		"",
		"enter on Action   open Huh form (Exit quits)",
		"form complete     run (confirm if mutating)",
		"y                 copy txId/explorer/artifact",
		"r                 refresh config/health",
		"esc               cancel form / confirm",
		"i / ?             identity / help",
		"q / ctrl+c / Exit quit",
		"",
		"Forms: wallet, proof, system, registrar, owner, tx",
		"",
		styleDim.Render("press ? or esc to close"),
	}, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
