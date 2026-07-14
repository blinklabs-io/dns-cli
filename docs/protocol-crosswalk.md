# Protocol crosswalk

Maps dns-cli commands to Aiken validators in `dns-contracts/onchain`.

## register-tld

| Item | Value |
|---|---|
| Validator | `tld_registrar` mint + output to registrar spend |
| Shell reference | `scripts/02-register-tld.sh` |
| Mint redeemer | `RegisterTLD` (constructor 0) |
| Output datum | `TLDRegisterDatum { minted: 0 }` |
| Registration token | `1 registrarPolicy.tldReferencePolicyId` |
| Reference input | `referenceUtxos.tldRegistrar` |
| Cardano signer | Registrar payment key |

## activate-tld

| Item | Value |
|---|---|
| Validators | `tld_registrar` spend + `tld_reference` mint |
| Shell reference | `scripts/03-mint-tld.sh` |
| Spend redeemer | `OwnerAction` (constructor 2) |
| Mint redeemer | `InitRemoveReference` (constructor 0) |
| Registration datum | `minted: 1` |
| TLD tokens | user + reference under `tldReferencePolicyId` |
| Reference inputs | `tldRegistrar`, `tldReference` |

## mint-sld

| Item | Value |
|---|---|
| Validators | `tld_reference` spend + `sld_reference` mint |
| Shell reference | `scripts/04-mint-sld.sh` |
| TLD spend redeemer | `SpendReference` (constructor 2) |
| SLD mint redeemer | `MintSld` (constructor 0) |
| Parent update | sorted `slds` list in `TLDReferenceDatum` |
| Child datum | `SLDReferenceDatum { records: [] }` |
| Reference inputs | `tldReference`, `sldReference` |

## update-sld

| Item | Value |
|---|---|
| Validator | `sld_reference` spend |
| Spend redeemer | empty `Data` bytes |
| Records | complete replacement via `--records` file |
| Identity | `tld` and `sld` fields unchanged |

## Token names

- Reference: `blake2b_256("r" + canonicalName)`
- User: `blake2b_256("u" + canonicalName)`

## Proof verification

Off-chain: `ECDSA-secp256k1(ownerKey, blake2b_256(tldBytes), signature)`
