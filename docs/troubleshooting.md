# Troubleshooting

## Diagnosing failures

Use `-v 3` (debug) or `-v 4` (trace) to see provider queries, transaction builder steps, and poll ticks on **stderr** without changing stdout JSON results. Example:

```bash
dns-cli owner mint-sld ... -v 3
```

## Invalid proof

```
invalid registrar proof: signature verification failed
```

- Confirm proof JSON `tld` matches `--tld` after canonicalization
- Verify secp256k1 key/signature hex encoding
- Check owner key in proof matches registration datum

## Missing owner token

```
tld user token: no utxo with asset ...
```

- Complete `activate-tld` first
- Confirm token is at the configured `tldOwner` address

## Duplicate state

```
tld "..." is already registered
sld "..." already exists under tld "..."
```

- Use a unique label for test deployments
- Query chain state before rebuilding

## Unused address / Blockfrost 404

A brand-new wallet with no history often returns HTTP 404 from Blockfrost
(`The requested component has not been found`). dns-cli treats that as an
**empty UTxO set** (0 lovelace), not a hard failure. You should see a clear
hint to fund the address via the Preprod faucet rather than an opaque API error.

Auth failures (401/403) and rate limits (429) still fail fast with guidance.

## Provider mismatch

```
manifest provider "utxorpc" does not match profile "blockfrost"
```

- Submit with the same provider used to build, or rebuild with the target profile

## ExUnit / evaluation failure

- Ensure reference UTxOs are confirmed on-chain
- UTxO RPC cannot evaluate chained/off-chain inputs; all spends must be confirmed

## Collateral failure

```
no suitable collateral utxo
```

- Fund actor with ADA-only UTxO (no tokens, no script ref)

## Reference script mismatch

- Run `config validate --online`
- Confirm `referenceUtxos` match the latest `system init` deployment

## Demo history empty

```
no demo history yet (run a fresh demo first)
```

- Run `dns-cli demo history` from inside the repo (or any cwd under a tree that contains `demo/runs`)
- Or pass `--runs-root path/to/demo/runs` explicitly
- `demo run --mode existing` is the same read-only viewer

## Confirmation timeout

```
confirmation timeout or canceled
```

- Increase `--timeout` or profile `confirmationTimeout`
- Check explorer for transaction status manually

## Transaction expired

- Rebuild with fresh validity interval (new chain tip + TTL)
