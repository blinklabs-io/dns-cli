#!/usr/bin/env bash
# =============================================================================
# run-demo.sh — Resumable Cardano Preprod end-to-end demo for dns-cli
# =============================================================================
#
# Purpose
#   Drive a full lifecycle on Preprod: wallets → funding → system prepare/init →
#   register/activate TLD → mint/update SLD. State is stored under runtime/ so
#   interrupted runs can resume without re-submitting confirmed steps.
#
# Modes
#   fresh     Create wallets, deploy contracts, and submit the full tx chain.
#   existing  Offline validation + print historical Preprod fixture evidence
#             (no live submissions).
#
# Providers
#   blockfrost  Needs DNS_CLI_BLOCKFROST_PROJECT_ID
#   utxorpc     Needs DNS_CLI_UTXORPC_URL (optional DNS_CLI_UTXORPC_HEADERS)
#
# Logging
#   --extensive-logging / -E / --verbose / -v
#   --log-level quiet|normal|extensive
#   Env: DEMO_LOG_LEVEL=quiet|normal|extensive
#        DEMO_EXTENSIVE_LOGGING=1
#   Extensive also passes -v N to dns-cli (see init_logging / CLI_VERBOSE).
#
# Prerequisites
#   --yes / DEMO_ASSUME_YES=1   auto-approve install/set prompts
#   --skip-install              only print guides; do not install or write .env
#   Credentials may be saved to runtime/.env (gitignored).
#
# Security
#   Fixture/runtime keys are Preprod test material only — never use on mainnet.
#
# Usage
#   ./run-demo.sh [--mode fresh|existing] [--provider blockfrost|utxorpc]
#                 [--tld NAME] [--sld NAME] [--extensive-logging]
#                 [--log-level quiet|normal|extensive] [--yes] [--skip-install]
# =============================================================================
set -euo pipefail

# --- Paths & constants (all relative to this demo/ folder) --------------------
DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${DEMO_ROOT}/runtime"          # generated wallets/artifacts/state
STATE_FILE="${RUNTIME_DIR}/state.json"      # resumable confirmed-step ledger
STATE_SCHEMA="${DEMO_ROOT}/state.schema.json"
RECORDS_FILE="${DEMO_ROOT}/records.json"    # DNS records for update-sld
ENV_FILE="${RUNTIME_DIR}/.env"              # local credentials (gitignored via runtime/)
TOOLS_DIR="${RUNTIME_DIR}/tools"            # optional local tool installs
CLI_ROOT="$(cd "${DEMO_ROOT}/.." && pwd)"
APOLLO_DIR="$(cd "${CLI_ROOT}/.." && pwd)/apollo"
FAUCET_URL="https://docs.cardano.org/cardano-testnets/tools/faucet/"
MIN_BOOTSTRAP_LOVELACE=150000000            # 150 ADA before starting fund tx
POLL_SECONDS=20                             # balance poll interval
BLOCKFROST_DASHBOARD="https://blockfrost.io/dashboard"
AIKEN_INSTALL_DOCS="https://aiken-lang.org/installation-instructions"
GO_INSTALL_DOCS="https://go.dev/dl/"
JQ_RELEASES="https://github.com/jqlang/jq/releases"

MODE=""
PROVIDER=""
TLD=""
SLD=""
TLD_SET=0
SLD_SET=0
MODE_SET=0
PROVIDER_SET=0
LOG_LEVEL="normal"   # quiet | normal | extensive
CLI_VERBOSE=2        # dns-cli -v level (0–4); raised under extensive
EXTENSIVE_FLAG=0
LOG_LEVEL_SET=0
ASSUME_YES=0
SKIP_INSTALL=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

  --mode fresh|existing     Demo mode (default: fresh, or DEMO_MODE)
  --provider blockfrost|utxorpc
                            Provider (default: blockfrost, or DEMO_PROVIDER)
  --tld NAME                TLD label (default: demo-<timestamp>)
  --sld NAME                SLD label (default: www)
  --log-level quiet|normal|extensive
                            Runner logging detail (default: normal)
  -E, --extensive-logging, -v, --verbose
                            Shortcut for --log-level extensive
  -y, --yes                 Auto-approve prerequisite install/set prompts
  --skip-install            Only print guides; do not install or write .env
  -h, --help                Show this help

Environment:
  CLI                         Path to dns-cli binary (default: ../dns-cli[.exe])
  DEMO_PROVIDER / DEMO_MODE   Defaults when flags omitted
  DEMO_LOG_LEVEL              quiet|normal|extensive
  DEMO_EXTENSIVE_LOGGING      1/true/yes/on enables extensive logging
  DEMO_ASSUME_YES             1/true/yes/on same as --yes
  DNS_CLI_BLOCKFROST_PROJECT_ID   Required for blockfrost (or prompt → runtime/.env)
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
    --log-level)
      LOG_LEVEL="${2:-}"
      LOG_LEVEL_SET=1
      shift 2
      ;;
    -E|--extensive-logging|-v|--verbose)
      EXTENSIVE_FLAG=1
      shift
      ;;
    -y|--yes)
      ASSUME_YES=1
      shift
      ;;
    --skip-install)
      SKIP_INSTALL=1
      shift
      ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# -----------------------------------------------------------------------------
