# Operator guide

## Roles

| Role | Actor | Responsibilities |
|---|---|---|
| Bootstrap | `bootstrap` | Fund actors and publish system reference scripts |
| Registrar | `registrar` | Register TLDs with Handshake proof |
| TLD owner | `tldOwner` | Activate TLD, mint SLDs under TLD |
| SLD owner | `sldOwner` | Update DNS records for an SLD |

## Preparation (manual)

1. Deploy contract reference scripts:
   - Prefer `dns-cli system prepare` + `system init` + `system bind`, or
   - Historical path: `01-init-system.sh` in dns-contracts
2. Fill `dns-cli.json` with policy IDs, addresses, and reference UTxOs
3. Fund actors with ADA for fees and collateral (≥ 5 ADA ADA-only collateral each)
4. Prepare Handshake proof JSON (`dns-cli proof generate` or a hand-built bundle)

## Preparation (automated Preprod demo)

```powershell
cd demo
$env:DNS_CLI_BLOCKFROST_PROJECT_ID = '...'
.\run-demo.ps1 -Mode fresh -Provider blockfrost
```

See [`demo/README.md`](../demo/README.md). The runner waits for faucet funding of a new bootstrap wallet (≥ 150 ADA), prompts before submissions, and resumes from `runtime/state.json`.

## Runbook

### Register → Activate → Mint → Update

```bash
dns-cli config validate --online
dns-cli registrar register-tld --tld hello --proof proof.json --out artifacts/01-register
dns-cli tx sign --tx artifacts/01-register.unsigned.json --actor registrar --out artifacts/01-register.signed.json
dns-cli tx submit --tx artifacts/01-register.signed.json --output json
dns-cli tx status --tx-id <TXID> --manifest artifacts/01-register.manifest.json --wait

dns-cli owner activate-tld --tld hello --proof proof.json --out artifacts/02-activate
# sign + submit + status ...

dns-cli owner mint-sld --tld hello --sld www --sld-owner sldOwner --out artifacts/03-mint
# sign (tldOwner) + submit + status ...

dns-cli owner update-sld --tld hello --sld www --records records.json --out artifacts/04-update
# sign (sldOwner) + submit + status ...
```

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
