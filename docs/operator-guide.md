# Operator guide

## Roles

| Role | Actor | Responsibilities |
|---|---|---|
| Bootstrap | `bootstrap` | Fund actors and publish system reference scripts |
| Registrar | `registrar` | Register TLDs with Handshake proof |
| TLD owner | `tldOwner` | Activate TLD, mint SLDs under TLD |
| SLD owner | `sldOwner` | Update DNS records for an SLD |

## Preparation (manual)

1. Deploy contract reference scripts with `dns-cli system prepare` + `system init` + `system bind`
2. Fill `config/dns-cli.json` (from `config/dns-cli.example.json`) with policy IDs, addresses, and reference UTxOs
3. Fund actors with ADA for fees and collateral (≥ 5 ADA ADA-only collateral each)
4. Prepare Handshake proof JSON (`dns-cli proof generate` or a hand-built bundle)

## Preparation (automated Preprod demo)

```bash
./scripts/setup.sh
export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod...
./bin/dns-cli demo run --mode fresh --provider blockfrost
```

```powershell
.\scripts\setup.ps1
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = 'preprod...'
.\bin\dns-cli.exe demo run --mode fresh --provider blockfrost
```

See [`demo/README.md`](../demo/README.md) and [`demo.md`](demo.md). The runner waits for faucet funding of the bootstrap wallet (≥ 150 ADA), prompts before submissions, and resumes from `demo/runs/<tld>/state.json` (and nested SLD run state). Inspect prior runs with `dns-cli demo history`.

## Runbook

### Register → Activate → Mint → Update

```bash
dns-cli config validate --online
dns-cli registrar register-tld --tld hello --proof proof.json --out artifacts/01-register
dns-cli tx apply --tx artifacts/01-register.unsigned.json --actor registrar \
  --signed artifacts/01-register.signed.json --manifest artifacts/01-register.manifest.json

dns-cli owner activate-tld --tld hello --proof proof.json --out artifacts/02-activate
dns-cli tx apply --tx artifacts/02-activate.unsigned.json --actor tldOwner \
  --signed artifacts/02-activate.signed.json --manifest artifacts/02-activate.manifest.json

dns-cli owner mint-sld --tld hello --sld www --sld-owner sldOwner --out artifacts/03-mint
dns-cli tx apply --tx artifacts/03-mint.unsigned.json --actor tldOwner \
  --signed artifacts/03-mint.signed.json --manifest artifacts/03-mint.manifest.json

dns-cli owner update-sld --tld hello --sld www --records records.json --out artifacts/04-update
dns-cli tx apply --tx artifacts/04-update.unsigned.json --actor sldOwner \
  --signed artifacts/04-update.signed.json --manifest artifacts/04-update.manifest.json
```

For air-gapped or multi-step ceremonies, use `tx sign` → `tx submit` → `tx status --wait` instead of `tx apply`. See [offline-transactions.md](offline-transactions.md).

## Evidence collection

Record for each step:

- Config validation output
- Unsigned/signed artifact SHA-256
- Transaction ID and explorer link
- Expected output indexes from manifest
- Before/after datum state (via `tx inspect` or chain query)

## Collateral

Script transactions require a separate ADA-only collateral UTxO where possible.
Ensure actors maintain ≥ 5 ADA in an unencumbered UTxO. The demo funder emits
explicit collateral + spend outputs per actor.
