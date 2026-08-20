package forms

import (
	"strconv"
	"strings"

	"charm.land/huh/v2"
)

// NewActionForm builds a Huh form for the given action id, binding into v.
func NewActionForm(action string, v *ActionValues) *huh.Form {
	if v.Network == "" {
		v.Network = "preprod"
	}
	if v.Format == "" {
		v.Format = "both"
	}
	if v.Collateral == "" {
		v.Collateral = "5000000"
	}
	if v.Actor == "" {
		v.Actor = "bootstrap"
	}
	if v.FromActor == "" {
		v.FromActor = "bootstrap"
	}
	if v.SLDOwner == "" {
		v.SLDOwner = "sldOwner"
	}
	if v.Provider == "" {
		v.Provider = "blockfrost"
	}

	var groups []*huh.Group
	switch action {
	case "wallet.create":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.Name),
			huh.NewSelect[string]().Title("Network").Options(
				huh.NewOption("preprod", "preprod"),
			).Value(&v.Network),
			huh.NewSelect[string]().Title("Format").Options(
				huh.NewOption("both", "both"),
				huh.NewOption("key-envelope", "key-envelope"),
				huh.NewOption("mnemonic", "mnemonic"),
			).Value(&v.Format),
			huh.NewInput().Title("Out dir").Placeholder("runtime/wallets/name").Value(&v.OutDir),
			huh.NewConfirm().Title("Force overwrite").Value(&v.Force),
		)}
	case "wallet.fund":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("From actor").Value(&v.FromActor),
			huh.NewText().Title("Allocations").Description("one per line or comma: actor=lovelace").Value(&v.Allocation),
			huh.NewInput().Title("Collateral lovelace").Value(&v.Collateral),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "wallet.balance":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Actor").Value(&v.Actor),
		)}
	case "proof.generate":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("TLD").Value(&v.TLD),
			huh.NewInput().Title("Out dir").Value(&v.OutDir),
			huh.NewInput().Title("Owner key (optional)").Value(&v.OwnerKey),
		)}
	case "system.prepare":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Blueprint file (plutus.json)").Value(&v.Blueprint),
			huh.NewInput().Title("Stake key / wallet dir").Value(&v.StakeKey),
			huh.NewInput().Title("Network").Value(&v.Network),
			huh.NewInput().Title("Out dir").Value(&v.OutDir),
			huh.NewInput().Title("Aiken binary (optional)").Value(&v.Aiken),
			huh.NewConfirm().Title("Force overwrite").Value(&v.Force),
		)}
	case "system.init":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Deployment JSON").Value(&v.Deployment),
			huh.NewInput().Title("Actor").Value(&v.Actor),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "system.bind":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Base config").Value(&v.BaseConfig),
			huh.NewInput().Title("Deployment JSON").Value(&v.Deployment),
			huh.NewInput().Title("Init tx id").Value(&v.TxID),
			huh.NewInput().Title("Actor dir").Value(&v.ActorDir),
			huh.NewSelect[string]().Title("Provider").Options(
				huh.NewOption("blockfrost", "blockfrost"),
				huh.NewOption("utxorpc", "utxorpc"),
			).Value(&v.Provider),
			huh.NewInput().Title("Out config path").Value(&v.Out),
			huh.NewConfirm().Title("Force overwrite").Value(&v.Force),
		)}
	case "registrar.register":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("TLD").Value(&v.TLD),
			huh.NewInput().Title("Proof bundle").Value(&v.Proof),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "owner.activate":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("TLD").Value(&v.TLD),
			huh.NewInput().Title("Owner key (HNS key JSON)").Value(&v.OwnerKey),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "owner.mint":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("TLD").Value(&v.TLD),
			huh.NewInput().Title("SLD").Value(&v.SLD),
			huh.NewInput().Title("SLD owner actor").Value(&v.SLDOwner),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "owner.update":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("TLD").Value(&v.TLD),
			huh.NewInput().Title("SLD").Value(&v.SLD),
			huh.NewInput().Title("Records JSON").Value(&v.Records),
			huh.NewInput().Title("Out prefix").Value(&v.Out),
		)}
	case "tx.inspect":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Tx envelope").Value(&v.TxPath),
		)}
	case "tx.sign":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Tx envelope").Value(&v.TxPath),
			huh.NewInput().Title("Actor").Value(&v.Actor),
			huh.NewInput().Title("Out path").Value(&v.Out),
			huh.NewConfirm().Title("Allow extra signer").Value(&v.AllowExtra),
		)}
	case "tx.submit":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Signed tx envelope").Value(&v.TxPath),
		)}
	case "tx.status":
		groups = []*huh.Group{huh.NewGroup(
			huh.NewInput().Title("Tx id").Value(&v.TxID),
			huh.NewInput().Title("Manifest (optional)").Value(&v.Manifest),
			huh.NewConfirm().Title("Wait for confirmation").Value(&v.Wait),
		)}
	default:
		groups = []*huh.Group{huh.NewGroup(
			huh.NewNote().Title("Unknown action").Description(action),
		)}
	}

	return huh.NewForm(groups...).WithWidth(48)
}

// ParseAllocations splits allocation text into actor=lovelace tokens.
func ParseAllocations(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", "\n")
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ParseInt64 parses a decimal int64 string with a default on empty.
func ParseInt64(raw string, def int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
