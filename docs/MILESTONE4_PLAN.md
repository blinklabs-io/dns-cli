# Milestone 4 Plan

> **Detailed implementation:** See [MILESTONE4_DETAILED_PLAN.md](./MILESTONE4_DETAILED_PLAN.md) for the extensive multi-phase implementation specification.

> **Implementation status (2026-07):** Milestone 4 CLI foundation is implemented in this repository — Cobra command tree, Apollo v2 transaction builders, config/providers, offline artifacts, documentation, examples, and CI workflows. Live preview/preprod runs require operator config, funded actors, and `DNS_CLI_RUN_LIVE=1` for integration tests.

This document outlines the proposed work breakdown for Milestone 4 of the decentralized DNS project.

The goal of this milestone is to build a Go-native CLI in this repository that reproduces the required on-chain flows already demonstrated in the reference contracts repository:

- `https://github.com/blinklabs-io/dns-contracts`

This CLI should not depend on `cardano-cli`.

For this plan, we assume the following Go-native stack is the intended implementation stack:

- `bursa` for wallet, key, and signing concerns
- `apollo` for Cardano transaction construction and Plutus-related transaction handling
- `dingo` / `utxorpc` for chain query, submission, and transaction status checks

## Milestone 4 Goal

Milestone 4 is about delivering a CLI that supports the real contract flow.

At a high level, the CLI must support:

1. a registrar registering a TLD on Cardano
2. a TLD owner activating that TLD on Cardano
3. a TLD owner minting an SLD under that TLD
4. an SLD owner updating DNS records for that SLD
5. documentation showing how to use the CLI

## Important Notes for First-Time Contributors

Some protocol terms sound similar but mean different things.

- `register TLD`:
  The registrar anchors a Handshake TLD on Cardano. This creates the initial registrar-side on-chain record.
- `activate TLD`:
  The TLD owner takes that registered TLD and turns it into an operational TLD on Cardano by minting the initial TLD token pair.
- `mint SLD`:
  The TLD owner creates a subdomain under an activated TLD.
- `update SLD`:
  The SLD owner updates DNS records stored for that subdomain.

These are separate protocol steps. They are not interchangeable.

## Assumptions

- One-time chain initialization is already done outside this milestone.
- Handshake-side proof generation can be simulated for this milestone with a static proof bundle or hardcoded signatures.
- The reference implementation for the transaction flow lives in `blinklabs-io/dns-contracts`.
- This repository, `dns-cli`, is where the Go-native CLI will be built.

## Reference Flows in `dns-contracts`

The planned CLI work is based on these existing reference materials:

- Register TLD shell flow:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/02-register-tld.sh`
- Activate TLD shell flow:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/03-mint-tld.sh`
- Mint SLD shell flow:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/04-mint-sld.sh`
- TLD registrar validator:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_registrar.ak`
- TLD reference validator:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_reference.ak`
- SLD reference validator:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/sld_reference.ak`
- Core protocol types:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/lib/types.ak`
- Architecture docs:
  `https://github.com/blinklabs-io/dns-contracts/blob/main/docs/architecture/smart-contract-architecture.md`

## CLI Scope

The milestone CLI should support these commands or equivalent flows:

- `config init`
- `config show`
- `config validate`
- `registrar register-tld`
- `owner activate-tld`
- `owner mint-sld`
- `owner update-sld`

The CLI should use configuration for:

- network / RPC settings
- validator or artifact paths
- reference script UTxOs
- registrar wallet config
- owner wallet config
- static Handshake proof bundle

## Task Overview

This plan uses 4 tasks.

The tasks are written so a first-time contributor can understand:

- what each task is for
- what needs to be delivered
- which earlier tasks it depends on
- how success will be checked

## Task 1: CLI Foundation and Go-Native Connectivity

Reward: `4,000 ADA`

Depends on:
- none

Use this task to complete:
- Task 2
- Task 3
- Task 4

Purpose:
- Create the initial Go CLI structure.
- Make configuration and actor setup usable.
- Provide the shared runtime needed by all later command flows.

Scope:
- Cobra-based CLI structure
- config file loading
- `config init`
- `config show`
- `config validate`
- proof bundle loading
- Bursa-based wallet and key loading
- Bursa-based signing support
- Dingo / UTxO RPC connectivity
- Apollo transaction setup and submission wiring
- common helpers for:
  - loading configured wallets
  - loading configured reference UTxOs
  - loading validator artifacts or script references
  - submitting transactions
  - waiting for transaction confirmation
- explicit runtime capability support for:
  - reference inputs
  - inline datums
  - minting with redeemers
  - multi-signer transactions
  - transaction evaluation or fee / execution-unit estimation where required by the stack

Not in scope for this task:
- protocol-specific datum builders unless they are needed for a minimal working example
- protocol-specific redeemer builders unless they are needed for a minimal working example
- full TLD or SLD command implementation

Acceptance criteria:
- A new user can initialize a config file from the CLI.
- A new user can validate the config file and get clear feedback about missing fields or files.
- The CLI can load a static Handshake proof bundle.
- The CLI can load configured wallets and sign through Bursa.
- The CLI can connect to the configured Dingo / UTxO RPC endpoint.
- The CLI can load configured reference UTxOs and validator or script references from config.
- The repository includes a documented smoke path proving the CLI can build, sign, submit, and confirm at least one transaction through the Go-native stack.
- The runtime capabilities required by later tasks are demonstrated or covered by automated checks:
  - reference inputs
  - inline datums
  - minting with redeemers
  - multi-signer transactions
- The CLI runtime can evaluate or otherwise correctly complete Plutus transactions using the chosen Go-native stack.
- The CLI does not require `cardano-cli`.

## Task 2: TLD Onboarding Flow

