---
name: dns-cli-milestone4
overview: Build a Go/Cobra `dns-cli` that reproduces the four required Cardano contract flows, supports selectable UTxO RPC and Blockfrost backends, and uses a complete offline build/sign/submit/status transaction lifecycle. Preserve the existing milestone summary and save this expanded implementation specification as `dns-cli/MILESTONE4_DETAILED_PLAN.md`.
todos:
  - id: foundation
    content: Initialize Go/Cobra, pin Apollo v2 commit, and prove API compatibility
    status: pending
  - id: config-provider
    content: Implement versioned JSON profiles, flag precedence, UTxO RPC/Blockfrost adapters, and confirmation
    status: pending
  - id: wallet-artifacts
    content: Implement secure actor loading and offline text-envelope inspect/sign/submit/status lifecycle
    status: pending
  - id: protocol
    content: Encode contract types, proof verification, token hashing, DNS records, and blueprint checks
    status: pending
  - id: state-query
    content: Implement registration/TLD/SLD state discovery, linked-list checks, ownership, and collateral selection
    status: pending
  - id: register-tld
    content: Build and test registrar TLD registration transaction
    status: pending
  - id: activate-tld
    content: Build and test separate owner TLD activation transaction
    status: pending
  - id: mint-sld
    content: Build and test atomic SLD mint and parent-state update transaction
    status: pending
  - id: update-sld
    content: Build and test generic DNS-record replacement transaction
    status: pending
  - id: verification
    content: Add fixed-chain, fuzz, security, CLI contract, Aiken, and manually tagged live tests
    status: pending
  - id: documentation
    content: Write installation, configuration, command, offline-signing, operator, protocol, security, and troubleshooting guides
    status: pending
  - id: release
    content: Add Windows/Linux CI, reproducible binaries, checksums, and milestone evidence workflow
    status: pending
isProject: false
---

# DNS CLI Milestone 4 Detailed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a production-oriented Go/Cobra CLI that registers a TLD, separately activates it, mints an SLD, and updates all contract-supported DNS records through fully offline-capable Cardano transactions.

**Architecture:** Domain commands validate and resolve chain state, then build unsigned Conway-era transactions with Apollo v2. A provider layer exposes either UTxO RPC or Blockfrost through Apollo’s `backend.ChainContext`; an artifact layer writes Cardano text envelopes plus manifests, while separate `tx sign`, `tx submit`, and `tx status` commands handle custody and broadcasting. Protocol types and transaction recipes mirror the Aiken source rather than shell-script implementation details.

**Tech Stack:** Go 1.25.x, Cobra, Apollo v2 pinned to commit `b2f56d0c6e9d22316b6938feeb325bdbab3846d2`, Bursa, plutigo/gouroboros types, UTxO RPC, Blockfrost, JSON, standard Go tests, Aiken contract tests, GitHub Actions.

**Plan destination:** Add [`dns-cli/MILESTONE4_DETAILED_PLAN.md`](dns-cli/MILESTONE4_DETAILED_PLAN.md); retain [`dns-cli/MILESTONE4_PLAN.md`](dns-cli/MILESTONE4_PLAN.md) as the milestone summary and link it to the detailed plan.

---

## Confirmed product decisions

- Executable name: `dns-cli`.
- TLD registration and activation remain separate commands because they are separate contract transactions.
- Networks: preview and preprod; network parameters remain config-driven.
- Providers: selectable `utxorpc` or `blockfrost`, with UTxO RPC the generated-config default; no silent fallback or cross-provider mixing.
- Credentials: Bursa-compatible key files or mnemonics supplied through environment variables; secret material is never stored in config, manifests, output, or logs.
- DNS scope: every shape representable by `DNSRecord { lhs, ttl, class, rtype, rdata }`.
- Handshake proof: static JSON file for this milestone.
- Output: human-readable by default and stable machine-readable output through `--output json`.
- Transaction mode: full offline lifecycle. Domain commands build unsigned text envelopes; `tx sign` merges witnesses; `tx submit` broadcasts; `tx status` waits for or inspects confirmation.
- Tests: deterministic unit/fixture tests in CI; credentialed preview/preprod integration tests are manually invoked with a build tag.
- Distribution: source plus reproducible Windows and Linux binaries.
- Apollo: use the v2 API at the pinned latest local commit, resolved as a Go pseudo-version with `go get github.com/Salvionied/apollo/v2@b2f56d0c6e9d22316b6938feeb325bdbab3846d2`. Do not use the old v1 imports from `apollo-testing-suite`.

