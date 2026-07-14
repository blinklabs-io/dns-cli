# Commands

## Global flags

| Flag | Description |
|---|---|
| `--config` | Path to JSON config (default `dns-cli.json`) |
| `--network` | Profile override (`preview` \| `preprod`) |
| `--provider` | Provider override (`utxorpc` \| `blockfrost`) |
| `--output` | `human` (default) or `json` |
| `--timeout` | Operation timeout (default `10m`) |
| `-v`, `--verbose` | Log verbosity `0`–`4` (default `2`): `0` error, `1` warn, `2` info, `3` debug, `4` trace |
| `--artifact-dir` | Override artifact output directory |
| `--no-color` | Disable ANSI colors in human output and logs |

Diagnostic logs always go to **stderr**. With `--output json`, command results stay on stdout and logs use JSON on stderr (no ANSI).

## Command tree

```
dns-cli version
dns-cli config init|show|validate
dns-cli wallet create|fund|balance
dns-cli proof generate
dns-cli system prepare|init|bind
dns-cli registrar register-tld
dns-cli owner activate-tld|mint-sld|update-sld
dns-cli tx inspect|sign|submit|status
```

## Wallet

```bash
dns-cli wallet create --name bootstrap --network preprod --format key-envelope --out-dir runtime/wallets/bootstrap
dns-cli wallet fund --config bootstrap.json --from-actor bootstrap \
  --allocation registrar=30000000 --allocation tldOwner=50000000 --allocation sldOwner=30000000 \
  --collateral 5000000 --out artifacts/00-fund
dns-cli wallet balance --config bootstrap.json --actor bootstrap --output json
```

`wallet create` supports `--format key-envelope|mnemonic|both` (preprod only). Generated keys must never be used on mainnet.

## Proof

```bash
dns-cli proof generate --tld demo-name --out-dir runtime/proofs
```

Writes `registrar.hns`, `owner.hns`, and `proof-bundle.json` with secp256k1 compact signatures over `blake2b_256(tld)`. Optional `--registrar-key` / `--owner-key` reuse existing `.hns` files.

## System bootstrap

```bash
dns-cli system prepare \
  --blueprint demo/fixtures/contracts \
  --registrar-hns-key runtime/proofs/registrar.hns \
  --stake-key runtime/wallets/bootstrap/stake.vkey \
  --network preprod \
  --out-dir runtime/contracts

dns-cli system init --config bootstrap.json --deployment runtime/contracts/deployment.json \
  --actor bootstrap --out artifacts/01-deploy

dns-cli system bind \
  --base-config bootstrap.json \
  --deployment runtime/contracts/deployment.json \
  --tx-id <deployTxId> \
  --actor-dir runtime/wallets \
  --provider blockfrost \
  --out runtime/config/blockfrost.json
```

`system prepare` invokes Aiken to apply validator parameters in order: registrar key → registrar policy → TLD policy → SLD policy.

## Protocol flow

```bash
# 1. Register TLD (registrar)
dns-cli registrar register-tld --tld NAME --proof proof.json --out artifacts/register

# 2. Sign and submit
dns-cli tx sign --tx artifacts/register.unsigned.json --actor registrar --out artifacts/register.signed.json
dns-cli tx submit --tx artifacts/register.signed.json --output json
dns-cli tx status --tx-id <TXID> --manifest artifacts/register.manifest.json --wait

# 3. Activate TLD (tld owner)
dns-cli owner activate-tld --tld NAME --proof proof.json --out artifacts/activate

# 4. Mint SLD
dns-cli owner mint-sld --tld NAME --sld LABEL --sld-owner sldOwner --out artifacts/mint-sld

# 5. Update DNS records
dns-cli owner update-sld --tld NAME --sld LABEL --records records.json --out artifacts/update
```

Use `--output json` on any command for script-friendly envelopes.

## End-to-end demo

See [`demo/README.md`](../demo/README.md) for the resumable Bash/PowerShell Preprod runners.

## JSON result envelope

Successful commands print:

```json
{
  "ok": true,
  "command": "owner mint-sld",
  "network": "preprod",
  "operation": "mint-sld",
  "artifact": "artifacts/mint-sld.unsigned.json",
  "message": "built unsigned mint-sld transaction"
}
```

Logs go to stderr; results go to stdout.
