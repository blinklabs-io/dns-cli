#!/usr/bin/env bash
# Resumable Preprod demo runner for dns-cli (Bash).
# Usage: ./run-demo.sh [--mode fresh|existing] [--provider blockfrost|utxorpc] [--tld NAME] [--sld NAME]
set -euo pipefail

DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${DEMO_ROOT}/runtime"
STATE_FILE="${RUNTIME_DIR}/state.json"
STATE_SCHEMA="${DEMO_ROOT}/state.schema.json"
RECORDS_FILE="${DEMO_ROOT}/records.json"
FAUCET_URL="https://docs.cardano.org/cardano-testnets/tools/faucet/"
MIN_BOOTSTRAP_LOVELACE=150000000
POLL_SECONDS=20

MODE=""
PROVIDER=""
TLD=""
SLD=""
TLD_SET=0
SLD_SET=0
MODE_SET=0
PROVIDER_SET=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

  --mode fresh|existing     Demo mode (default: fresh, or DEMO_MODE)
  --provider blockfrost|utxorpc
                            Provider (default: blockfrost, or DEMO_PROVIDER)
  --tld NAME                TLD label (default: demo-<timestamp>)
  --sld NAME                SLD label (default: www)
  -h, --help                Show this help

Environment:
  CLI                         Path to dns-cli binary (default: ../dns-cli[.exe])
  DEMO_PROVIDER / DEMO_MODE   Defaults when flags omitted
  DNS_CLI_BLOCKFROST_PROJECT_ID   Required for blockfrost
  DNS_CLI_UTXORPC_URL             Required for utxorpc
  DNS_CLI_UTXORPC_HEADERS         Optional for utxorpc
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:-}"; MODE_SET=1; shift 2 ;;
    --provider) PROVIDER="${2:-}"; PROVIDER_SET=1; shift 2 ;;
    --tld) TLD="${2:-}"; TLD_SET=1; shift 2 ;;
    --sld) SLD="${2:-}"; SLD_SET=1; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

log() { printf '[demo] %s\n' "$*"; }
die() { printf '[demo] ERROR: %s\n' "$*" >&2; exit 1; }

resolve_cli() {
  if [[ -n "${CLI:-}" ]]; then
    DNS_CLI="${CLI}"
  elif [[ -x "${DEMO_ROOT}/../dns-cli" ]]; then
    DNS_CLI="${DEMO_ROOT}/../dns-cli"
  elif [[ -x "${DEMO_ROOT}/../dns-cli.exe" ]]; then
    DNS_CLI="${DEMO_ROOT}/../dns-cli.exe"
  elif command -v dns-cli >/dev/null 2>&1; then
    DNS_CLI="$(command -v dns-cli)"
  else
    die "dns-cli binary not found. Set CLI=... or build ../dns-cli"
  fi
  [[ -x "${DNS_CLI}" || -f "${DNS_CLI}" ]] || die "CLI not executable: ${DNS_CLI}"
  log "Using CLI: ${DNS_CLI}"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

check_provider_env() {
  case "${PROVIDER}" in
    blockfrost)
      [[ -n "${DNS_CLI_BLOCKFROST_PROJECT_ID:-}" ]] \
        || die "DNS_CLI_BLOCKFROST_PROJECT_ID is required for --provider blockfrost"
      ;;
    utxorpc)
      [[ -n "${DNS_CLI_UTXORPC_URL:-}" ]] \
        || die "DNS_CLI_UTXORPC_URL is required for --provider utxorpc"
      ;;
    *) die "unsupported provider: ${PROVIDER}" ;;
  esac
}

empty_step() {
  jq -n '{txId:"",manifest:""}'
}

default_state_json() {
  local tld="$1" sld="$2" mode="$3" provider="$4"
  jq -n \
    --argjson schemaVersion 1 \
    --arg mode "$mode" \
    --arg network "preprod" \
    --arg provider "$provider" \
    --arg tld "$tld" \
    --arg sld "$sld" \
    --argjson fund "$(empty_step)" \
    --argjson deploy "$(empty_step)" \
    --argjson register "$(empty_step)" \
    --argjson activate "$(empty_step)" \
    --argjson mintSld "$(empty_step)" \
    --argjson updateSld "$(empty_step)" \
    '{
      schemaVersion: $schemaVersion,
      mode: $mode,
      network: $network,
      provider: $provider,
      tld: $tld,
      sld: $sld,
      confirmed: {
        fund: $fund,
        deploy: $deploy,
        register: $register,
        activate: $activate,
        mintSld: $mintSld,
        updateSld: $updateSld
      }
    }'
}