## Source-of-truth crosswalk

Implementation must trace these repository sources and record their commit hashes in generated build metadata:

- Registration shell reference: [`dns-contracts/scripts/02-register-tld.sh`](dns-contracts/scripts/02-register-tld.sh).
- Activation shell reference: [`dns-contracts/scripts/03-mint-tld.sh`](dns-contracts/scripts/03-mint-tld.sh).
- SLD mint shell reference: [`dns-contracts/scripts/04-mint-sld.sh`](dns-contracts/scripts/04-mint-sld.sh).
- Protocol types: [`dns-contracts/onchain/lib/types.ak`](dns-contracts/onchain/lib/types.ak).
- Token hashing and signature rules: [`dns-contracts/onchain/lib/utils.ak`](dns-contracts/onchain/lib/utils.ak).
- Registrar validator: [`dns-contracts/onchain/validators/tld_registration/tld_registrar.ak`](dns-contracts/onchain/validators/tld_registration/tld_registrar.ak).
- TLD reference validator: [`dns-contracts/onchain/validators/tld_registration/tld_reference.ak`](dns-contracts/onchain/validators/tld_registration/tld_reference.ak).
- SLD reference validator: [`dns-contracts/onchain/validators/tld_registration/sld_reference.ak`](dns-contracts/onchain/validators/tld_registration/sld_reference.ak).
- Blueprint: [`dns-contracts/onchain/plutus.json`](dns-contracts/onchain/plutus.json), built with Plutus V3/Aiken 1.1.19.
- Validator tests: [`dns-contracts/onchain/validators/tld_registration/tests`](dns-contracts/onchain/validators/tld_registration/tests).
- Current Apollo interfaces: [`apollo/backend/base.go`](apollo/backend/base.go), [`apollo/apollo.go`](apollo/apollo.go), [`apollo/wallet.go`](apollo/wallet.go), [`apollo/backend/utxorpc/utxorpc.go`](apollo/backend/utxorpc/utxorpc.go), and [`apollo/backend/blockfrost/blockfrost.go`](apollo/backend/blockfrost/blockfrost.go).
- Older examples are behavioral references only: [`apollo-testing-suite/utxo-rpc-based-tx-builder/main.go`](apollo-testing-suite/utxo-rpc-based-tx-builder/main.go) and [`apollo-testing-suite/plutus-v3-sc-tx-builder/main.go`](apollo-testing-suite/plutus-v3-sc-tx-builder/main.go).

## Target command surface

```text
dns-cli version

dns-cli config init [--network preview|preprod] [--provider utxorpc|blockfrost]
dns-cli config show [--redact]
dns-cli config validate [--online]

dns-cli registrar register-tld --tld NAME --proof FILE --out FILE

dns-cli owner activate-tld --tld NAME --proof FILE --out FILE
dns-cli owner mint-sld --tld NAME --sld LABEL --sld-owner ACTOR --out FILE
dns-cli owner update-sld --tld NAME --sld LABEL --records FILE --out FILE

dns-cli tx inspect --tx FILE
dns-cli tx sign --tx FILE --actor NAME --out FILE
dns-cli tx submit --tx FILE
dns-cli tx status --tx-id HASH [--manifest FILE] [--wait]
```

Global/persistent flags: `--config`, `--network`, `--provider`, `--output human|json`, `--timeout`, `--log-level`, `--artifact-dir`, and `--no-color`. Precedence is command flag, secret environment variable, selected JSON profile, then non-secret built-in default. Empty flags do not erase valid config values.

## Proposed repository map

Create focused files with one responsibility; avoid a single oversized command or transaction file.

