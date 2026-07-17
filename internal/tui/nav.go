package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type actionItem struct {
	title, desc, id string
}

func (a actionItem) FilterValue() string { return a.title }
func (a actionItem) Title() string       { return a.title }
func (a actionItem) Description() string { return a.desc }

func allActions() []list.Item {
	items := []actionItem{
		{"Wallet Create", "Generate preprod wallet", "wallet.create"},
		{"Wallet Fund", "Build funding transaction", "wallet.fund"},
		{"Wallet Balance", "Query actor balance", "wallet.balance"},
		{"Proof Generate", "HNS keys + proof bundle", "proof.generate"},
		{"System Prepare", "Parameterize validators", "system.prepare"},
		{"System Init", "Publish reference scripts", "system.init"},
		{"System Bind", "Bind deployment to config", "system.bind"},
		{"Register TLD", "Mint registration NFT", "registrar.register"},
		{"Activate TLD", "Activate TLD token pair", "owner.activate"},
		{"Mint SLD", "Mint second-level domain", "owner.mint"},
		{"Update SLD", "Replace DNS records", "owner.update"},
		{"Tx Inspect", "Inspect envelope", "tx.inspect"},
		{"Tx Sign", "Sign with actor key", "tx.sign"},
		{"Tx Submit", "Submit witnessed tx", "tx.submit"},
		{"Tx Status", "Poll confirmation", "tx.status"},
		{"Exit", "Quit dns-cli dashboard", "app.exit"},
	}
	out := make([]list.Item, len(items))
	for i := range items {
		out[i] = items[i]
	}
	return out
}

func newActionList(width, height int) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	l := list.New(allActions(), delegate, width, height)
	l.Title = "Actions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	return l
}

func selectedActionID(l list.Model) string {
	item, ok := l.SelectedItem().(actionItem)
	if !ok {
		return ""
	}
	return item.id
}

func updateNav(l list.Model, msg tea.Msg) (list.Model, tea.Cmd) {
	var cmd tea.Cmd
	l, cmd = l.Update(msg)
	return l, cmd
}
