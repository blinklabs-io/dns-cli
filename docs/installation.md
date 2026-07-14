# Installation

## Prerequisites

- Go **1.25.7+** (module pins toolchain `go1.25.12`)
- Local Apollo v2 checkout at `../apollo` (required by `go.mod` `replace`)
- Access to Cardano **preview** or **preprod** via UTxO RPC or Blockfrost
- Bursa-compatible signing keys or mnemonics for actors (never stored inline in config)
- For the Preprod demo (`demo/`):
  - [Aiken](https://aiken-lang.org/) on `PATH` (`aiken --version`)
  - `jq` for the Bash runner
  - Provider credentials (see Configuration)

## Build from source

```bash
cd dns-cli
# Apollo must exist at ../apollo relative to this module
go build -o dns-cli ./cmd/dns-cli
```

Or use the Makefile:

```bash
make build
```

## Verify

```bash
./dns-cli version
./dns-cli version --output json
```

## Windows

```powershell
go build -o dns-cli.exe ./cmd/dns-cli
.\dns-cli.exe version
```

Add the directory containing `dns-cli` to your `PATH` for convenience.

## Provider credentials

| Variable | Provider |
|---|---|
| `DNS_CLI_BLOCKFROST_PROJECT_ID` | Blockfrost |
| `DNS_CLI_UTXORPC_URL` | UTxO RPC (when config uses `baseUrlEnv`) |
| `DNS_CLI_UTXORPC_HEADERS` | Optional `Key=Value,...` headers |

## Release binaries

Tagged releases publish Windows amd64 and Linux amd64/arm64 artifacts with SHA-256
checksums. Verify checksums before use.