```text
dns-cli/
  cmd/dns-cli/main.go
  internal/cli/root.go
  internal/cli/config_commands.go
  internal/cli/registrar_commands.go
  internal/cli/owner_commands.go
  internal/cli/tx_commands.go
  internal/cli/output.go
  internal/config/model.go
  internal/config/load.go
  internal/config/validate.go
  internal/config/defaults.go
  internal/domain/name.go
  internal/domain/record.go
  internal/domain/proof.go
  internal/protocol/types.go
  internal/protocol/plutus.go
  internal/protocol/token.go
  internal/protocol/blueprint.go
  internal/provider/provider.go
  internal/provider/factory.go
  internal/provider/utxorpc.go
  internal/provider/blockfrost.go
  internal/provider/wait.go
  internal/wallet/source.go
  internal/wallet/keyfile.go
  internal/wallet/mnemonic.go
  internal/wallet/sign.go
  internal/chainquery/registration.go
  internal/chainquery/tld.go
  internal/chainquery/sld.go
  internal/chainquery/assets.go
  internal/txbuilder/common.go
  internal/txbuilder/register_tld.go
  internal/txbuilder/activate_tld.go
  internal/txbuilder/mint_sld.go
  internal/txbuilder/update_sld.go
  internal/artifact/envelope.go
  internal/artifact/manifest.go
  internal/artifact/sign.go
  internal/artifact/submit.go
  internal/testutil/fixed_chain.go
  internal/testutil/fixtures.go
  testdata/config/*.json
  testdata/proofs/*.json
  testdata/records/*.json
  testdata/plutus/*.json
  docs/installation.md
  docs/configuration.md
  docs/commands.md
  docs/offline-transactions.md
  docs/operator-guide.md
  docs/protocol-crosswalk.md
  docs/security.md
  docs/troubleshooting.md
  examples/config.preview.json
  examples/config.preprod.json
  examples/proof-bundle.json
  examples/records.json
  scripts/demo-preview.sh
  scripts/demo-preview.ps1
  .github/workflows/ci.yml
  .github/workflows/release.yml
  go.mod
  go.sum
  Makefile
  MILESTONE4_DETAILED_PLAN.md
```

Test files live beside each package as `*_test.go`; transaction builders also receive fixed-context integration tests.

## Configuration contract

The generated JSON must be commented in documentation, not with invalid JSON comments. A representative shape is:

```json
{
  "version": 1,
  "defaultProfile": "preview",
  "profiles": {
    "preview": {
      "network": {"name": "preview", "id": 0, "magic": 2, "explorerTxURL": "https://preview.cexplorer.io/tx/{txId}"},
      "provider": {"type": "utxorpc", "baseURL": "https://example.invalid", "headersEnv": "DNS_CLI_UTXORPC_HEADERS"},
      "contracts": {
        "blueprintPath": "../dns-contracts/onchain/plutus.json",
        "tldRegistrarAddress": "addr_test...",
        "tldReferenceAddress": "addr_test...",
        "sldReferenceAddress": "addr_test...",
        "tldRegistrarPolicyId": "...",
        "tldReferencePolicyId": "...",
        "sldReferencePolicyId": "...",
        "referenceUtxos": {
          "tldRegistrar": "txhash#0",
          "tldReference": "txhash#1",
          "sldReference": "txhash#2"
        }
      },
      "actors": {
        "registrar": {"address": "addr_test...", "signingKeyFile": "keys/registrar.skey"},
        "tldOwner": {"address": "addr_test...", "mnemonicEnv": "DNS_CLI_TLD_OWNER_MNEMONIC"},
        "sldOwner": {"address": "addr_test...", "signingKeyFile": "keys/sld-owner.skey"}
      },
      "transaction": {"ttlSlots": 300, "confirmationTimeout": "10m", "pollInterval": "5s", "artifactDir": "artifacts"}
    }
  }
}
```

For Blockfrost, use `provider.type = "blockfrost"`, a profile-specific base URL, and `projectIdEnv`; reject literal API keys in config. `config show` always redacts environment-derived values and key paths may be shown but never file contents. `config validate --online` additionally resolves reference UTxOs, confirms their Plutus V3 script hashes, checks provider network identity, and checks actor address network IDs.

## Offline artifact contract

Each build writes:

1. `<name>.unsigned.json`: Cardano text envelope with `type`, `description`, and `cborHex`.
2. `<name>.manifest.json`: operation, network, provider, transaction body hash, required signer key hashes, expected output roles/indexes/assets/datums, source contract commit, Apollo commit, and a SHA-256 digest of non-secret effective config.

`tx sign` must decode Conway CBOR, recompute and compare the body hash with the manifest, derive the selected actor key hash, reject an unexpected signer unless `--allow-extra-signer` is explicitly supplied, add one VKey witness, deduplicate identical witnesses, preserve existing witnesses, and write a new text envelope atomically. It must never rebuild the transaction.