save_state_atomic() {
  local tmp="${STATE_FILE}.tmp"
  printf '%s\n' "$1" >"${tmp}"
  mv -f "${tmp}" "${STATE_FILE}"
}

load_or_create_state() {
  mkdir -p "${RUNTIME_DIR}"
  local default_tld="demo-$(date +%Y%m%d%H%M%S)"

  if [[ "${MODE_SET}" -eq 0 ]]; then
    MODE="${DEMO_MODE:-fresh}"
  fi
  if [[ "${PROVIDER_SET}" -eq 0 ]]; then
    PROVIDER="${DEMO_PROVIDER:-blockfrost}"
  fi
  if [[ "${SLD_SET}" -eq 0 ]]; then
    SLD="www"
  fi

  case "${MODE}" in fresh|existing) ;; *) die "invalid mode: ${MODE}" ;; esac
  case "${PROVIDER}" in blockfrost|utxorpc) ;; *) die "invalid provider: ${PROVIDER}" ;; esac

  if [[ -f "${STATE_FILE}" ]]; then
    STATE="$(cat "${STATE_FILE}")"
    [[ "$(echo "${STATE}" | jq -r '.schemaVersion')" == "1" ]] \
      || die "unsupported state.schemaVersion (want 1)"
    if [[ "${TLD_SET}" -eq 0 ]]; then
      TLD="$(echo "${STATE}" | jq -r '.tld')"
    fi
    if [[ "${SLD_SET}" -eq 0 ]]; then
      SLD="$(echo "${STATE}" | jq -r '.sld')"
      [[ -n "${SLD}" && "${SLD}" != "null" ]] || SLD="www"
    fi
    if [[ "${MODE_SET}" -eq 0 ]]; then
      MODE="$(echo "${STATE}" | jq -r '.mode')"
    fi
    if [[ "${PROVIDER_SET}" -eq 0 && -z "${DEMO_PROVIDER:-}" ]]; then
      PROVIDER="$(echo "${STATE}" | jq -r '.provider')"
    fi
    STATE="$(echo "${STATE}" | jq \
      --arg mode "${MODE}" \
      --arg provider "${PROVIDER}" \
      --arg tld "${TLD}" \
      --arg sld "${SLD}" \
      '.mode=$mode | .provider=$provider | .tld=$tld | .sld=$sld | .network="preprod"')"
  else
    TLD="${TLD:-${default_tld}}"
    STATE="$(default_state_json "${TLD}" "${SLD}" "${MODE}" "${PROVIDER}")"
  fi
  save_state_atomic "${STATE}"
  log "State: ${STATE_FILE} (mode=${MODE} provider=${PROVIDER} tld=${TLD} sld=${SLD})"
}

read_payment_addr() {
  local dir="$1"
  local addr
  addr="$(tr -d '[:space:]' <"${dir}/payment.addr")"
  [[ -n "${addr}" ]] || die "empty payment.addr in ${dir}"
  printf '%s' "${addr}"
}

ensure_wallets() {
  local name out
  for name in bootstrap registrar tld-owner sld-owner; do
    out="${RUNTIME_DIR}/wallets/${name}"
    if [[ -f "${out}/payment.addr" && -f "${out}/payment.skey" ]]; then
      log "Wallet exists: ${name}"
      continue
    fi
    log "Creating wallet: ${name}"
    "${DNS_CLI}" wallet create \
      --name "${name}" \
      --network preprod \
      --format key-envelope \
      --out-dir "${out}" \
      --output json >/dev/null
  done
}