# Logging helpers
#   log     always-on progress (suppressed in quiet)
#   log_ext extensive/debug detail (paths, timings, redacted env)
#   die     always printed
# -----------------------------------------------------------------------------

init_logging() {
  # Precedence: -E/--verbose > --log-level > DEMO_EXTENSIVE_LOGGING > DEMO_LOG_LEVEL > normal
  if [[ "${EXTENSIVE_FLAG}" -eq 1 ]]; then
    LOG_LEVEL="extensive"
  elif [[ "${LOG_LEVEL_SET}" -eq 1 ]]; then
    LOG_LEVEL="$(printf '%s' "${LOG_LEVEL}" | tr '[:upper:]' '[:lower:]')"
  elif [[ "${DEMO_EXTENSIVE_LOGGING:-}" =~ ^(1|true|yes|on)$ ]]; then
    LOG_LEVEL="extensive"
  elif [[ -n "${DEMO_LOG_LEVEL:-}" ]]; then
    LOG_LEVEL="$(printf '%s' "${DEMO_LOG_LEVEL}" | tr '[:upper:]' '[:lower:]')"
  else
    LOG_LEVEL="normal"
  fi

  case "${LOG_LEVEL}" in
    0|quiet|q) LOG_LEVEL="quiet"; CLI_VERBOSE=1 ;;
    2|extensive|debug|verbose|ext|v) LOG_LEVEL="extensive"; CLI_VERBOSE=4 ;;
    1|normal|n|"") LOG_LEVEL="normal"; CLI_VERBOSE=2 ;;
    *) die "invalid --log-level: ${LOG_LEVEL} (want quiet|normal|extensive)" ;;
  esac
}

log() {
  [[ "${LOG_LEVEL}" == "quiet" ]] && return 0
  printf '[demo %s] %s\n' "$(date +%H:%M:%S)" "$*"
}

log_ext() {
  [[ "${LOG_LEVEL}" == "extensive" ]] || return 0
  # Prefer millisecond timestamps when GNU date is available; fall back otherwise.
  local ts
  ts="$(date +%H:%M:%S.%3N 2>/dev/null || date +%H:%M:%S)"
  printf '[demo:ext %s] %s\n' "${ts}" "$*"
}

die() {
  printf '[demo %s] ERROR: %s\n' "$(date +%H:%M:%S)" "$*" >&2
  exit 1
}

redacted_env_summary() {
  # Never print secret values; only presence / length for provider credentials.
  local name val
  for name in CLI DEMO_MODE DEMO_PROVIDER DEMO_LOG_LEVEL DEMO_EXTENSIVE_LOGGING \
    DNS_CLI_BLOCKFROST_PROJECT_ID DNS_CLI_UTXORPC_URL DNS_CLI_UTXORPC_HEADERS; do
    val="${!name-}"
    if [[ -z "${val}" ]]; then
      printf '  %s=<unset>\n' "${name}"
    elif [[ "${name}" =~ PROJECT_ID|HEADERS|KEY|SECRET|TOKEN|PASSWORD ]]; then
      printf '  %s=<set len=%s>\n' "${name}" "${#val}"
    else
      printf '  %s=%s\n' "${name}" "${val}"
    fi
  done
}

startup_banner() {
  log "LogLevel=${LOG_LEVEL} dns-cli -v ${CLI_VERBOSE}"
  log_ext "DEMO_ROOT=${DEMO_ROOT}"
  log_ext "RUNTIME_DIR=${RUNTIME_DIR}"
  log_ext "STATE_FILE=${STATE_FILE}"
  log_ext "bash=${BASH_VERSION} host=$(uname -s 2>/dev/null || echo unknown)"
  if [[ "${LOG_LEVEL}" == "extensive" ]]; then
    log_ext "Environment:"
    redacted_env_summary | while IFS= read -r line; do log_ext "${line}"; done
  fi
}

# =============================================================================
# Prerequisites — detect, optionally install/set with consent, else guide
# =============================================================================

confirm_yes() {
  local prompt="$1" ans
  if [[ "${ASSUME_YES}" -eq 1 || "${DEMO_ASSUME_YES:-}" =~ ^(1|true|yes|on)$ ]]; then
    log "${prompt} -> yes (auto)"
    return 0
  fi
  read -r -p "${prompt} [y/N] " ans || true
  [[ "${ans}" =~ ^[Yy]$ ]]
}

show_guide() {
  local title="$1"
  shift
  printf '\n-------- How to get: %s --------\n' "${title}"
  local line
  for line in "$@"; do
    # Always use -- so lines starting with '-' are not treated as printf options.
    printf -- '  %s\n' "${line}"
  done
  printf '%s\n\n' '----------------------------------------'
}

