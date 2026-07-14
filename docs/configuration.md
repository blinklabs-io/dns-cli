# Configuration

dns-cli uses versioned JSON profiles. Generate a starter file:

```bash
dns-cli config init --network preprod --provider blockfrost --config dns-cli.json
```

See `examples/config.preprod.json` / `examples/config.preview.json` for the full shape.
Demo templates live under `demo/config/`.

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

Exactly one of `baseURL` or `baseUrlEnv` must resolve. Optional headers are `Key=Value,...` in `DNS_CLI_UTXORPC_HEADERS`.

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

For Preprod demos, generate actors with `dns-cli wallet create --network preprod`.

## Flag precedence

1. Command flags (`--network`, `--provider`, `--artifact-dir`)
2. Secret environment variables
3. Selected JSON profile
4. Built-in defaults

Empty flags do not erase valid config values.

## Validation

```bash
dns-cli config validate --config dns-cli.json
dns-cli config validate --config dns-cli.json --online
```

`--online` constructs the provider, checks tip health, and confirms every configured reference UTxO exists on-chain.
