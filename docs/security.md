# Security

## Secrets

- **Never** store mnemonics, private keys, or API keys in `config/dns-cli.json` (or any config JSON)
- Use `signingKeyFile` paths or `mnemonicEnv` / `projectIdEnv` / `headersEnv` / `baseUrlEnv` only
- Restrict key file permissions (`chmod 600`); Windows ACL limitations are warned at init
- Automatic **Provider readiness** output shows only endpoint host, env-variable names, and present/missing — never secret values or full credential-bearing URLs

## Preprod demo material

`demo/runs/` intentionally tracks shared Preprod wallets, `.env` snippets, and run artifacts for a revocable end-to-end demo. Those keys are **testnet-only**. Never reuse them on mainnet. Mutating demo commands reject mainnet and Preview profiles.

For production or long-lived test identities, keep secrets out of git and rotate anything that was ever committed.

## Logging and output

- Secrets are not logged on parse or sign failures
- `config show --redact` (default) masks environment-derived values
- Manifests and JSON output exclude secret material
- Human stdout may use ANSI colors; disable with `--no-color` or `NO_COLOR=1`

## Artifacts

Unsigned envelopes contain transaction CBOR only. Manifests store:

- Body hash, required signers (key hashes), expected outputs
- Non-secret config digest, Apollo/contract revisions

## Offline signing

Signing does not rebuild transactions. Witnesses are merged against the existing body hash.

## Incident guidance

If a key is exposed:

1. Stop using the compromised actor
2. Rotate credentials and update config addresses if needed
3. Do not commit production `.skey` files or proof bundles with live signatures to git

## Provider credentials

Blockfrost project IDs, Demeter `DMTR_API_KEY`, and UTxO RPC headers/URLs are loaded from environment at runtime only. See `config/dns-cli.example.json` for the expected variable names.
