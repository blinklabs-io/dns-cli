# Offline transactions

dns-cli domain commands build **unsigned** Cardano text envelopes. Signing, transport,
and submission are separate steps.

## Artifacts

Each build writes:

1. `<prefix>.unsigned.json` — Conway-era text envelope (`type`, `description`, `cborHex`)
2. `<prefix>.manifest.json` — operation metadata, body hash, required signers, expected outputs

## Ceremony

1. **Build** on a networked machine with provider access
2. **Inspect** with `dns-cli tx inspect --tx FILE`
3. **Transport** unsigned envelope to signing environment (USB, secure share, etc.)
4. **Sign** with `dns-cli tx sign --tx FILE --actor NAME --out SIGNED`
5. Repeat sign for multi-signer flows (e.g. SLD mint)
6. **Submit** with `dns-cli tx submit --tx SIGNED`
7. **Confirm** with `dns-cli tx status --tx-id HASH --manifest FILE --wait`

## Integrity checks

`tx sign` verifies:

- Body hash matches manifest (when present)
- Signer is listed in `requiredSigners` unless `--allow-extra-signer`
- Duplicate witnesses are deduplicated

`tx submit` rejects unsigned envelopes and manifest/profile mismatches.

## Expiration

Manifests record validity interval slots. Submit before TTL expiry or rebuild.

## Air-gapped signing

The signing machine needs only:

- `dns-cli` binary
- Actor key file or mnemonic environment variable
- Unsigned text envelope + manifest

No provider credentials are required on the signer.
