# dns-cli

Go-native CLI for Handshake DNS on Cardano (**v1.0.0**). Builds unsigned Conway-era
transactions for TLD registration, activation, SLD minting, and DNS record updates
using [Apollo v2](https://github.com/Salvionied/apollo) — no `cardano-cli` at runtime.

Human stdout uses colored panels and step roadmaps by default. Use `--no-color` or
`NO_COLOR=1` for plain text; `--output json` for machine-readable results on stdout
(logs stay on stderr).

## Quick start

Requires **Go 1.25.10+** (module pins toolchain `go1.25.12`). Apollo resolves via
`go.mod` `replace` — no local sibling checkout needed for a normal build.

```bash
./scripts/setup.sh
./bin/dns-cli version
./bin/dns-cli config init --network preprod --provider blockfrost
./bin/dns-cli config validate
./bin/dns-cli dashboard --config dns-cli.json
```

Windows:

```powershell
.\scripts\setup.ps1
.\bin\dns-cli.exe version
.\bin\dns-cli.exe dashboard --config dns-cli.json
```

`scripts/setup.*` checks Go, creates `bin/`, and builds the binary. Interactive dashboard: [docs/tui.md](docs/tui.md).

## Command tree

```
dns-cli version
dns-cli config init|show|validate
dns-cli wallet create|fund|balance|wait-funds
dns-cli proof generate
dns-cli system prepare|init|bind
dns-cli registrar register-tld
dns-cli owner activate-tld|mint-sld|update-sld
dns-cli tx inspect|sign|submit|status|apply
dns-cli demo history|run
dns-cli dashboard
```

Full flag reference: [docs/commands.md](docs/commands.md).

## Protocol flow

1. `registrar register-tld` — mint registration NFT (`minted = 0`)
2. `owner activate-tld` — separate activation (`minted = 1`, TLD token pair)
3. `owner mint-sld` — atomic parent list update + SLD token pair
4. `owner update-sld` — replace full DNS record set

Each domain command writes an unsigned text envelope and manifest. Use `tx apply`
(or `tx sign` → `submit` → `status`) for the offline lifecycle.

Bootstrap helpers:

- `wallet create|fund|balance|wait-funds`
- `proof generate`
- `system prepare|init|bind` (Aiken for parameterization only)

## Preprod demo

End-to-end Preprod orchestration is `dns-cli demo run`. Quickstart: [`demo/README.md`](demo/README.md). Full guide: [`docs/demo.md`](docs/demo.md).

```bash
./scripts/setup.sh
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
./bin/dns-cli demo run                    # auto-finds demo/; prompts for unset options
./bin/dns-cli demo history                # auto-finds demo/runs
./bin/dns-cli demo run --mode existing
```

Starter DNS records live in `demo/config/records.json`. Run history and shared
Preprod wallets under `demo/runs/` are tracked demo material — **never use on mainnet**.

## Documentation

- [Installation](docs/installation.md)
- [Configuration](docs/configuration.md)
- [Commands](docs/commands.md)
- [Interactive TUI](docs/tui.md)
- [Offline transactions](docs/offline-transactions.md)
- [Operator guide](docs/operator-guide.md)
- [Protocol crosswalk](docs/protocol-crosswalk.md)
- [Security](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Preprod demo guide](docs/demo.md)
- [Demo quickstart](demo/README.md)

## Stack

- **Apollo v2** — transaction construction (`go.mod` replace)
- **Bursa** — wallet and signing
- **UTxO RPC / Blockfrost** — selectable chain providers
- **Aiken** — validator build/parameter apply for `system prepare`
- **dns-contracts** — on-chain source of truth
- **Bubble Tea / Lip Gloss** — dashboard and colored human reports

## Tests

```bash
go test ./...
go test -fuzz ./internal/domain -fuzztime 5s
go test -tags integration ./...   # requires DNS_CLI_RUN_LIVE=1
```

CI runs `go mod tidy` (must stay clean), `go vet`, and tests on Linux and Windows.

## Status

**v1.0.0** — CLI, config, providers, wallet, proof generation, system bootstrap,
protocol encoding, chain query, transaction builders, offline artifacts, colored
human reports, interactive dashboard, Preprod demo orchestration (`demo run` /
`demo history`), docs, and CI are released. Live Preprod acceptance still needs
provider credentials and faucet funding.