load_env_file() {
  # Load KEY=VALUE from runtime/.env into this process (no shell eval of values).
  [[ -f "${ENV_FILE}" ]] || { log_ext "no env file at ${ENV_FILE}"; return 0; }
  log "Loading credentials from ${ENV_FILE}"
  local line key val
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    [[ "${line}" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    val="${BASH_REMATCH[2]}"
    if [[ "${val}" =~ ^\".*\"$ || "${val}" =~ ^\'.*\'$ ]]; then
      val="${val:1:${#val}-2}"
    fi
    if [[ -z "${!key:-}" ]]; then
      export "${key}=${val}"
      log_ext "imported ${key} from .env (len=${#val})"
    else
      log_ext "keeping existing process env for ${key}"
    fi
  done <"${ENV_FILE}"
}

save_env_var() {
  local name="$1" value="$2" tmp
  export "${name}=${value}"
  mkdir -p "${RUNTIME_DIR}"
  tmp="${ENV_FILE}.tmp"
  if [[ -f "${ENV_FILE}" ]]; then
    grep -v -E "^[[:space:]]*${name}=" "${ENV_FILE}" >"${tmp}" || true
  else
    : >"${tmp}"
  fi
  printf '%s=%s\n' "${name}" "${value}" >>"${tmp}"
  mv -f "${tmp}" "${ENV_FILE}"
  log "Saved ${name} to ${ENV_FILE} (process env updated)"
}

prepend_tools_path() {
  mkdir -p "${TOOLS_DIR}"
  export PATH="${TOOLS_DIR}:${HOME}/.aiken/bin:${PATH}"
  log_ext "PATH prepended with ${TOOLS_DIR} and ~/.aiken/bin"
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

find_dns_cli() {
  if [[ -n "${CLI:-}" && -f "${CLI}" ]]; then
    printf '%s' "${CLI}"
    return 0
  fi
  if [[ -x "${CLI_ROOT}/dns-cli" ]]; then
    printf '%s' "${CLI_ROOT}/dns-cli"
    return 0
  fi
  if [[ -f "${CLI_ROOT}/dns-cli.exe" ]]; then
    printf '%s' "${CLI_ROOT}/dns-cli.exe"
    return 0
  fi
  if command -v dns-cli >/dev/null 2>&1; then
    command -v dns-cli
    return 0
  fi
  return 1
}

install_dns_cli_from_source() {
  log "Attempting to build dns-cli from source..."
  if ! has_cmd go; then
    show_guide "Go toolchain" \
      "Go is required to build dns-cli. Install from ${GO_INSTALL_DOCS}" \
      "Then reopen this terminal and re-run the demo." \
      "Expected Apollo checkout at: ${APOLLO_DIR}"
    return 1
  fi
  if [[ ! -f "${APOLLO_DIR}/go.mod" ]]; then
    show_guide "Apollo v2 checkout" \
      "dns-cli go.mod replace expects Apollo at: ${APOLLO_DIR}" \
      "Clone it next to the dns-cli module:" \
      "  git clone https://github.com/Salvionied/apollo.git \"${APOLLO_DIR}\"" \
      "  cd \"${APOLLO_DIR}\" && git checkout b2f56d0c6e9d22316b6938feeb325bdbab3846d2" \
      "Then re-run this demo."
    return 1
  fi
  local out="${CLI_ROOT}/dns-cli"
  case "$(uname -s 2>/dev/null || true)" in
    MINGW*|MSYS*|CYGWIN*) out="${CLI_ROOT}/dns-cli.exe" ;;
  esac
  (
    cd "${CLI_ROOT}"
    log "go build -o ${out} ./cmd/dns-cli"
    go build -o "${out}" ./cmd/dns-cli
  ) || {
    show_guide "dns-cli build failed" \
      "Manual build from ${CLI_ROOT}:" \
      "  go build -o dns-cli ./cmd/dns-cli" \
      "See docs/installation.md for Apollo + Go version pins."
    return 1
  }
  [[ -f "${out}" ]] || return 1
  export CLI="${out}"
  log "Built dns-cli: ${out}"
  return 0
}

ensure_dns_cli() {
  local found
  if found="$(find_dns_cli)"; then
    DNS_CLI="${found}"
    log "Using CLI: ${DNS_CLI}"
    return 0
  fi

  log "dns-cli binary not found."
  show_guide "dns-cli" \
    "Set CLI to a built binary, or build from ${CLI_ROOT}:" \
    "  go build -o dns-cli ./cmd/dns-cli" \
    "Apollo must exist at ${APOLLO_DIR} (see go.mod replace)." \
    "Docs: docs/installation.md"

  if [[ "${SKIP_INSTALL}" -eq 1 ]]; then
    die "dns-cli missing and --skip-install was set"
  fi
  confirm_yes "Build dns-cli from source now?" || die "dns-cli is required. Build it or set CLI=..."
  install_dns_cli_from_source || die "could not build dns-cli; follow the guide above"
  DNS_CLI="$(find_dns_cli)" || die "dns-cli still missing after build attempt"
  log "Using CLI: ${DNS_CLI}"
}

install_jq_interactive() {
  log "Attempting jq install..."
  prepend_tools_path
  has_cmd jq && return 0

  if has_cmd brew; then
    confirm_yes "Install jq with Homebrew (brew install jq)?" || return 1
    brew install jq || return 1
  elif has_cmd apt-get; then
    confirm_yes "Install jq with apt-get (may prompt for sudo)?" || return 1
    sudo apt-get update && sudo apt-get install -y jq || return 1
  elif has_cmd dnf; then
    confirm_yes "Install jq with dnf (may prompt for sudo)?" || return 1
    sudo dnf install -y jq || return 1
  elif has_cmd curl; then
    confirm_yes "Download a portable jq binary into runtime/tools?" || return 1
    mkdir -p "${TOOLS_DIR}"
    local os arch asset url
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    case "${os}-${arch}" in
      linux-x86_64|linux-amd64) asset="jq-linux-amd64" ;;
      linux-aarch64|linux-arm64) asset="jq-linux-arm64" ;;
      darwin-arm64) asset="jq-macos-arm64" ;;
      darwin-x86_64) asset="jq-macos-amd64" ;;
      mingw*|msys*|cygwin*) asset="jq-windows-amd64.exe" ;;
      *)
        show_guide "jq" \
          "Could not detect a portable binary for ${os}/${arch}." \
          "Install manually from ${JQ_RELEASES}" \
          "Or: brew install jq / apt install jq"
        return 1
        ;;
    esac
    url="https://github.com/jqlang/jq/releases/latest/download/${asset}"
    log "Downloading ${url}"
    curl -fsSL -o "${TOOLS_DIR}/jq" "${url}" || curl -fsSL -o "${TOOLS_DIR}/jq.exe" "${url}" || return 1
    if [[ -f "${TOOLS_DIR}/jq" ]]; then
      chmod +x "${TOOLS_DIR}/jq"
    fi
  else
    return 1
  fi
  prepend_tools_path
  has_cmd jq
}

