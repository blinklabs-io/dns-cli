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

## Address API lag after a confirmed tx

```
registration not found: no utxo with asset ...
Address has no on-chain history; treating as empty UTxO set
no suitable collateral utxo
```

Tx confirmation (Blockfrost **tx** UTxOs) can succeed before the **address**
UTxO index catches up. dns-cli waits for:

| After | Before next build |
| --- | --- |
| `tx apply` confirm | signer funding UTxOs refresh (spent inputs gone) |
| deploy | reference script UTxOs (demo + `UtxoByRef` poll) |
| register / activate / mint | expected script/user tokens (`WaitByAsset`) |
| fund / any actor build | actor funding visible (`EnsureFundingVisible`) |

Re-run the failed command if you interrupted a wait; do not redeploy unless
the TLD is genuinely missing on-chain.

## Provider mismatch

```
manifest provider "utxorpc" does not match profile "blockfrost"
```

- Submit with the same provider used to build, or rebuild with the target profile

## ExUnit / evaluation failure

```
ExUnit estimation failed: EvaluateTx failed: script evaluation returned no results
ExUnit estimation failed: … Phase1ValidationRejected(… PreservationOfValue …)
```

- Ensure reference UTxOs are confirmed on-chain (check deploy tx on explorer)
- UTxO RPC cannot evaluate chained/off-chain inputs; all spends must be confirmed
- After deploy, the UTxO RPC **EvalTx** service can lag behind **ReadUtxos**; wait and re-run the failed step if evaluation still fails
- Apollo balances and converges the EvalTx draft (change/fee/collateral/ExUnits) and sends real required-signer witnesses; UTxO RPC surfaces `TxEval.errors` and validates redeemer purposes against the submitted transaction
- If the error lists `script evaluation failed: …` with a Plutus/script message, that is a real script failure (bad datum, redeemer, missing ref script, etc.)
- Blockfrost is more forgiving for demos when EvalTx indexing is slow

## Collateral failure

```
no suitable collateral utxo
```

- Fund actor with ADA-only UTxO (no tokens, no script ref)

## Reference script mismatch

- Run `config validate --online`
- Confirm `referenceUtxos` match the latest `system init` deployment

## Provider readiness failures

```
environment variable DNS_CLI_BLOCKFROST_PROJECT_ID is required
environment variable DNS_CLI_UTXORPC_URL is required
provider health (blockfrost): …
```

- Export the env var named in the readiness panel (never put the secret in JSON)
- For UTxO RPC on Demeter (or when `headersEnv` is configured), set `DMTR_API_KEY` or `DNS_CLI_UTXORPC_HEADERS`
- Copy `config/dns-cli.example.json` → `config/dns-cli.json` and select the matching profile (`--network preprod-utxorpc` for UTxO RPC)
- Auth/timeout/health failures exit with code **5** (`provider`); missing env vars exit **3** (`config`)

## Demo history empty

```
no demo history yet (run a fresh demo first)
```

- Run `dns-cli demo history` from inside the repo (or any cwd under a tree that contains `demo/runs`)
- Or pass `--runs-root path/to/demo/runs` explicitly

## Demo existing: no resumable runs / completed selection

```
no local TLD/SLD demo runs found under …
Run is complete and cannot be resumed
```

- `demo run --mode existing` needs at least one SLD run state under `runs/<tld>/<sld>/<runId>/`
- Completed runs appear in the list but cannot be selected — use `demo run --mode fresh` for a new SLD run
- Malformed or conflicting state files fail with validation errors (path included)

## Demo history vs existing

- `demo history` — read-only tx IDs + explorer URLs
- `demo run --mode existing` — numbered stage picker and exact-run continuation

## Confirmation timeout / stuck wait (especially UTxO RPC)

```
confirmation timeout or canceled
transaction validity expired: tip slot … >= ttl …
```

UTxO RPC confirmation uses `WaitForTx` plus `ReadUtxos`. If the tx never lands
(dropped from mempool or **TTL expired**), polling used to look “stuck” until
the full confirmation timeout. dns-cli now aborts when tip ≥ manifest TTL.

- Check the explorer URL for the tx id — if missing, rebuild and resubmit
- Default `ttlSlots` is 900 (~15m); raise it in the profile if Preprod is slow
- Prefer `log level normal` during demo waits (`extensive` floods the wait UI)
- Increase `--timeout` / `confirmationTimeout` only after TTL is sufficient

## Transaction expired

- Rebuild with a fresh validity interval (new tip + TTL); do not reuse a stale unsigned artifact
