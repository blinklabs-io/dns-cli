# Installation

## Prerequisites

- Go **1.25.13+** (module pins toolchain `go1.25.13`)
- Apollo v2 is pulled via `go.mod` as `github.com/Salvionied/apollo/v2` (commit pin; no fork `replace`, no local sibling checkout required)
- Access to Cardano **preview** or **preprod** via UTxO RPC or Blockfrost
- Bursa-compatible signing keys or mnemonics for actors (never stored inline in config)
- For the Preprod demo (`demo/`):
  - [Aiken](https://aiken-lang.org/) on `PATH`, version **≥ 1.1.19** (`aiken --version`)
  - Provider credentials (see Configuration)

## Bootstrap (recommended)

From the repo root, `scripts/setup.*` checks Go, creates `bin/`, and builds dns-cli.
It does **not** scaffold `demo/` — `dns-cli demo run` repairs the demo tree when needed.

```bash
./scripts/setup.sh
./bin/dns-cli version
```

```powershell
.\scripts\setup.ps1
.\bin\dns-cli.exe version
```

Flags: `-y` / `-Yes`, `--skip-build` / `-SkipBuild`. Env: `ASSUME_YES=1`.

## Build from source (manual)

```bash
cd dns-cli
mkdir -p bin
go build -o bin/dns-cli ./cmd/dns-cli
```

Or use the Makefile:

```bash
make build
```

## Verify

```bash
./bin/dns-cli version
./bin/dns-cli version --output json
```

## Windows

```powershell
.\scripts\setup.ps1
.\bin\dns-cli.exe version
```

Add `bin/` (or another install location) to your `PATH` for convenience.

## Configuration

Default config path is `config/dns-cli.json`. Start from the dual-provider example:

```bash
cp config/dns-cli.example.json config/dns-cli.json
```

Or: `dns-cli config init --network preprod --provider blockfrost`.

Provider-dependent commands automatically print a readiness summary and block if credentials or health checks fail. There is no separate `config check` command.

## Provider credentials

| Variable | Provider |
|---|---|
| `DNS_CLI_BLOCKFROST_PROJECT_ID` | Blockfrost |
| `DNS_CLI_UTXORPC_URL` | UTxO RPC (when config uses `baseUrlEnv`) |
| `DMTR_API_KEY` | Optional Demeter API key (sent as `dmtr-api-key`); required for Demeter hosts when headers are empty |
| `DNS_CLI_UTXORPC_HEADERS` | Optional `Key=Value,...` headers |

## Release binaries

Tagged releases publish Windows amd64 and Linux amd64/arm64 artifacts with SHA-256
checksums. Verify checksums before use.