ensure_jq() {
  prepend_tools_path
  if has_cmd jq; then
    log "Using jq: $(command -v jq)"
    return 0
  fi
  log "jq not found (required by the Bash demo runner)."
  show_guide "jq" \
    "Install options:" \
    "  macOS:  brew install jq" \
    "  Debian/Ubuntu:  sudo apt-get install -y jq" \
    "  Fedora:  sudo dnf install -y jq" \
    "  Or download from ${JQ_RELEASES} and put jq on PATH"

  if [[ "${SKIP_INSTALL}" -eq 1 ]]; then
    die "jq missing and --skip-install was set"
  fi
  confirm_yes "Try to install jq now?" || die "jq is required. Install it and re-run."
  install_jq_interactive || die "jq install did not succeed. Follow the guide above."
  log "Using jq: $(command -v jq)"
}

install_aiken_interactive() {
  log "Attempting Aiken install..."
  prepend_tools_path
  has_cmd aiken && return 0

  case "$(uname -s 2>/dev/null || true)" in
    MINGW*|MSYS*|CYGWIN*)
      show_guide "Aiken on Windows Git Bash" \
        "Prefer PowerShell installer:" \
        "  powershell -c \"irm https://windows.aiken-lang.org | iex\"" \
        "Or: cargo install aiken" \
        "Docs: ${AIKEN_INSTALL_DOCS}"
      if has_cmd cargo; then
        confirm_yes "Install aiken with cargo install aiken? (may take several minutes)" || return 1
        cargo install aiken || return 1
      else
        return 1
      fi
      ;;
    *)
      if has_cmd curl; then
        confirm_yes "Install aikup via https://install.aiken-lang.org then run aikup?" || return 1
        curl --proto '=https' --tlsv1.2 -LsSf https://install.aiken-lang.org | sh || return 1
        # shellcheck disable=SC1090
        [[ -f "${HOME}/.aiken/env" ]] && . "${HOME}/.aiken/env" || true
        export PATH="${HOME}/.aiken/bin:${PATH}"
        if has_cmd aikup; then
          aikup || return 1
        fi
      elif has_cmd cargo; then
        confirm_yes "Install aiken with cargo install aiken?" || return 1
        cargo install aiken || return 1
      else
        return 1
      fi
      ;;
  esac
  prepend_tools_path
  has_cmd aiken
}

ensure_aiken() {
  prepend_tools_path
  if has_cmd aiken; then
    log "Using aiken: $(command -v aiken)"
    log_ext "aiken version: $(aiken --version 2>&1 || true)"
    return 0
  fi
  log "aiken not found on PATH (required for fresh mode system prepare)."
  show_guide "Aiken" \
    "Install docs: ${AIKEN_INSTALL_DOCS}" \
    "Windows:  powershell -c \"irm https://windows.aiken-lang.org | iex\"" \
    "macOS/Linux: curl --proto \"=https\" --tlsv1.2 -LsSf https://install.aiken-lang.org | sh && aikup" \
    "Or: cargo install aiken  (requires Rust)" \
    "Then ensure aiken is on PATH and re-run."

  if [[ "${SKIP_INSTALL}" -eq 1 ]]; then
    die "aiken missing and --skip-install was set"
  fi
  confirm_yes "Try to install Aiken now?" || die "aiken is required for fresh mode. Install it and re-run."
  install_aiken_interactive || die "Aiken install did not succeed. Follow the guide above."
  log "Using aiken: $(command -v aiken)"
}

