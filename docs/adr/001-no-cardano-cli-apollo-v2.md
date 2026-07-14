# ADR 001: Apollo v2 Only — No cardano-cli or Apollo v1

## Status

Accepted

## Context

DNS CLI Milestone 4 must build Conway-era Plutus V3 transactions for the Handshake DNS
contracts without shelling out to `cardano-cli`. The repository contains:

- **Apollo v2** at `github.com/Salvionied/apollo/v2` (pinned commit `b2f56d0`)
- **apollo-testing-suite** examples that import legacy Apollo v1 APIs

Mixing v1 and v2 imports causes incompatible types, duplicate module paths, and
behavior that diverges from the contract team's current toolchain.

## Decision

1. Production code in `dns-cli` imports **only** `github.com/Salvionied/apollo/v2`.
2. `cardano-cli` is **not** invoked at runtime for build, sign, or submit.
3. `apollo-testing-suite` is a behavioral reference only; its v1 imports are prohibited
   in `dns-cli` production packages.
4. Offline signing merges witnesses against decoded Conway CBOR; transactions are never
   rebuilt during `tx sign`.

## Consequences

- Transaction builders use Apollo v2 fluent APIs: `New`, `CollectFrom`, `Mint`,
  `AddReferenceInput`, `PayToContract`, `Complete`, `GetTxCbor`.
- CI includes a check that rejects v1 import paths.
- Operators use text-envelope JSON artifacts compatible with Cardano tooling, but the
  CLI remains self-contained.

## Alternatives considered

| Alternative | Rejected because |
|---|---|
| Apollo v1 from testing-suite | Unmaintained API surface; no Plutus V3 reference-input parity |
| cardano-cli subprocess | Violates Go-native milestone goal; poor Windows ergonomics |
| Hybrid v1+v2 | Import cycles and inconsistent transaction encoding |