`tx submit` must reject an unsigned/incompletely witnessed transaction, wrong-network manifest, changed body hash, or already expired validity interval. `tx status` uses manifest output indexes and `ChainContext.UtxoByRef` to prove inclusion; this works for both selected providers without provider-specific hidden fallback.

## Phase 0: Baseline, dependency proof, and protocol lock

- [ ] Record clean repository states and source commits for `dns-cli`, `dns-contracts`, and `apollo`.
- [ ] Save this plan as `MILESTONE4_DETAILED_PLAN.md` and link it from the summary plan and README.
- [ ] Initialize module `github.com/blinklabs-io/dns-cli` and add Cobra plus Apollo v2 using the exact commit command above; commit the resolved pseudo-version in `go.mod`/`go.sum`.
- [ ] Add a compile-only compatibility test proving the selected Apollo version exposes `backend.ChainContext`, `apollo.New`, `CollectFrom`, `Mint`, `AddReferenceInput`, `PayToContract`, `Complete`, `GetTxCbor`, and the wallet signing interface.
- [ ] Add an architecture decision record explaining why old Apollo v1 imports and `cardano-cli` are prohibited.
- [ ] Run `go test ./...`; expected result is PASS before protocol work begins.

## Phase 1: Cobra foundation and deterministic output

- [ ] Write command-construction tests that execute Cobra with in-memory stdin/stdout/stderr and no global mutable state.
- [ ] Implement `main.go` as wiring only; return errors from `ExecuteContext`, print once, and map typed errors to documented exit codes.
- [ ] Add root persistent flags and command groups `config`, `registrar`, `owner`, and `tx`.
- [ ] Define stable JSON result envelopes containing `ok`, `command`, `network`, `operation`, `artifact`, `txId`, `explorerUrl`, `warnings`, and structured error details where applicable.
- [ ] Keep logs on stderr and command results on stdout so scripts can safely parse JSON.
- [ ] Add `version` output with CLI version, Git commit, build date, Go version, Apollo dependency revision, and contract-source revision.
- [ ] Test help text, unknown commands, invalid output modes, exit codes, and JSON cleanliness.

## Phase 2: Versioned config, profiles, flags, and validation

- [ ] Write table-driven tests for missing files, malformed JSON, unknown schema version, missing profile, preview/preprod network mismatches, invalid provider type, duplicate actors, bad addresses, malformed policy IDs, malformed `txhash#index`, and forbidden inline secrets.
- [ ] Implement strict JSON decoding with unknown-field rejection and path-aware errors such as `profiles.preview.contracts.referenceUtxos.tldRegistrar`.
- [ ] Implement preview and preprod defaults without embedding deployment-specific policy IDs, addresses, UTxOs, keys, API keys, or mnemonics.
- [ ] Implement deterministic precedence and expose the effective non-secret values in `config show`.
- [ ] Implement `config init` with refusal to overwrite unless `--force`; use owner-only file permissions where supported and warn on Windows ACL limitations.
- [ ] Implement offline validation first, then `--online` provider/reference-script validation.
- [ ] Add golden tests for generated preview/preprod JSON and human/JSON validation output.

## Phase 3: Provider abstraction and confirmation

Define an application interface that embeds Apollo’s chain context and adds context-aware health/confirmation behavior:

```go
type Provider interface {
    backend.ChainContext
    Name() string
    Health(ctx context.Context) error
    AwaitOutputs(ctx context.Context, txID common.Blake2b256, indexes []uint32) error
}
```

- [ ] Write factory tests for both provider types, required secret environment variables, custom UTxO RPC headers, timeout handling, network IDs, and unsupported providers.
- [ ] Wrap `utxorpc.NewUtxoRpcChainContext(baseURL, networkID, headers)`.
- [ ] Wrap `blockfrost.NewBlockFrostChainContext(baseURL, networkID, projectID)`.
- [ ] Implement bounded exponential/poll-interval confirmation via `UtxoByRef`; distinguish pending, confirmed, rejected/submission error, timeout, and provider-unavailable states.
- [ ] Add fake-chain tests for delayed inclusion, cancellation, timeout, wrong output index, malformed provider responses, and transient query errors.
- [ ] Document that UTxO RPC cannot evaluate chained/off-chain additional UTxOs in the current Apollo backend; each milestone transaction therefore evaluates against already confirmed inputs.