ensure_provider_credentials() {
  case "${PROVIDER}" in
    blockfrost)
      if [[ -n "${DNS_CLI_BLOCKFROST_PROJECT_ID:-}" ]]; then
        log_ext "DNS_CLI_BLOCKFROST_PROJECT_ID is set (len=${#DNS_CLI_BLOCKFROST_PROJECT_ID})"
        return 0
      fi
      log "Missing DNS_CLI_BLOCKFROST_PROJECT_ID for Blockfrost Preprod."
      show_guide "Blockfrost project id" \
        "Open ${BLOCKFROST_DASHBOARD} and create/select a Preprod project." \
        "Copy the project ID (looks like preprod...)." \
        "Set for this shell:  export DNS_CLI_BLOCKFROST_PROJECT_ID=preprod..." \
        "Or let this script save it to ${ENV_FILE}"
      if [[ "${SKIP_INSTALL}" -eq 1 ]]; then
        die "DNS_CLI_BLOCKFROST_PROJECT_ID missing and --skip-install was set"
      fi
      confirm_yes "Enter Blockfrost project id now and save to runtime/.env?" \
        || die "DNS_CLI_BLOCKFROST_PROJECT_ID is required for --provider blockfrost"
      local id
      read -r -p "Blockfrost Preprod project id: " id || true
      [[ -n "${id}" ]] || die "empty project id; cannot continue"
      save_env_var DNS_CLI_BLOCKFROST_PROJECT_ID "$(printf '%s' "${id}" | tr -d '[:space:]')"
      ;;
    utxorpc)
      if [[ -n "${DNS_CLI_UTXORPC_URL:-}" ]]; then
        log_ext "DNS_CLI_UTXORPC_URL=${DNS_CLI_UTXORPC_URL}"
        return 0
      fi
      log "Missing DNS_CLI_UTXORPC_URL for UTxO RPC provider."
      show_guide "UTxO RPC endpoint" \
        "Set DNS_CLI_UTXORPC_URL to your Preprod UTxO RPC base URL (https://...)." \
        "Optional DNS_CLI_UTXORPC_HEADERS as Key=Value,Key2=Value2." \
        "Example:  export DNS_CLI_UTXORPC_URL=https://preprod.example/..."
      if [[ "${SKIP_INSTALL}" -eq 1 ]]; then
        die "DNS_CLI_UTXORPC_URL missing and --skip-install was set"
      fi
      confirm_yes "Enter UTxO RPC URL now and save to runtime/.env?" \
        || die "DNS_CLI_UTXORPC_URL is required for --provider utxorpc"
      local url hdr
      read -r -p "DNS_CLI_UTXORPC_URL: " url || true
      [[ -n "${url}" ]] || die "empty URL; cannot continue"
      save_env_var DNS_CLI_UTXORPC_URL "$(printf '%s' "${url}" | tr -d '[:space:]')"
      if confirm_yes "Also set optional DNS_CLI_UTXORPC_HEADERS?"; then
        read -r -p "DNS_CLI_UTXORPC_HEADERS (Key=Value,...): " hdr || true
        if [[ -n "${hdr}" ]]; then
          save_env_var DNS_CLI_UTXORPC_HEADERS "${hdr}"
        fi
      fi
      ;;
    *) die "unsupported provider: ${PROVIDER}" ;;
  esac
}

ensure_prerequisites() {
  local mode_name="$1"
  log "Checking prerequisites for mode=${mode_name} ..."
  mkdir -p "${RUNTIME_DIR}" "${TOOLS_DIR}"
  load_env_file
  prepend_tools_path
  ensure_dns_cli
  ensure_jq
  if [[ "${mode_name}" == "fresh" ]]; then
    ensure_provider_credentials
    ensure_aiken
  fi
  log "Prerequisites OK"
}

resolve_cli() {
  # Prefer Ensure-DnsCli via ensure_prerequisites for interactive setup.
  local found
  found="$(find_dns_cli)" || die "dns-cli binary not found. Set CLI=... or build ../dns-cli"
  DNS_CLI="${found}"
  [[ -x "${DNS_CLI}" || -f "${DNS_CLI}" ]] || die "CLI not executable: ${DNS_CLI}"
  log "Using CLI: ${DNS_CLI}"
  if [[ "${LOG_LEVEL}" == "extensive" && -f "${DNS_CLI}" ]]; then
    log_ext "CLI size=$(wc -c <"${DNS_CLI}" | tr -d ' ') bytes"
  fi
}

