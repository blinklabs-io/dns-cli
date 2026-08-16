# dns-cli Preprod demo

Self-contained Cardano **Preprod** demonstration for Handshake DNS lifecycle flows.

Orchestration lives in Go (`dns-cli demo run`). Build the binary from the repo root with [`scripts/setup.sh`](../scripts/setup.sh) / [`scripts/setup.ps1`](../scripts/setup.ps1) first.

Detailed documentation: [docs/demo.md](../docs/demo.md).

## Security warning

Generated wallets under `runs/shared/wallets/` and any generated HNS private keys are **Preprod-only test material**. Never use them on mainnet. Mutating demo commands reject mainnet and Preview profiles.

## Modes

| Mode | Purpose |
|---|---|
| `fresh` (default) | Create wallets, wait for faucet funding, deploy parameterized validators, run register → activate → mint SLD → update DNS |
| `existing` | Numbered picker of local TLD/SLD runs by stage; resume the selected incomplete run (no tx/explorer links in the list). Use `demo history` for the read-only tx/explorer report |

## Layout

```text
demo/
  README.md                   # this quickstart
  config/
    blockfrost.template.json  # system bind base (fresh)
    utxorpc.template.json
    records.json              # DNS records template (copied into each SLD run)
  fixtures/
    contracts/                # vendored plutus.json blueprint for system prepare
  runs/                       # full Preprod demo history + shared test wallets (tracked)
    .gitkeep
    states/                   # tracked JSON schemas
    shared/                   # wallets, .env, tools (Preprod demo material)
    <tld>/                    # contracts, proofs, config, artifacts 00–03, TLD state
    <tld>/<sld>/<runId>/      # run.json, records.json, artifacts 04–05, SLD state
```

## Quick start

From the `dns-cli/` module root (after setup):

```bash
# from repo root:
./scripts/setup.sh
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
./bin/dns-cli demo run                              # auto-finds demo/
./bin/dns-cli demo run --mode fresh --provider blockfrost
./bin/dns-cli demo run                              # resume latest incomplete (fresh)
./bin/dns-cli demo history                          # read-only tx/explorer report
./bin/dns-cli demo run --mode existing              # pick a run by number and continue
```

```powershell
# from repo root:
.\scripts\setup.ps1
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = 'preprod...'
.\bin\dns-cli.exe demo run
.\bin\dns-cli.exe demo history
```

Unset mode / provider / TLD / SLD / log level use numbered menus (or yes-no) in the CLI. Pass flags or `DEMO_MODE` / `DEMO_PROVIDER` / `DEMO_LOG_LEVEL` to skip prompts. `--yes` / `DEMO_ASSUME_YES=1` auto-approves defaults (not the Preprod submission confirm).

## Prerequisites (fresh mode)

- Go 1.25.12+ and a built `dns-cli` (`./scripts/setup.sh` or `.\scripts\setup.ps1`)
- Aiken CLI on `PATH`, **version ≥ 1.1.19** (used to apply/convert the vendored blueprint, not to build it)
- Provider credentials:
  - Blockfrost: `DNS_CLI_BLOCKFROST_PROJECT_ID`
  - UTxO RPC: `DNS_CLI_UTXORPC_URL`, optional `DMTR_API_KEY` / `DNS_CLI_UTXORPC_HEADERS`

The Go runner verifies the demo tree **before any run mode**. If `demo/` config files are missing, it asks to create them (or prints a guide with `--skip-install`); `fixtures/contracts/plutus.json` is a tracked repo file, so a missing/deleted copy is restored with `git checkout`, not regenerated. Fresh mode also requires Aiken ≥ 1.1.19. Bootstrap faucet address is copied to the clipboard when possible (`--no-clipboard` to disable).

Bootstrap needs **≥ 150 ADA** from the [Preprod faucet](https://docs.cardano.org/cardano-testnets/tools/faucet/) before fund/deploy.

## Resume behavior

**Fresh re-run**

- **TLD** steps (`fund`, `deploy`, `register`, `activate`) resume from `runs/<tld>/state.json`
- **SLD** steps: if the latest `runs/<tld>/<sld>/<runId>/` is incomplete, that run is resumed; otherwise a new `yyyyMMdd-HHmmss` run folder is created

**Existing mode**

- Lists all local runs with stage (including `bind` when `config/<provider>.json` is missing)
- Selection loads the **exact** `runId` (never invents a newer folder)
- Provider readiness is checked before submissions; failures block continuation
- `--yes` does not auto-select a run and does not skip the submission confirm

## Provenance

| Source | Destination |
|---|---|
| `dns-contracts/onchain/plutus.json` | `demo/fixtures/contracts/plutus.json` (manual copy when contracts change) |

DNS record samples for fresh runs come from `demo/config/records.json`. `fixtures/` must not be written by runners. See [docs/demo.md](../docs/demo.md) for full operator guidance.