## Phase 4: Wallet custody and witness handling

- [ ] Define mutually exclusive actor sources: `signingKeyFile` or `mnemonicEnv`; fail if neither or both are set.
- [ ] Add fixtures for supported Bursa/Cardano signing-key text envelopes and mnemonic-derived wallets.
- [ ] Parse key files without logging raw JSON/CBOR; validate key length and type before calling cryptographic libraries.
- [ ] For mnemonics, load only the named environment variable and construct the Bursa wallet for the profile network/account/address index.
- [ ] Verify the derived payment key hash controls the configured actor address before signing.
- [ ] Implement direct witness creation/merge against decoded transaction CBOR so `tx sign` can operate on an existing envelope without rerunning Apollo coin selection or evaluation.
- [ ] Add security tests proving mnemonic, private key, API key, headers, and signatures are redacted from errors, `%v`, `%+v`, JSON output, and manifests.
- [ ] Add multi-sign tests: first signer, second signer, duplicate signer, wrong signer, tampered body, malformed witness set, and atomic output replacement.

## Phase 5: Domain and input validation

- [ ] Normalize user-entered DNS labels to canonical lowercase ASCII; use IDNA conversion for Unicode input and display both original and canonical values when they differ.
- [ ] Enforce DNS label length, allowed characters, no leading/trailing hyphen, and no dots in individual TLD/SLD arguments; reject ambiguous fully-qualified input where separate labels are required.
- [ ] Preserve canonical bytes exactly because contract token names and signatures are byte-sensitive.
- [ ] Validate proof bundle hex lengths and secp256k1 compressed public-key/signature formats before building.
- [ ] Verify static signatures off-chain using the contract rule `ECDSA-secp256k1(publicKey, blake2b_256(tldBytes), signature)` so invalid proofs fail before fees are spent.
- [ ] Model every DNS record field as contract bytes with explicit `text` or `hex` encoding and model TTL as omitted/null or DNS-compatible unsigned value.
- [ ] Validate record count, duplicate canonical records, TTL range, class/type non-empty values, byte lengths, and a configurable maximum encoded datum size before Apollo evaluation.
- [ ] Add fuzz tests for names, hex, proof JSON, record JSON, and canonical sorting.

## Phase 6: Plutus data and blueprint compatibility

Use typed Go structs with `plutus` tags and Apollo v2’s `plutusencoder`; do not hand-assemble untyped nested arrays throughout builders.

- [ ] Implement and document `TLDRegisterDatum`, `RegistrarRedeemer` variants, `DNSRecord`, `TLDReferenceDatum`, `TLDReferenceAction`, `SLDReferenceDatum`, and `MintSld` with constructor indexes matching `types.ak`.
- [ ] Implement the `Option<Int>` encoding for TTL exactly as the blueprint specifies, including `None` versus `Some(0)`.
- [ ] Implement token names as `blake2b_256("r" + canonicalName)` and `blake2b_256("u" + canonicalName)`.
- [ ] Parse `plutus.json`, select validators by full blueprint title, verify Plutus V3, apply parameters in declared order, and compare resulting hashes with configured policy IDs/addresses.
- [ ] Add golden CBOR tests against every existing datum/redeemer JSON fixture under `dns-contracts/preprod`.
- [ ] Add blueprint-schema tests so a constructor, parameter, compiler, or field-order change causes an explicit compatibility failure rather than a malformed transaction.
- [ ] Run `aiken check` in `dns-contracts/onchain`; expected result is all validator tests passing.

## Phase 7: Chain-state discovery and ownership checks

- [ ] Implement deterministic asset lookup by policy ID/token name, not “largest ADA UTxO” heuristics from the shell scripts.
- [ ] Registration lookup: find exactly one registrar UTxO containing the registration token whose asset name is the TLD reference policy ID; decode and compare `TLDRegisterDatum.tld`.
- [ ] TLD lookup: traverse TLD reference UTxOs by the reference token and `next` pointers; detect missing nodes, duplicate nodes, cycles, unsorted SLDs, duplicate SLDs, and policy mismatches.
- [ ] SLD lookup: find exactly one SLD reference token UTxO and verify datum `tld` and `sld` match the requested canonical labels.
- [ ] Ownership checks: locate TLD or SLD user-token UTxOs under the configured actor address and report the exact missing asset without exposing unrelated wallet contents.
- [ ] Select a separate key-locked ADA-only collateral UTxO where possible; reject script-only collateral and document Apollo’s single-UTxO overlap behavior.
- [ ] Add fake-provider tests for every success, absent, duplicate, corrupted datum, wrong owner, wrong network, and linked-list corruption case.

