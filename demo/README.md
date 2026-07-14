# dns-cli Preprod demo

Self-contained Cardano **Preprod** demonstration for Handshake DNS lifecycle flows.

## Security warning

Copied keys under `fixtures/preprod/wallets/` and any generated HNS private keys are **public, compromised Preprod-only material**. Never use them on mainnet. Mutating demo commands reject mainnet and Preview profiles.

## Modes

| Mode | Purpose |
|---|---|
| `fresh` (default) | Create new wallets, wait for faucet funding, deploy parameterized validators, run register → activate → mint SLD → update DNS |
| `existing` | Read-only inspection of the historical Preprod deployment in `fixtures/preprod` |

## Layout

```text
demo/
  README.md
  state.schema.json           # JSON Schema for runtime/state.json
  run-demo.sh / run-demo.ps1  # resumable runners
  records.json
  config/                     # templates + historical existing-* profiles
  fixtures/
    preprod/                  # byte-for-byte copy of dns-contracts/preprod (immutable)
    contracts/                # Aiken sources + plutus.json for system prepare
  runtime/                    # generated wallets, proofs, validators, artifacts, state
```

## Prerequisites

- Go 1.25.7+ and local Apollo checkout at `../apollo` (see `go.mod` replace)
- Built `dns-cli` binary (`dns-cli.exe` on Windows)
- Aiken CLI on `PATH` (fresh mode)
- `jq` (Bash runner only)
- Provider credentials:
  - Blockfrost: `DNS_CLI_BLOCKFROST_PROJECT_ID`
  - UTxO RPC: `DNS_CLI_UTXORPC_URL` and optional `DNS_CLI_UTXORPC_HEADERS`

## Historical deployment (existing mode)

Reference UTxOs from init tx `ef635b55fce6abc39cd4c843722d9d574cb719114e224f2cd1c8747d5abfc19e`:

| Role | Ref | Policy ID |
|---|---|---|
| tldRegistrar | `#0` | `ea32305e62561a0c0bb69588a936afb6fabd0fb4d2cc2a6c67363e9d` |
| tldReference | `#1` | `694cb48da919e928b3e51c4648f051326ac150eaa9436792ec7a6e35` |
| sldReference | `#2` | `96512d4c426d912ba453014e74a57d655dfb3980154c4de106f69320` |

Addresses live in `fixtures/preprod/validators/*.addr`. Historical tx files are evidence only and are not replayable.

## Funding budget (fresh mode)

The runner waits until the bootstrap wallet holds **≥ 150 ADA**, then allocates:

| Actor | Allocation | Notes |
|---|---|---|
| registrar | 30 ADA | includes 5 ADA collateral + spend |
| tldOwner | 50 ADA | includes 5 ADA collateral + spend |
| sldOwner | 30 ADA | includes 5 ADA collateral + spend |

Remainder covers reference-script deployment fees and buffers.

Fund the printed bootstrap address with the [Cardano Preprod faucet](https://docs.cardano.org/cardano-testnets/tools/faucet/).

## Runners

Both runners are resumable via `runtime/state.json` (see `state.schema.json`). Confirmed steps (`fund`, `deploy`, `register`, `activate`, `mintSld`, `updateSld`) are skipped on re-run. Always parse CLI stdout JSON (`--output json`); logs stay on stderr.

### Bash

```bash
cd demo
chmod +x run-demo.sh

# Fresh deploy (Blockfrost default)
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
./run-demo.sh --mode fresh --provider blockfrost

# Fresh with UTxO RPC and custom labels
export DNS_CLI_UTXORPC_URL=https://...
./run-demo.sh --mode fresh --provider utxorpc --tld demo-acme --sld www

# Resume after interruption (same runtime/state.json)
./run-demo.sh

# Historical read-only
./run-demo.sh --mode existing --provider blockfrost
```

Optional env overrides: `CLI` (path to binary), `DEMO_MODE`, `DEMO_PROVIDER`.

### PowerShell

```powershell
cd demo

# Fresh deploy (Blockfrost default)
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = 'preprod...'
.\run-demo.ps1 -Mode fresh -Provider blockfrost

# Fresh with UTxO RPC and custom labels
$env:DNS_CLI_UTXORPC_URL = 'https://...'
.\run-demo.ps1 -Mode fresh -Provider utxorpc -Tld demo-acme -Sld www

# Resume after interruption
.\run-demo.ps1

# Historical read-only
.\run-demo.ps1 -Mode existing -Provider blockfrost
```

Optional env overrides: `CLI`, `DEMO_MODE`, `DEMO_PROVIDER`.

### Fresh flow (both shells)

1. Resolve `dns-cli` from `CLI` or `../dns-cli` / `../dns-cli.exe`
2. Create wallets under `runtime/wallets/{bootstrap,registrar,tld-owner,sld-owner}` if missing
3. Write `runtime/config/bootstrap.json` (real actor addresses/keys; `REPLACE_ME` policy placeholders allowed offline)
4. Poll `wallet balance` until bootstrap ≥ 150 ADA
5. `proof generate` → `system prepare` (Aiken)
6. Prompt: `Proceed with Preprod submissions? [y/N]`
7. For each unconfirmed step: build → `tx sign` → `tx submit` → `tx status --wait --manifest` → save state atomically
8. After deploy: `system bind` → `runtime/config/{blockfrost|utxorpc}.json`
9. Lifecycle: register → activate → mint-sld → update-sld

Artifacts land under `runtime/artifacts/00-fund` … `05-update-sld`.

## Provenance

| Source | Destination |
|---|---|
| `dns-contracts/preprod/**` | `demo/fixtures/preprod/**` |
| `dns-contracts/onchain/{aiken.toml,aiken.lock,plutus.json,validators,lib}` | `demo/fixtures/contracts/**` |
| `dns-cli/examples/records.json` | `demo/records.json` |

Copy date: 2026-07-14. `fixtures/` must not be written by runners.