write_bootstrap_config() {
  local bootstrap_addr registrar_addr tld_owner_addr sld_owner_addr
  local out="${RUNTIME_DIR}/config/bootstrap.json"
  mkdir -p "${RUNTIME_DIR}/config"

  bootstrap_addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/bootstrap")"
  registrar_addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/registrar")"
  tld_owner_addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/tld-owner")"
  sld_owner_addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/sld-owner")"

  local provider_block
  if [[ "${PROVIDER}" == "blockfrost" ]]; then
    provider_block='{"type":"blockfrost","baseURL":"https://cardano-preprod.blockfrost.io/api/v0","projectIdEnv":"DNS_CLI_BLOCKFROST_PROJECT_ID"}'
  else
    provider_block='{"type":"utxorpc","baseUrlEnv":"DNS_CLI_UTXORPC_URL","headersEnv":"DNS_CLI_UTXORPC_HEADERS"}'
  fi

  jq -n \
    --argjson provider "${provider_block}" \
    --arg bootstrapAddr "${bootstrap_addr}" \
    --arg registrarAddr "${registrar_addr}" \
    --arg tldOwnerAddr "${tld_owner_addr}" \
    --arg sldOwnerAddr "${sld_owner_addr}" \
    '{
      version: 1,
      defaultProfile: "preprod",
      profiles: {
        preprod: {
          network: {
            name: "preprod",
            id: 0,
            magic: 1,
            explorerTxURL: "https://preprod.cexplorer.io/tx/{txId}"
          },
          provider: $provider,
          contracts: {
            blueprintPath: "../../fixtures/contracts/plutus.json",
            tldRegistrarAddress: "addr_test1...",
            tldReferenceAddress: "addr_test1...",
            sldReferenceAddress: "addr_test1...",
            tldRegistrarPolicyId: "REPLACE_ME",
            tldReferencePolicyId: "REPLACE_ME",
            sldReferencePolicyId: "REPLACE_ME",
            referenceUtxos: {
              tldRegistrar: "REPLACE_ME_TXHASH#0",
              tldReference: "REPLACE_ME_TXHASH#1",
              sldReference: "REPLACE_ME_TXHASH#2"
            }
          },
          actors: {
            bootstrap: {
              address: $bootstrapAddr,
              signingKeyFile: "../wallets/bootstrap/payment.skey"
            },
            registrar: {
              address: $registrarAddr,
              signingKeyFile: "../wallets/registrar/payment.skey"
            },
            tldOwner: {
              address: $tldOwnerAddr,
              signingKeyFile: "../wallets/tld-owner/payment.skey"
            },
            sldOwner: {
              address: $sldOwnerAddr,
              signingKeyFile: "../wallets/sld-owner/payment.skey"
            }
          },
          transaction: {
            ttlSlots: 300,
            confirmationTimeout: "10m",
            pollInterval: "5s",
            artifactDir: "../artifacts",
            maxDatumBytes: 4000
          }
        }
      }
    }' >"${out}.tmp"
  mv -f "${out}.tmp" "${out}"
  BOOTSTRAP_CONFIG="${out}"
  log "Wrote ${BOOTSTRAP_CONFIG}"
  "${DNS_CLI}" config validate --config "${BOOTSTRAP_CONFIG}" --output json >/dev/null \
    || die "bootstrap.json failed offline validation"
}

wait_for_bootstrap_funds() {
  local json lovelace addr
  addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/bootstrap")"
  log "Waiting for bootstrap balance >= ${MIN_BOOTSTRAP_LOVELACE} lovelace (150 ADA)"
  log "Fund address via Preprod faucet: ${FAUCET_URL}"
  log "Bootstrap address: ${addr}"
  while true; do
    json="$("${DNS_CLI}" wallet balance --config "${BOOTSTRAP_CONFIG}" --actor bootstrap --output json)"
    lovelace="$(echo "${json}" | jq -r '.data.lovelace')"
    if [[ "${lovelace}" =~ ^[0-9]+$ ]] && (( lovelace >= MIN_BOOTSTRAP_LOVELACE )); then
      log "Bootstrap funded: ${lovelace} lovelace"
      return 0
    fi
    log "Current lovelace=${lovelace:-unknown}; sleep ${POLL_SECONDS}s (faucet: ${FAUCET_URL})"
    sleep "${POLL_SECONDS}"
  done
}

ensure_proof_and_prepare() {
  mkdir -p "${RUNTIME_DIR}/proofs" "${RUNTIME_DIR}/contracts"
  local proof_bundle="${RUNTIME_DIR}/proofs/proof-bundle.json"
  if [[ ! -f "${proof_bundle}" ]]; then
    log "Generating proof bundle for tld=${TLD}"
    "${DNS_CLI}" proof generate --tld "${TLD}" --out-dir "${RUNTIME_DIR}/proofs" --output json >/dev/null
  else
    log "Proof bundle exists: ${proof_bundle}"
  fi
  [[ -f "${RUNTIME_DIR}/proofs/registrar.hns" ]] || die "missing registrar.hns after proof generate"

  local deployment="${RUNTIME_DIR}/contracts/deployment.json"
  if [[ ! -f "${deployment}" ]]; then
    need_cmd aiken
    log "Running system prepare (aiken build + parameterize)"
    "${DNS_CLI}" system prepare \
      --blueprint "${DEMO_ROOT}/fixtures/contracts" \
      --registrar-hns-key "${RUNTIME_DIR}/proofs/registrar.hns" \
      --stake-key "${RUNTIME_DIR}/wallets/bootstrap/stake.vkey" \
      --network preprod \
      --out-dir "${RUNTIME_DIR}/contracts" \
      --output json >/dev/null
  else
    log "Deployment exists: ${deployment}"
  fi
  DEPLOYMENT_JSON="${deployment}"
  PROOF_BUNDLE="${proof_bundle}"
}