## Phase 8: Shared Apollo transaction pipeline

- [ ] Implement a builder context carrying profile, provider, wallet address, resolved contract state, validity interval, required signers, operation metadata, and expected output roles.
- [ ] Centralize Apollo setup: `apollo.New(provider)`, external/watch wallet for unsigned construction, loaded UTxOs, change address, collateral, validity start/TTL, reference inputs, required signer hashes, and automatic ExUnit estimation.
- [ ] Ensure every reference-script UTxO resolves and contains the expected Plutus V3 script before `Complete()`; fail if protocol parameters omit reference-script pricing.
- [ ] Ensure minimum ADA is calculated by Apollo instead of retaining hardcoded shell values such as `1525740`.
- [ ] Ensure deterministic output ordering so manifests can name output indexes reliably.
- [ ] After `Complete()`, decode the CBOR and independently assert the operation’s required inputs, outputs, mint, redeemers, datums, signers, network, and validity interval before writing artifacts.
- [ ] Add fixed-chain tests for reference inputs, inline datums, mint and spend redeemers, execution-unit evaluation, collateral, fee convergence, and multiple required signers.

## Phase 9: `registrar register-tld`

Build exactly the registrar mint path from `tld_registrar.ak`:

- [ ] Test rejection of invalid label/proof, owner-key mismatch, duplicate registration, wrong registrar actor, missing fee/collateral UTxOs, mismatched policy IDs, and invalid reference script.
- [ ] Create `RegisterTLD(tld, ownerKey, registrarSignature, tldReferencePolicyId)`.
- [ ] Mint exactly one asset under the registrar policy, with asset name equal to the TLD reference policy ID.
- [ ] Include the configured TLD registrar reference-script UTxO and mint redeemer.
- [ ] Pay the registration NFT to the registrar validator with inline `TLDRegisterDatum(tld, ownerKey, 0)`.
- [ ] Require the registrar Cardano payment signer for funding; do not confuse this witness with the Handshake registrar signature inside the redeemer.
- [ ] Emit unsigned envelope/manifest details: canonical TLD, owner HNS key fingerprint, registration asset, registrar output index, fee, validity interval, required signer, and explorer URL template without claiming submission.
- [ ] Add golden transaction-structure tests and command tests in human/JSON modes.

## Phase 10: `owner activate-tld`

Build the distinct first-owner-action transaction:

- [ ] Resolve the registration UTxO and require `minted == 0`; verify proof owner key equals datum owner key.
- [ ] Spend it using `OwnerAction(ownerSignature)` and the registrar reference script.
- [ ] Recreate the registration output with unchanged TLD/owner and `minted == 1`.
- [ ] Mint exactly one TLD reference token and one TLD user token under the TLD reference policy using `InitRemoveReference` and its reference script.
- [ ] Lock the reference token at the TLD reference validator with `TLDReferenceDatum(tld, [], sldPolicyId, emptyNext, [])`.
- [ ] Send the user token explicitly to the configured TLD owner address.
- [ ] Require all Cardano witnesses needed for funded/key-controlled inputs and list them in the manifest.
- [ ] Test already activated, wrong owner signature, changed registrar datum, wrong token names, missing reference input, incorrect initial datum, and output ownership.

## Phase 11: `owner mint-sld`

Build the atomic parent update and child mint transaction:

- [ ] Resolve the correct TLD linked-list node, reject an existing SLD, and require the TLD user token at the configured TLD-owner address.
- [ ] Spend the selected TLD reference UTxO with `SpendReference` and include the TLD reference script UTxO.
- [ ] Insert canonical SLD bytes into the node’s SLD list, preserving strict lexicographic sort/uniqueness and unchanged TLD, SLD policy ID, `next`, and TLD records.
- [ ] Mint SLD reference/user token names under the SLD policy using `MintSld(tld, [sld], [])` and include the SLD reference script UTxO.
- [ ] Create `SLDReferenceDatum(tld, sld, [])` and lock the SLD reference token at the SLD reference validator.
- [ ] Send the SLD user token explicitly to `--sld-owner`; return the TLD user token to its owner through deterministic output/change handling.
- [ ] Require and merge both fee-payer/TLD-owner witnesses when distinct; represent duplicate roles by one signer hash.
- [ ] Test parent/child redeemer coordination, sorted insertion at beginning/middle/end, duplicate SLD, wrong linked-list node, missing owner token, wrong recipient, multi-signer artifacts, and atomic state invariants.
- [ ] Treat TLD-reference split/merge as a separately documented future command unless datum-size validation proves a split is required; in that case stop with an actionable capacity error rather than building an invalid oversized datum.

## Phase 12: `owner update-sld`

Build the SLD spend path used for DNS configuration:

- [ ] Resolve the SLD reference UTxO and require the SLD user token at the configured SLD-owner address.
- [ ] Parse and canonicalize the complete records file before any provider calls that incur cost/rate limits.
- [ ] Spend the SLD reference with the generic empty-byte Data redeemer used by the validator tests and include the SLD reference script UTxO.
- [ ] Recreate exactly one SLD reference output with unchanged `tld` and `sld`, updated `records`, the same reference token, and no attached reference script.
- [ ] Preserve and return the SLD user token to its owner.
- [ ] Support explicit record replacement as the first release behavior; document that an empty list clears all records. Do not silently merge records.
- [ ] Emit old/new record counts, datum hashes/CBOR fingerprints, expected output index, signer, fee, and artifact paths.
- [ ] Test all contract-supported byte combinations, TTL None/Some, clear-all, wrong owner, changed identity fields, duplicate reference output, oversized datum, and no-op replacement warning.

## Phase 13: Transaction inspection, signing, submission, and status

- [ ] `tx inspect`: decode envelope and manifest, show operation, body hash, network, inputs, outputs, mint, redeemers, required/present signers, fee, validity, and datum summaries without private data.
- [ ] `tx sign`: implement the witness checks and merge rules specified above; allow repeated invocations for SLD multi-signing.
- [ ] `tx submit`: verify completeness and provider/network consistency, submit exact CBOR through the selected `ChainContext.SubmitTx`, compare returned hash with locally computed transaction ID, and never mutate the signed artifact.
- [ ] `tx status`: support one-shot and `--wait`; use expected output indexes from manifest and return pending/confirmed/timeout with stable exit codes.
- [ ] Add `--output json` contract tests suitable for shell/PowerShell automation.
- [ ] Add negative tests for malformed envelope, wrong era/type, absent manifest, mismatched body hash, incomplete witnesses, expired transaction, wrong profile, provider rejection, duplicate submission, timeout, and Ctrl+C cancellation.

## Phase 14: End-to-end tests and contract parity

