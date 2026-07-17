# Security

## Secrets

- **Never** store mnemonics, private keys, or API keys in `dns-cli.json`
- Use `signingKeyFile` paths or `mnemonicEnv` / `projectIdEnv` / `headersEnv` only
- Restrict key file permissions (`chmod 600`); Windows ACL limitations are warned at init

## Logging and output

- Secrets are not logged on parse or sign failures
- `config show --redact` (default) masks environment-derived values
- Manifests and JSON output exclude secret material

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
3. Do not commit `.skey` files or proof bundles with live signatures to git

## Provider credentials

Blockfrost project IDs, Demeter `DMTR_API_KEY`, and UTxO RPC headers are loaded from environment at runtime only.