print_preflight() {
  cat <<EOF

======== Preprod demo preflight ========
DEMO_ROOT:     ${DEMO_ROOT}
CLI:           ${DNS_CLI}
mode:          ${MODE}
provider:      ${PROVIDER}
tld / sld:     ${TLD} / ${SLD}
bootstrap cfg: ${BOOTSTRAP_CONFIG}
deployment:    ${DEPLOYMENT_JSON}
proof:         ${PROOF_BUNDLE}
records:       ${RECORDS_FILE}
artifacts:     ${RUNTIME_DIR}/artifacts
state:         ${STATE_FILE}
schema:        ${STATE_SCHEMA}
Funding:       registrar=30 ADA, tldOwner=50 ADA, sldOwner=30 ADA (+5 ADA collateral each)
========================================

EOF
}

confirm_proceed() {
  local ans
  read -r -p "Proceed with Preprod submissions? [y/N] " ans || true
  [[ "${ans}" =~ ^[Yy]$ ]] || { log "Aborted before submissions."; exit 0; }
}

step_tx_id() {
  echo "${STATE}" | jq -r --arg k "$1" '.confirmed[$k].txId // ""'
}

mark_confirmed() {
  local key="$1" tx_id="$2" manifest="$3"
  STATE="$(echo "${STATE}" | jq \
    --arg key "${key}" \
    --arg txId "${tx_id}" \
    --arg manifest "${manifest}" \
    '.confirmed[$key] = {txId: $txId, manifest: $manifest}')"
  save_state_atomic "${STATE}"
  log "Confirmed step ${key}: txId=${tx_id}"
}

# Shared submit cycle: sign -> submit -> status --wait, then persist state.
# Args: step_key actor unsigned_path signed_path manifest_path
submit_cycle() {
  local step_key="$1" actor="$2" unsigned="$3" signed="$4" manifest="$5" config="$6"
  local json tx_id

  log "Signing ${step_key} as ${actor}"
  "${DNS_CLI}" tx sign \
    --config "${config}" \
    --tx "${unsigned}" \
    --actor "${actor}" \
    --out "${signed}" \
    --output json >/dev/null

  log "Submitting ${step_key}"
  json="$("${DNS_CLI}" tx submit --config "${config}" --tx "${signed}" --output json)"
  tx_id="$(echo "${json}" | jq -r '.txId')"
  [[ -n "${tx_id}" && "${tx_id}" != "null" ]] || die "submit returned empty txId for ${step_key}"

  log "Waiting for confirmation: ${tx_id}"
  "${DNS_CLI}" tx status \
    --config "${config}" \
    --tx-id "${tx_id}" \
    --manifest "${manifest}" \
    --wait \
    --output json >/dev/null

  mark_confirmed "${step_key}" "${tx_id}" "${manifest}"
}

bound_config_path() {
  echo "${RUNTIME_DIR}/config/${PROVIDER}.json"
}

ensure_bound_config() {
  local out base deploy_tx
  out="$(bound_config_path)"
  deploy_tx="$(step_tx_id deploy)"
  [[ -n "${deploy_tx}" ]] || die "cannot bind: deploy not confirmed"
  base="${DEMO_ROOT}/config/${PROVIDER}.template.json"
  [[ -f "${base}" ]] || die "missing base template: ${base}"
  log "Binding provider config -> ${out}"
  "${DNS_CLI}" system bind \
    --base-config "${base}" \
    --deployment "${DEPLOYMENT_JSON}" \
    --tx-id "${deploy_tx}" \
    --actor-dir "${RUNTIME_DIR}/wallets" \
    --provider "${PROVIDER}" \
    --out "${out}" \
    --force \
    --output json >/dev/null
  BOUND_CONFIG="${out}"
}