- [ ] Build a deterministic fixed-chain fixture containing registrar/reference scripts, actor UTxOs, collateral, and protocol parameters.
- [ ] Execute the complete offline sequence in tests: register build/sign, simulated submit/confirm, activate build/sign, SLD build/two-sign, DNS update build/sign.
- [ ] After each simulated confirmation, assert exact datum and token state matching contract acceptance criteria.
- [ ] Compare generated Plutus data and transaction shapes against script fixtures and Aiken invariants; do not compare unstable fees byte-for-byte.
- [ ] Add manually tagged tests `//go:build integration` for preview and preprod. They require explicit config, funded disposable actors, proof bundle, and `DNS_CLI_RUN_LIVE=1`.
- [ ] Make live tests use unique labels, print artifact paths/tx IDs, wait for confirmation, and never auto-deregister or destroy shared state.
- [ ] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go test -coverprofile=coverage.out ./...`, and `aiken check`; record commands and expected success in contributor docs.

## Phase 15: Operator and developer documentation

- [ ] Expand [`dns-cli/README.md`](dns-cli/README.md) with purpose, status, quick start, four-step protocol flow, offline lifecycle, and links to all guides.
- [ ] `installation.md`: Go prerequisites, Windows/Linux builds, binary verification, PATH, and version checks.
- [ ] `configuration.md`: every JSON field, preview/preprod examples, provider selection, flag precedence, environment secret names, and online validation.
- [ ] `commands.md`: complete Cobra-generated command reference plus realistic human/JSON examples.
- [ ] `offline-transactions.md`: ceremony for build, inspect, transport, one or multiple signers, submit, status, artifact integrity, expiration, and air-gapped machines.
- [ ] `operator-guide.md`: registrar versus TLD owner versus SLD owner responsibilities, funding/collateral preparation, full register→activate→mint→update runbook, and evidence collection.
- [ ] `protocol-crosswalk.md`: each command’s Aiken validator path, inputs, outputs, mint, datum, redeemer, owner token, reference script, and shell-script counterpart.
- [ ] `security.md`: mnemonic/API-key handling, file permissions, log redaction, artifact contents, offline signing, backup, and incident guidance.
- [ ] `troubleshooting.md`: invalid proof, missing owner token, duplicate state, provider mismatch, ExUnit failure, collateral failure, reference-script mismatch, expiration, and confirmation timeout.
- [ ] Add JSON examples and equivalent Bash and PowerShell end-to-end demo scripts that stop on errors and never embed secrets.
- [ ] Comment exported Go APIs and non-obvious protocol invariants; avoid comments that merely restate code. Include package docs for protocol, artifacts, provider, wallet, and each transaction builder.

## Phase 16: CI, release, and milestone evidence

- [ ] CI on Windows and Linux: format check, dependency verification, vet, unit/fixed-chain tests, race tests where supported, coverage artifact, and binary build.
- [ ] Add a check that rejects old `github.com/Salvionied/apollo` v1 imports, inline secret fixtures outside `testdata`, and accidental `cardano-cli` runtime dependencies.
- [ ] Release workflow builds reproducible `dns-cli` Windows amd64 and Linux amd64/arm64 binaries with version metadata and SHA-256 checksums.
- [ ] Generate shell completions and command docs from Cobra in a deterministic verification test.
- [ ] Create a milestone evidence checklist containing config-validation output, unsigned/signed artifact hashes, transaction IDs, explorer links, expected output references, and decoded before/after datums for all four transactions.
- [ ] Perform a fresh-machine documentation test on Windows and Linux using only released binaries and documented inputs.
- [ ] Update the summary plan status without deleting its budget/context sections.

## Acceptance gates

1. `dns-cli config init/show/validate` works for preview/preprod and both providers without exposing secrets.
2. Registration produces an unsigned transaction containing valid `RegisterTLD` data and `TLDRegisterDatum.minted = 0`.
3. Activation is a separate transaction that changes `minted` to 1, creates the TLD token pair, locks the reference token, and sends the user token to the owner.
4. SLD mint atomically updates the sorted parent list and creates the child token pair/datum, with all required Cardano witnesses represented in the manifest.
5. DNS update preserves TLD/SLD identity and replaces the full generic `DNSRecord` list.
6. Every domain command performs local validation, ownership/state checks, Apollo evaluation, independent post-build assertions, and writes an unsigned text envelope without broadcasting.
7. Offline signing merges and validates multiple witnesses; submission broadcasts exact signed CBOR; status confirms expected outputs and prints transaction details.
8. UTxO RPC and Blockfrost both satisfy the provider contract and pass deterministic adapter tests; preview/preprod live tests are manually reproducible.
9. No runtime path invokes `cardano-cli`; old Apollo v1 testing-suite APIs do not enter production code.
10. Unit, fuzz, fixed-chain, security, CLI contract, Aiken, Windows build, and Linux build checks pass.
11. A new operator can follow documentation from installation through confirmed DNS update without reading source code.
12. A new developer can trace every datum, redeemer, token, reference input, and ownership rule from Go code back to the exact Aiken source.

## Recommended implementation sequence and commit boundaries

Each numbered phase should be implemented test-first and split into focused commits: failing test/fixture, minimal implementation, passing verification, docs. Suggested commit subjects are `build: initialize cobra cli and apollo v2`, `feat: add versioned profile configuration`, `feat: add selectable chain providers`, `feat: add offline transaction artifacts`, `feat: encode dns contract plutus data`, `feat: build tld registration transaction`, `feat: build tld activation transaction`, `feat: build sld mint transaction`, `feat: build sld dns update transaction`, `test: add milestone fixed-chain flow`, `docs: add dns cli operator guides`, and `ci: build release binaries`. Commits are recommendations for implementation only; do not create them unless explicitly requested.