# dns-cli

`dns-cli` is the Go-native command-line interface for the Cardano-based decentralized DNS system implemented in [`blinklabs-io/dns-contracts`](https://github.com/blinklabs-io/dns-contracts).

The purpose of this repository is to provide an operator CLI for the real on-chain domain flows without depending on `cardano-cli`.

## What This CLI Is For

This CLI is intended to support the Milestone 4 contract integration flows for decentralized DNS on Cardano:

- registrar registers a TLD on Cardano
- TLD owner activates that TLD on Cardano
- TLD owner mints an SLD under that TLD
- SLD owner updates DNS records for that SLD

These flows are based on the on-chain contracts and reference shell scripts in `dns-contracts`, but the implementation in this repository is intended to be fully Go-native.

## Stack

The intended implementation stack for this repository is:

- [`bursa`](https://github.com/blinklabs-io/bursa) for wallet, key, and signing concerns
- [`apollo`](https://github.com/Salvionied/apollo) for Cardano transaction construction and Plutus transaction handling
- [`dingo`](https://github.com/blinklabs-io/dingo) / `utxorpc` for chain query, transaction submission, and confirmation

## Relationship to `dns-contracts`

This repository does not define the on-chain protocol.

The source of truth for the protocol lives in:

- [`blinklabs-io/dns-contracts`](https://github.com/blinklabs-io/dns-contracts)

This CLI is expected to reproduce the behavior of the reference flows there, especially:

- [`scripts/02-register-tld.sh`](https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/02-register-tld.sh)
- [`scripts/03-mint-tld.sh`](https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/03-mint-tld.sh)
- [`scripts/04-mint-sld.sh`](https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/04-mint-sld.sh)

And the corresponding validators:

- [`tld_registrar.ak`](https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_registrar.ak)
- [`tld_reference.ak`](https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_reference.ak)
- [`sld_reference.ak`](https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/sld_reference.ak)

## Expected Command Surface

The planned CLI shape is:

```text
dns-cli config init
dns-cli config show
dns-cli config validate

dns-cli registrar register-tld

dns-cli owner activate-tld
dns-cli owner mint-sld
dns-cli owner update-sld
```

These commands reflect the real protocol flow:

- `register-tld` is a registrar action
- `activate-tld` is an owner action required before SLD minting
- `mint-sld` creates a subdomain under an activated TLD
- `update-sld` updates DNS records for an existing SLD

## Milestone 4 Planning

The current Milestone 4 work breakdown and acceptance criteria are documented in:

- [MILESTONE4_PLAN.md](./MILESTONE4_PLAN.md)

## Current Status

This repository is in the early implementation stage.

The immediate goal is to build the CLI foundation and then implement the TLD and SLD flows described above.
