# Configuration

dns-cli uses versioned JSON profiles. The default config path is **`config/dns-cli.json`**
(explicit `--config` always wins).

Copy the dual-provider example, or generate a starter:

```bash
cp config/dns-cli.example.json config/dns-cli.json
# or:
dns-cli config init --network preprod --provider blockfrost
```

`config/dns-cli.example.json` contains two Preprod profiles:

| Profile | Provider | Select with |
|---|---|---|
| `preprod-blockfrost` (default) | Blockfrost | `--network preprod-blockfrost` or leave `defaultProfile` |
| `preprod-utxorpc` | UTxO RPC | `--network preprod-utxorpc` |

`dns-cli config init --network mainnet --provider blockfrost` writes a mainnet profile (network id 1, magic 764824073, Blockfrost mainnet URL). `wallet create` stays preprod-only; mainnet signing keys must come from an external or hardware wallet.

Do **not** rely on `--provider` alone to switch profiles: that flag only overrides
`provider.type`, not URL/env fields. Prefer separate profiles as in the example.

`config init` writes the full schema (network, provider, contracts placeholders, actors, transaction defaults) and adjusts relative paths so blueprint/keys/artifacts still resolve from the project root when the file lives under `config/`. Demo bind templates live under `demo/config/` (`blockfrost.template.json`, `utxorpc.template.json`); DNS record samples are in `demo/config/records.json`.

## Profiles

| Field | Description |
|---|---|
| `version` | Schema version (currently `1`) |
| `defaultProfile` | Profile selected when `--network` is omitted |
| `profiles.<name>.network` | Cardano network id, magic, explorer URL template |
| `profiles.<name>.provider` | `utxorpc` or `blockfrost` backend |
| `profiles.<name>.contracts` | Blueprint path, script addresses, policy IDs, reference UTxOs |
| `profiles.<name>.actors` | Named signing actors |
| `profiles.<name>.transaction` | TTL, confirmation polling, artifact directory |

Relative paths for `blueprintPath`, actor `signingKeyFile`, and `artifactDir` are resolved against the **config file directory**, not the process working directory.

## Provider selection

### UTxO RPC

```json
"provider": {
  "type": "utxorpc",
  "baseURL": "https://your-utxorpc-endpoint",
  "headersEnv": "DNS_CLI_UTXORPC_HEADERS"
}
```

Or supply the URL via environment:

```json
"provider": {
  "type": "utxorpc",
  "baseUrlEnv": "DNS_CLI_UTXORPC_URL",
  "headersEnv": "DNS_CLI_UTXORPC_HEADERS"
}
```

Exactly one of `baseURL` or `baseUrlEnv` must resolve.

**Authentication rules (UTxO RPC readiness):**

- A generic endpoint with **no** `headersEnv` may run without auth.
- When `headersEnv` is configured **or** the host looks like Demeter (`demeter` / `dmtr`), auth is required via a non-empty `headersEnv` value **or** `DMTR_API_KEY`.
- `DMTR_API_KEY` is applied as the `dmtr-api-key` header (matching the [utxorpc go-sdk](https://github.com/utxorpc/go-sdk) convention). Optional extra headers are `Key=Value,...` in `DNS_CLI_UTXORPC_HEADERS`; an explicit `dmtr-api-key` there wins over `DMTR_API_KEY`.

### Blockfrost

```json
"provider": {
  "type": "blockfrost",
  "baseURL": "https://cardano-preprod.blockfrost.io/api/v0",
  "projectIdEnv": "DNS_CLI_BLOCKFROST_PROJECT_ID"
}
```

Never embed API keys in config. Use environment variables only.

## Actors

Each actor requires exactly one credential source:

- `signingKeyFile` — Cardano CLI text envelope (`.skey`)
- `mnemonicEnv` — environment variable holding a Bursa mnemonic

Optional `accountId` / `addressId` apply to mnemonic-derived wallets.

For Preprod demos, generate actors with `dns-cli wallet create --network preprod` (or let `demo run` create them under `demo/runs/shared/wallets/`).

## Flag precedence

1. Command flags (`--network`, `--provider`, `--artifact-dir`)
2. Secret environment variables
3. Selected JSON profile
4. Built-in defaults

Empty flags do not erase valid config values.

## Provider readiness (automatic)

There is **no** separate `config check` command. Whenever a command needs the
chain provider, dns-cli loads the config and prints a secret-safe **Provider readiness**
summary (provider, network, endpoint **host**, endpoint source / env names,
credential present/missing, health ready/failed). Secrets and full URLs are never printed.

Missing credentials or failed health **block** the operation (`exit 3` config / `exit 5` provider).

Offline commands (`config init`, `config show`, offline `config validate`, `tx inspect`,
`tx sign`, `wallet create`, `proof generate`, `system prepare`, `system bind`) do **not**
require credentials or network access.

## Validation

```bash
dns-cli config validate                          # default: config/dns-cli.json
dns-cli config validate --config config/dns-cli.json --online
dns-cli config validate --config config/dns-cli.example.json --network preprod-utxorpc
```

Offline validate checks schema only. `--online` first runs the automatic readiness
check, then confirms every configured reference UTxO exists on-chain.