Reward: `6,000 ADA`

Depends on:
- Task 1

Use this task to complete:
- Task 3
- Task 4

Purpose:
- Implement the full TLD onboarding flow.
- This includes both the registrar-side TLD registration step and the owner-side TLD activation step.

Why these steps are grouped together:
- They are both part of bringing a TLD from proof bundle to usable on-chain TLD state.
- A first-time contributor can understand them better as one TLD onboarding track.

References:
- `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/02-register-tld.sh`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/03-mint-tld.sh`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_registrar.ak`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_reference.ak`

Deliverables:
- `registrar register-tld` command
- `owner activate-tld` command
- all TLD-specific datum and redeemer builders needed for those flows
- TLD-specific on-chain state lookup logic needed for those flows
- clear command output showing success and tx ids

Acceptance criteria:
- A registrar can run one CLI command to register a TLD on Cardano.
- The resulting on-chain state includes a valid `TLDRegisterDatum` with `minted = 0`.
- A TLD owner can run one CLI command to activate a previously registered TLD.
- The activation flow results in:
  - updated registrar datum with `minted = 1`
  - TLD user token in the owner wallet
  - TLD reference token locked with an initial empty `TLDReferenceDatum`
- Both commands work through the Go-native stack only.
- The commands clearly show which TLD was processed, which transaction was submitted, and which resulting UTxO or token state is needed by Task 3.

## Task 3: SLD Lifecycle Flow

Reward: `7,000 ADA`

Depends on:
- Task 1
- Task 2

Use this task to complete:
- Task 4

Purpose:
- Implement the SLD lifecycle needed by Milestone 4.
- This includes both creating an SLD and updating its DNS records.

Why these steps are grouped together:
- They both operate on the same SLD state model.
- The `update-sld` flow depends on the existence of an SLD reference state.
- It is easier for contributors to reason about them together as one SLD track.

References:
- `https://github.com/blinklabs-io/dns-contracts/blob/main/scripts/04-mint-sld.sh`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/tld_reference.ak`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/onchain/validators/tld_registration/sld_reference.ak`
- `https://github.com/blinklabs-io/dns-contracts/blob/main/docs/architecture/smart-contract-architecture.md`

Deliverables:
- `owner mint-sld` command
- `owner update-sld` command
- all SLD-specific datum and redeemer builders needed for those flows
- DNS record input format
- SLD-specific state lookup logic needed for those flows
- clear command output showing success and tx ids

Acceptance criteria:
- A TLD owner can mint an SLD under an activated TLD with one CLI command.
- The parent TLD `slds` list is updated correctly in sorted order.
- The SLD token pair is minted correctly.
- A valid `SLDReferenceDatum` is created for the new SLD.
- An SLD owner can update DNS records for an SLD with one CLI command.
- The `tld` and `sld` fields remain unchanged while the `records` field updates as expected.
- At least one supported DNS record input format is documented and works end to end.
- Both commands work through the Go-native stack only.
- The command outputs include the tx id and enough state information to feed Task 4 milestone evidence.

## Task 4: Integration, Demo Flow, and Documentation

Reward: `3,000 ADA`

Depends on:
- Task 1
- Task 2
- Task 3

Purpose:
- Prove the milestone works end to end.
- Make the CLI usable by a new operator.
- Prepare the material needed for milestone review.

Deliverables:
- reproducible end-to-end demo flow
- integration coverage or reproducible scripted demo of the main success path
- setup guide
- config guide
- command guide
- actor guide for registrar vs owner
- proof bundle format documentation
- milestone evidence guide

Acceptance criteria:
- There is a reproducible full milestone flow:
  - config
  - register TLD
  - activate TLD
  - mint SLD
  - update SLD
- The full flow uses the static Handshake proof bundle defined in config.
- The resulting outputs include tx ids and evidence of the expected state changes.
- The documentation includes an explicit milestone crosswalk:
  - milestone item "register TLD" maps to `registrar register-tld`
  - required enabling step `owner activate-tld` is explained as part of the real contract flow
  - milestone item "mint SLD" maps to `owner mint-sld`
  - milestone item "configure DNS" maps to `owner update-sld`
- A new operator can follow the docs to run the full flow from config to DNS update.
- The docs clearly explain the difference between `register-tld` and `activate-tld`.
- The docs link back to the relevant `dns-contracts` references for protocol context.

## Budget Summary

- Task 1: `4,000 ADA`
- Task 2: `6,000 ADA`
- Task 3: `7,000 ADA`
- Task 4: `3,000 ADA`

Total: `20,000 ADA`

## Why This Budget Split Makes Sense

- Task 1 is still substantial because it establishes the CLI, config flow, wallet/signing setup through Bursa, provider connectivity through Dingo / UTxO RPC, and transaction submission wiring through Apollo, but it is lower risk now that this stack is the confirmed implementation direction.
- Task 2 is substantial because it implements two separate actor flows for TLD onboarding and includes the TLD-specific protocol payloads and state handling. Both flows have concrete shell-script references in `dns-contracts`, but they are still separate end-user commands and the second flow is required before SLD work can begin.
- Task 3 is the heaviest protocol task because `mint-sld` coordinates parent and child on-chain state and mints a new SLD token pair, while also including the lighter `update-sld` flow needed to satisfy the milestone's DNS configuration requirement.
- Task 4 is smaller because it depends on the earlier protocol and CLI work being completed, but it is still important because it provides milestone evidence and operator usability.

## Final Notes

- The shell scripts in `dns-contracts` are references for behavior, not the final implementation approach.
- The final CLI in this repository should be fully Go-native.
- `activate-tld` is included because it is required by the real contract flow, even though the original milestone wording does not name it explicitly.
