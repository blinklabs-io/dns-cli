# Commands

## Global flags

| Flag | Description |
|---|---|
| `--config` | Path to JSON config (default `dns-cli.json`) |
| `--network` | Profile override (`preview` \| `preprod`). Demo orchestration is **preprod-only**; `preview` is for non-demo protocol work when your config allows it |
| `--provider` | Provider override (`utxorpc` \| `blockfrost`) |
| `--output` | `human` (default) or `json` |
| `--timeout` | Operation timeout (default `20m`) |
| `-v`, `--verbose` | Log verbosity `0`–`4` (default `2`): `0` error, `1` warn, `2` info, `3` debug, `4` trace |
| `--artifact-dir` | Override artifact output directory |
| `--no-color` | Disable ANSI colors in human output and logs |

Diagnostic logs always go to **stderr**. With `--output json`, command results stay on stdout and logs use JSON on stderr (no ANSI). Human (`--output human`) results use colored panels by default; disable with `--no-color` or `NO_COLOR=1`.

Missing required flags (including Cobra `MarkFlagRequired`) exit with code **2** (`usage`), not `1` (`internal`).

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

## Dashboard

`dns-cli dashboard` opens the Bubble Tea Layout A operator UI (identity header, actions, status wall, activity log). Pass `--config` or pick a path in the TUI before network/actions. See [tui.md](tui.md).

Interactive TTY waits (for example `tx status --wait`) use a Bubble Tea status view. `--output json` and non-TTY keep plain status lines.

## Wallet

```bash
dns-cli wallet create --name bootstrap --network preprod --format both --out-dir runtime/wallets/bootstrap
dns-cli wallet fund --config bootstrap.json --from-actor bootstrap \
  --allocation registrar=30000000 --allocation tldOwner=50000000 --allocation sldOwner=30000000 \
  --collateral 5000000 --out artifacts/00-fund
dns-cli wallet balance --config bootstrap.json --actor bootstrap --output json
dns-cli wallet wait-funds --config bootstrap.json --actor bootstrap \
  --min-lovelace 150000000 --poll 20s --output json
```

`wallet create` supports `--format key-envelope|mnemonic|both` (default: `both`, preprod only). The `both` and `mnemonic` formats write `mnemonic.json` (BIP39 phrase + derivation metadata) alongside key files so wallets can be re-imported later. Generated keys must never be used on mainnet.

`wallet wait-funds` polls `wallet balance` until the actor holds at least `--min-lovelace` (default 150 ADA). Unused-address 404s are treated as zero; auth/config failures abort. Omit global `--timeout` to wait until canceled.

For builders (`wallet fund`, `registrar`, `owner`, `system init`), `--out` is a **path prefix**: the CLI writes `<out>.unsigned.json` and `<out>.manifest.json`. JSON results include `data.out`, `data.unsigned`, and `data.manifest`.

## Proof

```bash
dns-cli proof generate --tld demo-name --out-dir runtime/proofs
```

Writes `registrar.hns`, `owner.hns`, and `proof-bundle.json` with secp256k1 compact signatures over `blake2b_256(tld)`. Optional `--registrar-key` / `--owner-key` reuse existing `.hns` files. `--registrar-hns-key` is an alias for `--registrar-key`.

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

`system prepare` invokes Aiken to apply validator parameters in order: registrar key → registrar policy → TLD policy → SLD policy. `--registrar-key` is an alias for `--registrar-hns-key`.

## Protocol flow

```bash
# 1. Register TLD (registrar)
dns-cli registrar register-tld --tld NAME --proof proof.json --out artifacts/register

# 2. Sign, submit, and wait (one shot)
dns-cli tx apply --tx artifacts/register.unsigned.json --actor registrar \
  --signed artifacts/register.signed.json --manifest artifacts/register.manifest.json

# Or step-by-step:
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

`tx apply` composes sign → submit → status `--wait` and prints the confirmed `txId`. Use `--output json` on any command for script-friendly envelopes.

Flag aliases (UX only): `tx sign --signed` ≡ `--out`; `tx apply --out` ≡ `--signed`.

## Demo

```bash
dns-cli demo history
dns-cli demo history --output json
# optional override:
dns-cli demo history --runs-root demo/runs

dns-cli demo run --demo-root demo \
  --mode fresh --provider blockfrost --tld mytld --sld www
dns-cli demo run --demo-root demo --mode existing
```

`demo history` auto-detects `demo/runs` by walking upward from the current directory (override with `--runs-root`). It is a read-only scan of `runs/<tld>/state.json` and nested SLD run state (skips `shared/` and `states/`).

`demo run` owns the full Preprod orchestration (prereq checks for demo layout + dns-contracts + Aiken, wallets, faucet wait, prepare/deploy, register → update, resume). `--yes` auto-approves install/default prompts but **never** skips `Proceed with Preprod submissions?`. Missing contracts are cloned from `https://github.com/blinklabs-io/dns-contracts.git` when the operator agrees.

## End-to-end demo

See [`demo/README.md`](../demo/README.md) and [`demo.md`](demo.md). Prefer `dns-cli demo run`; `demo/scripts/run-demo.ps1` / `run-demo.sh` are thin flag-mapping wrappers.

## JSON result envelope

Successful commands print:

```json
{
  "ok": true,
  "command": "owner mint-sld",
  "network": "preprod",
  "operation": "mint-sld",
  "artifact": "artifacts/mint-sld.unsigned.json",
  "message": "built unsigned mint-sld transaction",
  "data": {
    "tld": "NAME",
    "sld": "LABEL",
    "sldOwner": "sldOwner",
    "out": "artifacts/mint-sld",
    "unsigned": "artifacts/mint-sld.unsigned.json",
    "manifest": "artifacts/mint-sld.manifest.json"
  }
}
```

Logs go to stderr; results go to stdout. Builders include `data.out` / `data.unsigned` / `data.manifest` so orchestrators do not need to guess sibling paths. `tx sign|submit|status|apply` similarly put paths and status under `data`.
