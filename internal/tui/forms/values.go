package forms

// ActionValues holds workbench field values bound by Huh forms.
type ActionValues struct {
	// Shared / wallet
	Name       string
	Network    string
	Format     string
	OutDir     string
	Force      bool
	FromActor  string
	Allocation string // repeatable-style: "a=1,b=2" or newline-separated actor=lovelace
	Collateral string
	Out        string
	Actor      string

	// Proof / protocol
	TLD      string
	SLD      string
	Proof    string
	OwnerKey string
	Records  string
	SLDOwner string

	// System
	Blueprint            string
	RegistrarTokenPolicy string
	StakeKey             string
	Aiken                string
	Deployment           string
	BaseConfig           string
	TxID                 string
	ActorDir             string
	Provider             string

	// Tx
	TxPath     string
	Manifest   string
	AllowExtra bool
	Wait       bool
}

// Mutating reports whether the action should show a confirm dialog.
func Mutating(action string) bool {
	switch action {
	case "wallet.create", "wallet.fund", "system.prepare", "system.init", "system.bind",
		"registrar.register", "owner.activate", "owner.mint", "owner.update",
		"tx.sign", "tx.submit":
		return true
	default:
		return false
	}
}

// NeedsConfig reports whether the action requires a loaded dns-cli config.
func NeedsConfig(action string) bool {
	switch action {
	case "wallet.create", "proof.generate", "system.prepare", "system.bind", "app.exit":
		return false
	default:
		return true
	}
}