run_fresh_submissions() {
  mkdir -p "${RUNTIME_DIR}/artifacts"
  local prefix unsigned signed manifest json
  local config="${BOOTSTRAP_CONFIG}"

  # --- fund ---
  if [[ -z "$(step_tx_id fund)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/00-fund"
    log "Building wallet fund"
    "${DNS_CLI}" wallet fund \
      --config "${config}" \
      --from-actor bootstrap \
      --allocation "registrar=30000000" \
      --allocation "tldOwner=50000000" \
      --allocation "sldOwner=30000000" \
      --collateral 5000000 \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle fund bootstrap \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping fund (already confirmed)"
  fi

  # --- deploy (system init) ---
  if [[ -z "$(step_tx_id deploy)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/01-deploy"
    log "Building system init"
    "${DNS_CLI}" system init \
      --config "${config}" \
      --deployment "${DEPLOYMENT_JSON}" \
      --actor bootstrap \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle deploy bootstrap \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping deploy (already confirmed)"
  fi

  ensure_bound_config
  config="${BOUND_CONFIG}"

  # --- register ---
  if [[ -z "$(step_tx_id register)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/02-register"
    log "Building register-tld"
    "${DNS_CLI}" registrar register-tld \
      --config "${config}" \
      --tld "${TLD}" \
      --proof "${PROOF_BUNDLE}" \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle register registrar \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping register (already confirmed)"
  fi

  # --- activate ---
  if [[ -z "$(step_tx_id activate)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/03-activate"
    log "Building activate-tld"
    "${DNS_CLI}" owner activate-tld \
      --config "${config}" \
      --tld "${TLD}" \
      --proof "${PROOF_BUNDLE}" \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle activate tldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping activate (already confirmed)"
  fi

  # --- mint SLD ---
  if [[ -z "$(step_tx_id mintSld)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/04-mint-sld"
    log "Building mint-sld"
    "${DNS_CLI}" owner mint-sld \
      --config "${config}" \
      --tld "${TLD}" \
      --sld "${SLD}" \
      --sld-owner sldOwner \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle mintSld tldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping mintSld (already confirmed)"
  fi

  # --- update SLD ---
  if [[ -z "$(step_tx_id updateSld)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/05-update-sld"
    [[ -f "${RECORDS_FILE}" ]] || die "missing records file: ${RECORDS_FILE}"
    log "Building update-sld"
    "${DNS_CLI}" owner update-sld \
      --config "${config}" \
      --tld "${TLD}" \
      --sld "${SLD}" \
      --records "${RECORDS_FILE}" \
      --out "${prefix}" \
      --output json >/dev/null
    submit_cycle updateSld sldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping updateSld (already confirmed)"
  fi
}

print_success_summary() {
  cat <<EOF

======== Demo complete ========
tld / sld:  ${TLD} / ${SLD}
provider:   ${PROVIDER}
config:     $(bound_config_path)
fund:       $(step_tx_id fund)
deploy:     $(step_tx_id deploy)
register:   $(step_tx_id register)
activate:   $(step_tx_id activate)
mintSld:    $(step_tx_id mintSld)
updateSld:  $(step_tx_id updateSld)
state:      ${STATE_FILE}
Explorer:   https://preprod.cexplorer.io/tx/$(step_tx_id updateSld)
===============================
EOF
}

run_existing_mode() {
  local cfg="${DEMO_ROOT}/config/existing.${PROVIDER}.json"
  [[ -f "${cfg}" ]] || die "missing existing config: ${cfg}"
  log "Existing mode: validating ${cfg} offline"
  if "${DNS_CLI}" config validate --config "${cfg}" --output json; then
    log "Offline validation OK"
  else
    log "WARNING: offline validation reported errors (historical fixture may share actor addresses)."
  fi

  cat <<EOF

======== Historical Preprod deployment ========
Config: ${cfg}
Init tx: ef635b55fce6abc39cd4c843722d9d574cb719114e224f2cd1c8747d5abfc19e

| Role          | Ref | Policy ID                                                      |
|---------------|-----|----------------------------------------------------------------|
| tldRegistrar  | #0  | ea32305e62561a0c0bb69588a936afb6fabd0fb4d2cc2a6c67363e9d |
| tldReference  | #1  | 694cb48da919e928b3e51c4648f051326ac150eaa9436792ec7a6e35 |
| sldReference  | #2  | 96512d4c426d912ba453014e74a57d655dfb3980154c4de106f69320 |

Addresses: ${DEMO_ROOT}/fixtures/preprod/validators/*.addr
Tx evidence: ${DEMO_ROOT}/fixtures/preprod/tx/ (not replayable)
No submissions performed in existing mode.
===============================================
EOF
}

main() {
  resolve_cli
  need_cmd jq
  load_or_create_state

  if [[ "${MODE}" == "existing" ]]; then
    run_existing_mode
    exit 0
  fi

  check_provider_env
  need_cmd aiken
  mkdir -p \
    "${RUNTIME_DIR}/wallets" \
    "${RUNTIME_DIR}/config" \
    "${RUNTIME_DIR}/proofs" \
    "${RUNTIME_DIR}/contracts" \
    "${RUNTIME_DIR}/artifacts"

  ensure_wallets
  write_bootstrap_config
  wait_for_bootstrap_funds
  ensure_proof_and_prepare
  print_preflight
  confirm_proceed
  run_fresh_submissions
  print_success_summary
}

main "$@"
