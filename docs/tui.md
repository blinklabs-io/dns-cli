# dns-cli dashboard (TUI)

Interactive Bubble Tea dashboard for operators. Cobra commands remain the automation API (`--output json`).

## Start

```powershell
go build -o bin/dns-cli.exe ./cmd/dns-cli
.\bin\dns-cli.exe dashboard
.\bin\dns-cli.exe dashboard --config path\to\dns-cli.json
```

```bash
go build -o bin/dns-cli ./cmd/dns-cli
./bin/dns-cli dashboard --config dns-cli.json
```

Without `--config`, the workbench shows a path picker. No network/actions run until a config is loaded.

## Layout A

- **Header:** `dns-cli` title, version, commit, build date, config path, network, provider, health, actor, wait badge
- **Left:** Actions (wallet, proof, system, registrar, owner, tx)
- **Center:** Config picker / forms / confirm modal
- **Right:** TLD, SLD, balances, checklist, wait, artifact, txId, body hash, errors
- **Bottom:** Redacted activity log

Press `i` for Apollo/contract/Go revisions. Press `?` for keys.

## Keys

| Key | Action |
|---|---|
| Tab / Shift+Tab | Cycle panes |
| Enter | Select action / confirm path / run |
| Exit (Actions) | Quit the dashboard |
| y | Copy txId (else explorer, else artifact) |
| r | Refresh config + online validate (soft) |
| Esc | Cancel form / confirm |
| c | Cancel wait (standalone `tx status --wait` TTY UI) |
| q / Ctrl+C | Quit |

## Wired interactive actions

Enter an Action to open its Huh form, then complete the form to run via the shared ops façade:

- **Wallet:** Create, Fund, Balance
- **Proof:** Generate
- **System:** Prepare, Init, Bind
- **Registrar:** Register TLD
- **Owner:** Activate TLD, Mint SLD, Update SLD
- **Tx:** Inspect, Sign, Submit, Status (wait)
- **Exit:** Quit the dashboard

Mutating actions show a confirm step (`y` / Esc). Offline actions (wallet create, proof generate, system prepare/bind) do not require a loaded config; others do.

## Wait UI

Interactive TTY waits (including `tx status --wait`) use Bubble Tea. `--output json` and non-TTY keep plain newline status lines.

## Secrets

Prefer actor key paths / env from config. Optional masked mnemonic paste is allowed for wallet create; secrets are never written to the activity log.

## Debugging

```powershell
$env:DEBUG = "1"
# tea.LogToFile can be added when diagnosing TUI issues
```