need_cmd() {
  # Legacy hard-fail helper; prefer ensure_* for user-guided setup.
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

check_provider_env() {
  ensure_provider_credentials
}

# Run dns-cli with -v and --output json; log duration under extensive.
# stdout must stay JSON-only (tint/verbose logs go to stderr).
run_cli_json() {
  local start end elapsed out code
  log "dns-cli $*"
  log_ext "full argv: ${DNS_CLI} -v ${CLI_VERBOSE} $* --output json"
  start="$(date +%s)"
  set +e
  out="$("${DNS_CLI}" -v "${CLI_VERBOSE}" "$@" --output json)"
  code=$?
  set -e
  end="$(date +%s)"
  elapsed=$((end - start))
  log_ext "exit=${code} duration_s=${elapsed} stdout_bytes=${#out}"
  if [[ "${code}" -ne 0 ]]; then
    if [[ -n "${out}" ]]; then
      log_ext "stdout tail:"
      printf '%s\n' "${out}" | tail -c 2000 | while IFS= read -r line; do log_ext "${line}"; done
    fi
    die "dns-cli failed (exit ${code}): $*"
  fi
  if [[ -z "${out}" ]]; then
    log_ext "empty JSON stdout (ok for some commands)"
    printf ''
    return 0
  fi
  if [[ "${LOG_LEVEL}" == "extensive" ]]; then
    log_ext "json preview: $(printf '%s' "${out}" | tr '\n' ' ' | head -c 1200)"
  fi
  printf '%s' "${out}"
}

run_cli_quiet() {
  run_cli_json "$@" >/dev/null
}

empty_step() {
  # Placeholder for a not-yet-confirmed lifecycle step in state.json.
  jq -n '{txId:"",manifest:""}'
}

default_state_json() {
  # Initial resumable ledger: schemaVersion 1 with empty confirmed.* steps.
  local tld="$1" sld="$2" mode="$3" provider="$4"
  log_ext "default_state_json tld=${tld} sld=${sld} mode=${mode} provider=${provider}"
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
  # Write via .tmp + mv so a crash mid-write cannot leave a half file.
  local tmp="${STATE_FILE}.tmp"
  printf '%s\n' "$1" >"${tmp}"
  mv -f "${tmp}" "${STATE_FILE}"
  log_ext "save_state_atomic wrote ${STATE_FILE} (${#1} bytes)"
}

load_or_create_state() {
  # Load runtime/state.json if present; otherwise create defaults.
  # Explicit CLI flags / env vars override stored values where intended.
  log_ext "load_or_create_state: begin"
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
    log_ext "loading existing state from ${STATE_FILE}"
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
    log_ext "resume confirmed: fund=$(step_tx_id fund) deploy=$(step_tx_id deploy) register=$(step_tx_id register) activate=$(step_tx_id activate) mintSld=$(step_tx_id mintSld) updateSld=$(step_tx_id updateSld)"
  else
    log_ext "no state file; creating fresh defaults"
    TLD="${TLD:-${default_tld}}"
    STATE="$(default_state_json "${TLD}" "${SLD}" "${MODE}" "${PROVIDER}")"
  fi
  save_state_atomic "${STATE}"
  log "State: ${STATE_FILE} (mode=${MODE} provider=${PROVIDER} tld=${TLD} sld=${SLD})"
}

read_payment_addr() {
  # payment.addr is a single bech32 line written by `wallet create`.
  local dir="$1"
  local addr
  log_ext "read_payment_addr: ${dir}/payment.addr"
  addr="$(tr -d '[:space:]' <"${dir}/payment.addr")"
  [[ -n "${addr}" ]] || die "empty payment.addr in ${dir}"
  log_ext "addr=${addr}"
  printf '%s' "${addr}"
}

ensure_wallets() {
  # Create the four Preprod actors once; skip any wallet that already has addr+skey.
  local name out
  log_ext "ensure_wallets: bootstrap/registrar/tld-owner/sld-owner"
  for name in bootstrap registrar tld-owner sld-owner; do
    out="${RUNTIME_DIR}/wallets/${name}"
    if [[ -f "${out}/payment.addr" && -f "${out}/payment.skey" ]]; then
      log "Wallet exists: ${name}"
      log_ext "reuse ${out}"
      continue
    fi
    log "Creating wallet: ${name}"
    run_cli_quiet wallet create \
      --name "${name}" \
      --network preprod \
      --format key-envelope \
      --out-dir "${out}"
  done
}

write_bootstrap_config() {
  # Temporary profile used only for faucet wait, fund, and system init.
  # Contract placeholders are REPLACE_ME until `system bind` after deploy confirms.
  local bootstrap_addr registrar_addr tld_owner_addr sld_owner_addr
  local out="${RUNTIME_DIR}/config/bootstrap.json"
  log_ext "write_bootstrap_config: begin"
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
  run_cli_quiet config validate --config "${BOOTSTRAP_CONFIG}"
}

wait_for_bootstrap_funds() {
  # Poll until bootstrap has >= 150 ADA so fund+init have enough Lovelace.
  local json lovelace addr attempt=0
  addr="$(read_payment_addr "${RUNTIME_DIR}/wallets/bootstrap")"
  log "Waiting for bootstrap balance >= ${MIN_BOOTSTRAP_LOVELACE} lovelace (150 ADA)"
  log "Fund address via Preprod faucet: ${FAUCET_URL}"
  log "Bootstrap address: ${addr}"
  while true; do
    attempt=$((attempt + 1))
    log_ext "balance poll attempt=${attempt}"
    json="$(run_cli_json wallet balance --config "${BOOTSTRAP_CONFIG}" --actor bootstrap)"
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
  # Generate HNS proof once; run system prepare (Aiken) once for deployment.json.
  log_ext "ensure_proof_and_prepare: begin"
  mkdir -p "${RUNTIME_DIR}/proofs" "${RUNTIME_DIR}/contracts"
  local proof_bundle="${RUNTIME_DIR}/proofs/proof-bundle.json"
  if [[ ! -f "${proof_bundle}" ]]; then
    log "Generating proof bundle for tld=${TLD}"
    run_cli_quiet proof generate --tld "${TLD}" --out-dir "${RUNTIME_DIR}/proofs"
  else
    log "Proof bundle exists: ${proof_bundle}"
  fi
  [[ -f "${RUNTIME_DIR}/proofs/registrar.hns" ]] || die "missing registrar.hns after proof generate"
  log_ext "registrar.hns=${RUNTIME_DIR}/proofs/registrar.hns"

  local deployment="${RUNTIME_DIR}/contracts/deployment.json"
  if [[ ! -f "${deployment}" ]]; then
    need_cmd aiken
    log "Running system prepare (aiken build + parameterize)"
    log_ext "aiken=$(command -v aiken)"
    run_cli_quiet system prepare \
      --blueprint "${DEMO_ROOT}/fixtures/contracts" \
      --registrar-hns-key "${RUNTIME_DIR}/proofs/registrar.hns" \
      --stake-key "${RUNTIME_DIR}/wallets/bootstrap/stake.vkey" \
      --network preprod \
      --out-dir "${RUNTIME_DIR}/contracts"
  else
    log "Deployment exists: ${deployment}"
  fi
  DEPLOYMENT_JSON="${deployment}"
  PROOF_BUNDLE="${proof_bundle}"
}

print_preflight() {
  # Operator checkpoint before any signing/submit (allocations are fixed by design).
  cat <<EOF

======== Preprod demo preflight ========
DEMO_ROOT:     ${DEMO_ROOT}
CLI:           ${DNS_CLI}
logLevel:      ${LOG_LEVEL} (dns-cli -v ${CLI_VERBOSE})
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
  log_ext "operator confirmed; starting submissions"
}

step_tx_id() {
  # Return confirmed tx id for a step key, or '' if missing / empty.
  echo "${STATE}" | jq -r --arg k "$1" '.confirmed[$k].txId // ""'
}

mark_confirmed() {
  # Persist a confirmed on-chain step so resume skips rebuild/resubmit.
  local key="$1" tx_id="$2" manifest="$3"
  STATE="$(echo "${STATE}" | jq \
    --arg key "${key}" \
    --arg txId "${tx_id}" \
    --arg manifest "${manifest}" \
    '.confirmed[$key] = {txId: $txId, manifest: $manifest}')"
  save_state_atomic "${STATE}"
  log "Confirmed step ${key}: txId=${tx_id}"
  log_ext "manifest=${manifest}"
}

# Shared submit cycle: sign -> submit -> status --wait, then persist state.
# Args: step_key actor unsigned_path signed_path manifest_path config
submit_cycle() {
  local step_key="$1" actor="$2" unsigned="$3" signed="$4" manifest="$5" config="$6"
  local json tx_id

  log_ext "submit_cycle step=${step_key} actor=${actor}"
  log_ext "unsigned=${unsigned} signed=${signed} manifest=${manifest} config=${config}"
  [[ -f "${unsigned}" ]] || die "submit cycle missing unsigned for ${step_key}: ${unsigned}"
  [[ -f "${config}" ]] || die "submit cycle missing config for ${step_key}: ${config}"
  if [[ "${LOG_LEVEL}" == "extensive" ]]; then
    log_ext "unsigned size=$(wc -c <"${unsigned}" | tr -d ' ')"
  fi

  log "Signing ${step_key} as ${actor}"
  run_cli_quiet tx sign \
    --config "${config}" \
    --tx "${unsigned}" \
    --actor "${actor}" \
    --out "${signed}"

  log "Submitting ${step_key}"
  json="$(run_cli_json tx submit --config "${config}" --tx "${signed}")"
  tx_id="$(echo "${json}" | jq -r '.txId')"
  [[ -n "${tx_id}" && "${tx_id}" != "null" ]] || die "submit returned empty txId for ${step_key}"

  log "Waiting for confirmation: ${tx_id}"
  run_cli_quiet tx status \
    --config "${config}" \
    --tx-id "${tx_id}" \
    --manifest "${manifest}" \
    --wait

  mark_confirmed "${step_key}" "${tx_id}" "${manifest}"
}

bound_config_path() {
  echo "${RUNTIME_DIR}/config/${PROVIDER}.json"
}

ensure_bound_config() {
  # After deploy confirms, stamp policy IDs / ref UTxOs into the provider template.
  local out base deploy_tx
  out="$(bound_config_path)"
  deploy_tx="$(step_tx_id deploy)"
  [[ -n "${deploy_tx}" ]] || die "cannot bind: deploy not confirmed"
  base="${DEMO_ROOT}/config/${PROVIDER}.template.json"
  [[ -f "${base}" ]] || die "missing base template: ${base}"
  log "Binding provider config -> ${out}"
  log_ext "base=${base} deployTx=${deploy_tx}"
  run_cli_quiet system bind \
    --base-config "${base}" \
    --deployment "${DEPLOYMENT_JSON}" \
    --tx-id "${deploy_tx}" \
    --actor-dir "${RUNTIME_DIR}/wallets" \
    --provider "${PROVIDER}" \
    --out "${out}" \
    --force
  BOUND_CONFIG="${out}"
}

run_fresh_submissions() {
  # Ordered, resumable steps. Each skips when state.confirmed.<key>.txId is set.
  log_ext "run_fresh_submissions: begin"
  mkdir -p "${RUNTIME_DIR}/artifacts"
  local prefix
  local config="${BOOTSTRAP_CONFIG}"

  # --- fund: split bootstrap ADA to registrar / tldOwner / sldOwner ---
  if [[ -z "$(step_tx_id fund)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/00-fund"
    log "Building wallet fund"
    run_cli_quiet wallet fund \
      --config "${config}" \
      --from-actor bootstrap \
      --allocation "registrar=30000000" \
      --allocation "tldOwner=50000000" \
      --allocation "sldOwner=30000000" \
      --collateral 5000000 \
      --out "${prefix}"
    submit_cycle fund bootstrap \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping fund (already confirmed)"
    log_ext "fund txId=$(step_tx_id fund)"
  fi

  # --- deploy: publish parameterized reference scripts (system init) ---
  if [[ -z "$(step_tx_id deploy)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/01-deploy"
    log "Building system init"
    run_cli_quiet system init \
      --config "${config}" \
      --deployment "${DEPLOYMENT_JSON}" \
      --actor bootstrap \
      --out "${prefix}"
    submit_cycle deploy bootstrap \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping deploy (already confirmed)"
    log_ext "deploy txId=$(step_tx_id deploy)"
  fi

  # Bound config carries real policy IDs / ref UTxOs for later protocol txs.
  ensure_bound_config
  config="${BOUND_CONFIG}"

  # --- register TLD (registrar signature + HNS proof) ---
  if [[ -z "$(step_tx_id register)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/02-register"
    log "Building register-tld"
    run_cli_quiet registrar register-tld \
      --config "${config}" \
      --tld "${TLD}" \
      --proof "${PROOF_BUNDLE}" \
      --out "${prefix}"
    submit_cycle register registrar \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping register (already confirmed)"
    log_ext "register txId=$(step_tx_id register)"
  fi

  # --- activate TLD (owner claim) ---
  if [[ -z "$(step_tx_id activate)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/03-activate"
    log "Building activate-tld"
    run_cli_quiet owner activate-tld \
      --config "${config}" \
      --tld "${TLD}" \
      --proof "${PROOF_BUNDLE}" \
      --out "${prefix}"
    submit_cycle activate tldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping activate (already confirmed)"
    log_ext "activate txId=$(step_tx_id activate)"
  fi

  # --- mint SLD under the TLD ---
  if [[ -z "$(step_tx_id mintSld)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/04-mint-sld"
    log "Building mint-sld"
    run_cli_quiet owner mint-sld \
      --config "${config}" \
      --tld "${TLD}" \
      --sld "${SLD}" \
      --sld-owner sldOwner \
      --out "${prefix}"
    submit_cycle mintSld tldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping mintSld (already confirmed)"
    log_ext "mintSld txId=$(step_tx_id mintSld)"
  fi

  # --- replace SLD DNS records from demo/records.json ---
  if [[ -z "$(step_tx_id updateSld)" ]]; then
    prefix="${RUNTIME_DIR}/artifacts/05-update-sld"
    [[ -f "${RECORDS_FILE}" ]] || die "missing records file: ${RECORDS_FILE}"
    log "Building update-sld"
    log_ext "records=${RECORDS_FILE}"
    run_cli_quiet owner update-sld \
      --config "${config}" \
      --tld "${TLD}" \
      --sld "${SLD}" \
      --records "${RECORDS_FILE}" \
      --out "${prefix}"
    submit_cycle updateSld sldOwner \
      "${prefix}.unsigned.json" "${prefix}.signed.json" "${prefix}.manifest.json" "${config}"
  else
    log "Skipping updateSld (already confirmed)"
    log_ext "updateSld txId=$(step_tx_id updateSld)"
  fi
}

print_success_summary() {
  cat <<EOF

======== Demo complete ========
tld / sld:  ${TLD} / ${SLD}
provider:   ${PROVIDER}
logLevel:   ${LOG_LEVEL}
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
  # No chain writes — validate historical config and print fixture evidence.
  log_ext "run_existing_mode: begin"
  local cfg="${DEMO_ROOT}/config/existing.${PROVIDER}.json"
  [[ -f "${cfg}" ]] || die "missing existing config: ${cfg}"
  log "Existing mode: validating ${cfg} offline"
  # Validate must not abort the runner: historical fixtures may fail offline checks.
  set +e
  "${DNS_CLI}" -v "${CLI_VERBOSE}" config validate --config "${cfg}" --output json >/dev/null
  local vc=$?
  set -e
  if [[ "${vc}" -eq 0 ]]; then
    log "Offline validation OK"
  else
    log "WARNING: offline validation reported errors (historical fixture may share actor addresses)."
    log_ext "config validate exit=${vc}"
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

# =============================================================================
# main — orchestration
# =============================================================================
main() {
  init_logging
  if [[ "${DEMO_ASSUME_YES:-}" =~ ^(1|true|yes|on)$ ]]; then
    ASSUME_YES=1
  fi
  startup_banner
  load_env_file
  prepend_tools_path
  # jq is required to read/write state.json before the rest of the flow.
  ensure_jq
  load_or_create_state

  # Mode-aware tool/credential setup (prompt to install/set, else print guides).
  ensure_prerequisites "${MODE}"

  if [[ "${MODE}" == "existing" ]]; then
    run_existing_mode
    exit 0
  fi

  mkdir -p \
    "${RUNTIME_DIR}/wallets" \
    "${RUNTIME_DIR}/config" \
    "${RUNTIME_DIR}/proofs" \
    "${RUNTIME_DIR}/contracts" \
    "${RUNTIME_DIR}/artifacts" \
    "${TOOLS_DIR}"

  ensure_wallets
  write_bootstrap_config
  wait_for_bootstrap_funds
  ensure_proof_and_prepare
  print_preflight
  confirm_proceed
  run_fresh_submissions
  print_success_summary
  log_ext "main: complete"
}

main "$@"
