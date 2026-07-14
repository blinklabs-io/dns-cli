# dns-cli

Go-native CLI for Handshake DNS on Cardano. Builds unsigned Conway-era transactions
for TLD registration, activation, SLD minting, and DNS record updates using
[Apollo v2](https://github.com/Salvionied/apollo) — no `cardano-cli` at runtime.

## Quick start

```bash
go build -o dns-cli ./cmd/dns-cli
./dns-cli config init --network preprod --provider blockfrost
./dns-cli config validate
./dns-cli version --output json
```

Windows:

```powershell
go build -o dns-cli.exe ./cmd/dns-cli
.\dns-cli.exe version
```

Requires a local Apollo checkout at `../apollo` (see `go.mod` replace).

## Protocol flow

1. `registrar register-tld` — mint registration NFT (`minted = 0`)
2. `owner activate-tld` — separate activation (`minted = 1`, TLD token pair)
3. `owner mint-sld` — atomic parent list update + SLD token pair
4. `owner update-sld` — replace full DNS record set

Each domain command writes an unsigned text envelope and manifest. Use `tx sign`,
`tx submit`, and `tx status` for the offline lifecycle.

Bootstrap helpers:

- `wallet create|fund|balance`
- `proof generate`
- `system prepare|init|bind` (Aiken for parameterization only)

## Preprod demo

Self-contained fresh/existing runners live in [`demo/`](demo/README.md):

```powershell
cd demo
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = '...'
.\run-demo.ps1 -Mode fresh -Provider blockfrost
```

Keys under `demo/fixtures/preprod/wallets/` are **public Preprod-only fixtures** — never use on mainnet.

## Documentation

- [Installation](docs/installation.md)
- [Configuration](docs/configuration.md)
- [Commands](docs/commands.md)
- [Offline transactions](docs/offline-transactions.md)
- [Operator guide](docs/operator-guide.md)
- [Protocol crosswalk](docs/protocol-crosswalk.md)
- [Security](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md)
- [ADR: Apollo v2 only](docs/adr/001-no-cardano-cli-apollo-v2.md)
- [Preprod demo](demo/README.md)

## Stack

- **Apollo v2** — transaction construction (local `../apollo` replace)
- **Bursa** — wallet and signing
- **UTxO RPC / Blockfrost** — selectable chain providers
- **Aiken** — validator build/parameter apply for `system prepare`
- **dns-contracts** — on-chain source of truth

## Examples

See `examples/` for starter config, proof bundle, and records JSON.

## Tests

```bash
go test ./...
go test -fuzz ./internal/domain -fuzztime 5s
go test -tags integration ./...   # requires DNS_CLI_RUN_LIVE=1
```

## Status

CLI, config, providers, wallet, proof generation, system bootstrap, protocol encoding,
chain query, transaction builders, offline artifacts, Preprod demo runners, docs, and CI
are in place. Live Preprod acceptance requires provider credentials and faucet funding.
