# dns-cli Preprod demo

Self-contained Cardano **Preprod** demonstration for Handshake DNS lifecycle flows.

Orchestration lives in Go (`dns-cli demo run`). The scripts under `scripts/` are thin wrappers that map flags and call the CLI.

Detailed documentation: [docs/demo.md](../docs/demo.md).

## Security warning

Generated wallets under `runs/shared/wallets/` and any generated HNS private keys are **Preprod-only test material**. Never use them on mainnet. Mutating demo commands reject mainnet and Preview profiles.

## Modes

| Mode | Purpose |
|---|---|
| `fresh` (default) | Create wallets, wait for faucet funding, deploy parameterized validators, run register → activate → mint SLD → update DNS |
| `existing` | Read-only summary of local `runs/` history (each TLD + its SLD runs, confirmed tx IDs, explorer URLs). No chain writes |

## Layout

```text
demo/
  README.md                   # this quickstart
  scripts/
    run-demo.ps1 / run-demo.sh
  config/
    blockfrost.template.json  # system bind base (fresh)
    utxorpc.template.json
    records.json              # DNS records template (copied into each SLD run)
  fixtures/
    contracts/                # Aiken sources + plutus.json for system prepare
  runs/                       # full Preprod demo history + shared test wallets (tracked)
    .gitkeep
    states/                   # tracked JSON schemas
    shared/                   # wallets, .env, tools (Preprod demo material)
    <tld>/                    # contracts, proofs, config, artifacts 00–03, TLD state
    <tld>/<sld>/<runId>/      # run.json, records.json, artifacts 04–05, SLD state
```

## Quick start

From the `dns-cli/` module root (or any cwd with an absolute `--demo-root`):

```bash
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
dns-cli demo run --demo-root demo --mode fresh --provider blockfrost
dns-cli demo run --demo-root demo                    # resume
dns-cli demo run --demo-root demo --mode existing    # history
```

From the `demo/` directory via wrappers (prompts for unset options, asks to build into `../bin/` if needed):

```powershell
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = 'preprod...'
.\scripts\run-demo.ps1
.\scripts\run-demo.ps1 -Mode fresh -Provider blockfrost -LogLevel Normal -Yes
.\scripts\run-demo.ps1 -Mode existing
```

```bash
chmod +x scripts/run-demo.sh
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
./scripts/run-demo.sh
./scripts/run-demo.sh --mode fresh --provider blockfrost --log-level normal -y
./scripts/run-demo.sh --mode existing
```

Wrappers resolve `CLI` → `dns-cli/bin/dns-cli(.exe)` → tree root → `PATH`. Interactive prompts (skipped with `-Yes` / `--yes`): mode, provider, TLD/SLD, **log level**, skip-install, clipboard. Missing/outdated binaries ask to compile into `bin/`.
## Prerequisites (fresh mode)

- Go 1.25.10+ (Apollo resolves via `go.mod` replace; no local sibling checkout)
- Built `dns-cli` binary (`bin/dns-cli.exe` on Windows, or wrappers will ask to build it)
- Aiken CLI on `PATH`, **version ≥ 1.1.19** (matches `fixtures/contracts/aiken.toml`)
- Provider credentials:
  - Blockfrost: `DNS_CLI_BLOCKFROST_PROJECT_ID`
  - UTxO RPC: `DNS_CLI_UTXORPC_URL`, optional `DMTR_API_KEY` / `DNS_CLI_UTXORPC_HEADERS`

The Go runner verifies the demo tree and contracts **before any run mode**. If `demo/` assets or `dns-contracts` are missing, it asks to create/pull them (or prints guides with `--skip-install`). Fresh mode also requires Aiken ≥ 1.1.19. Bootstrap faucet address is copied to the clipboard when possible (`--no-clipboard` to disable).

Flags: `-Yes` / `--yes`, `-SkipInstall` / `--skip-install`. Env: `DEMO_ASSUME_YES=1`.

Bootstrap needs **≥ 150 ADA** from the [Preprod faucet](https://docs.cardano.org/cardano-testnets/tools/faucet/) before fund/deploy.

## Resume behavior

- **TLD** steps (`fund`, `deploy`, `register`, `activate`) resume from `runs/<tld>/state.json`
- **SLD** steps: if the latest `runs/<tld>/<sld>/<runId>/` is incomplete, that run is resumed; otherwise a new `yyyyMMdd-HHmmss` run folder is created

## Provenance

| Source | Destination |
|---|---|
| `dns-contracts/onchain/{aiken.toml,aiken.lock,plutus.json,validators,lib}` | `demo/fixtures/contracts/**` |
| Historical `examples/records.json` (removed) | `demo/config/records.json` (canonical starter) |

`fixtures/` must not be written by runners. See [docs/demo.md](../docs/demo.md) for full operator guidance (requirements, expected results, what you can change).
