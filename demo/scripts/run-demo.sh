#!/usr/bin/env bash
# =============================================================================
# run-demo.sh — Thin wrapper around `dns-cli demo run`
# =============================================================================
# Resolves dns-cli, interactively fills unset flags, then execs demo run.
# From demo/: ./scripts/run-demo.sh [flags]
#
# Binary resolution: $CLI → dns-cli/bin/ → tree root → PATH.
# Missing/outdated builds ask before compiling into dns-cli/bin/.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CLI_ROOT="$(cd "${DEMO_ROOT}/.." && pwd)"
BIN_DIR="${CLI_ROOT}/bin"
BIN_OUT="${BIN_DIR}/dns-cli"

MODE=""
PROVIDER=""
TLD=""
SLD=""
LOG_LEVEL=""
YES=0
SKIP_INSTALL=""
NO_CLIPBOARD=""
VERBOSE=2
MODE_SET=0
PROVIDER_SET=0
TLD_SET=0
SLD_SET=0
LOG_SET=0
SKIP_SET=0
CLIP_SET=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; MODE_SET=1; shift 2 ;;
    --provider) PROVIDER="$2"; PROVIDER_SET=1; shift 2 ;;
    --tld) TLD="$2"; TLD_SET=1; shift 2 ;;
    --sld) SLD="$2"; SLD_SET=1; shift 2 ;;
    --log-level) LOG_LEVEL="$2"; LOG_SET=1; shift 2 ;;
    -E|--extensive-logging|-v|--verbose) LOG_LEVEL="extensive"; LOG_SET=1; shift ;;
    -y|--yes) YES=1; shift ;;
    --skip-install) SKIP_INSTALL=1; SKIP_SET=1; shift ;;
    --no-clipboard) NO_CLIPBOARD=1; CLIP_SET=1; shift ;;
    -h|--help)
      cat <<'EOF'
Usage: run-demo.sh [--mode fresh|existing] [--provider blockfrost|utxorpc]
                   [--tld NAME] [--sld NAME] [--log-level quiet|normal|extensive]
                   [-y|--yes] [--skip-install] [--no-clipboard]

When flags are omitted, the script asks interactively (unless -y / DEMO_ASSUME_YES).
Missing/outdated dns-cli: asks to build into dns-cli/bin/.
EOF
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

assume_yes() {
  if [[ "${YES}" -eq 1 ]]; then
    return 0
  fi
  if [[ "${DEMO_ASSUME_YES:-}" =~ ^(1|true|yes|on)$ ]]; then
    return 0
  fi
  return 1
}

read_choice() {
  local prompt="$1"
  local default="$2"
  shift 2
  local allowed=("$@")
  if assume_yes; then
    echo "${prompt} [${default}]: ${default} (assume-yes)" >&2
    printf '%s' "${default}"
    return
  fi

  # Free-text when no fixed options.
  if [[ ${#allowed[@]} -eq 0 ]]; then
    local ans=""
    printf '%s [%s]: ' "${prompt}" "${default}" >&2
    read -r ans || true
    if [[ -z "${ans}" ]]; then
      printf '%s' "${default}"
    else
      printf '%s' "${ans}"
    fi
    return
  fi

  local i default_index=1
  for i in "${!allowed[@]}"; do
    if [[ "$(echo "${allowed[$i]}" | tr '[:upper:]' '[:lower:]')" == "$(echo "${default}" | tr '[:upper:]' '[:lower:]')" ]]; then
      default_index=$((i + 1))
      break
    fi
  done

  echo "${prompt}" >&2
  for i in "${!allowed[@]}"; do
    local n=$((i + 1))
    local mark=""
    [[ "${n}" -eq "${default_index}" ]] && mark=" (default)"
    echo "  ${n}) ${allowed[$i]}${mark}" >&2
  done

  local ans=""
  printf 'Enter number [%s]: ' "${default_index}" >&2
  read -r ans || true
  if [[ -z "${ans}" ]]; then
    printf '%s' "${allowed[$((default_index - 1))]}"
    return
  fi
  if [[ "${ans}" =~ ^[0-9]+$ ]]; then
    local n="${ans}"
    if [[ "${n}" -ge 1 && "${n}" -le ${#allowed[@]} ]]; then
      printf '%s' "${allowed[$((n - 1))]}"
      return
    fi
  fi
  # Also accept the option name if typed in full.
  for i in "${!allowed[@]}"; do
    if [[ "$(echo "${ans}" | tr '[:upper:]' '[:lower:]')" == "$(echo "${allowed[$i]}" | tr '[:upper:]' '[:lower:]')" ]]; then
      printf '%s' "${allowed[$i]}"
      return
    fi
  done
  echo "Invalid choice '${ans}'; keeping default '${allowed[$((default_index - 1))]}'." >&2
  printf '%s' "${allowed[$((default_index - 1))]}"
}

read_yes_no() {
  local prompt="$1"
  local default_yes="$2" # 1 or 0
  if assume_yes; then
    local pick=N
    [[ "${default_yes}" -eq 1 ]] && pick=Y
    echo "${prompt}: ${pick} (assume-yes)" >&2
    return $((1 - default_yes)) # return 0 if yes
  fi
  local hint='y/N'
  [[ "${default_yes}" -eq 1 ]] && hint='Y/n'
  local ans=""
  printf '%s [%s]: ' "${prompt}" "${hint}" >&2
  read -r ans || true
  if [[ -z "${ans}" ]]; then
    [[ "${default_yes}" -eq 1 ]]
    return
  fi
  case "$(echo "${ans}" | tr '[:upper:]' '[:lower:]')" in
    y|yes) return 0 ;;
    *) return 1 ;;
  esac
}

dns_cli_has_demo() {
  local path="$1"
  "${path}" demo --help >/dev/null 2>&1
}

show_dns_cli_build_guide() {
  cat <<EOF >&2
Build dns-cli yourself, then re-run this script:

  cd ${CLI_ROOT}
  mkdir -p bin
  go build -o bin/dns-cli ./cmd/dns-cli

Or set CLI to an existing binary that includes 'demo'.
EOF
}

confirm_build_dns_cli() {
  local reason="${1:-missing or outdated}"
  if [[ "${SKIP_INSTALL:-0}" == "1" ]]; then
    echo "dns-cli is ${reason} and --skip-install was set; not building." >&2
    show_dns_cli_build_guide
    return 1
  fi
  if assume_yes; then
    return 0
  fi
  printf "dns-cli is %s. Build into bin/ now? [y/N] " "${reason}" >&2
  local ans=""
  read -r ans || true
  case "$(echo "${ans}" | tr '[:upper:]' '[:lower:]')" in
    y|yes) return 0 ;;
  esac
  show_dns_cli_build_guide
  return 1
}

build_dns_cli() {
  if ! command -v go >/dev/null 2>&1; then
    echo "go is not on PATH; install Go 1.25.10+ or build dns-cli manually" >&2
    exit 1
  fi
  mkdir -p "${BIN_DIR}"
  echo "Building dns-cli → ${BIN_OUT}" >&2
  (cd "${CLI_ROOT}" && go build -o "${BIN_OUT}" ./cmd/dns-cli)
  if ! dns_cli_has_demo "${BIN_OUT}"; then
    echo "built ${BIN_OUT} but 'demo' is still missing" >&2
    exit 1
  fi
  echo "Built ${BIN_OUT}" >&2
  echo "${BIN_OUT}"
}

resolve_dns_cli() {
  if [[ -n "${CLI:-}" && -x "${CLI}" ]]; then
    if ! dns_cli_has_demo "${CLI}"; then
      echo "CLI at \$CLI lacks 'demo'. Rebuild dns-cli or unset CLI." >&2
      exit 1
    fi
    echo "${CLI}"
    return
  fi

  local cand
  for cand in "${BIN_OUT}" "${BIN_DIR}/dns-cli.exe" "${CLI_ROOT}/dns-cli" "${CLI_ROOT}/dns-cli.exe"; do
    if [[ -x "${cand}" ]]; then
      if dns_cli_has_demo "${cand}"; then
        echo "${cand}"
        return
      fi
      if ! confirm_build_dns_cli "outdated (missing demo)"; then
        exit 1
      fi
      build_dns_cli
      return
    fi
  done

  if command -v dns-cli >/dev/null 2>&1; then
    local path
    path="$(command -v dns-cli)"
    if dns_cli_has_demo "${path}"; then
      echo "${path}"
      return
    fi
  fi

  if ! confirm_build_dns_cli "not found"; then
    exit 1
  fi
  build_dns_cli
}

prompt_demo_flags() {
  if [[ "${MODE_SET}" -eq 0 && -n "${DEMO_MODE:-}" ]]; then
    MODE="${DEMO_MODE}"
    MODE_SET=1
  fi
  if [[ "${PROVIDER_SET}" -eq 0 && -n "${DEMO_PROVIDER:-}" ]]; then
    PROVIDER="${DEMO_PROVIDER}"
    PROVIDER_SET=1
  fi
  if [[ "${LOG_SET}" -eq 0 && "${DEMO_EXTENSIVE_LOGGING:-}" =~ ^(1|true|yes|on)$ ]]; then
    LOG_LEVEL=extensive
    LOG_SET=1
  fi
  if [[ "${LOG_SET}" -eq 0 && -n "${DEMO_LOG_LEVEL:-}" ]]; then
    LOG_LEVEL="${DEMO_LOG_LEVEL}"
    LOG_SET=1
  fi

  local need_ask=0
  [[ "${MODE_SET}" -eq 0 || "${LOG_SET}" -eq 0 || "${SKIP_SET}" -eq 0 || "${CLIP_SET}" -eq 0 ]] && need_ask=1

  if [[ "${need_ask}" -eq 1 ]] && ! assume_yes; then
    echo "" >&2
    if [[ -t 2 && -z "${NO_COLOR:-}" ]]; then
      printf '\033[36m══ Demo run options ══\033[0m\n' >&2
    else
      echo "══ Demo run options ══" >&2
    fi
  fi

  if [[ "${MODE_SET}" -eq 0 ]]; then
    MODE="$(read_choice 'Mode' 'fresh' fresh existing)"
    MODE_SET=1
  fi

  if [[ "${MODE}" != "existing" ]]; then
    if [[ "${PROVIDER_SET}" -eq 0 ]]; then
      PROVIDER="$(read_choice 'Provider' 'blockfrost' blockfrost utxorpc)"
      PROVIDER_SET=1
    fi
    if [[ "${TLD_SET}" -eq 0 ]]; then
      TLD="$(read_choice 'TLD (blank = auto demo-<timestamp>)' '')"
      TLD_SET=1
    fi
    if [[ "${SLD_SET}" -eq 0 ]]; then
      SLD="$(read_choice 'SLD' 'www')"
      SLD_SET=1
    fi
  fi

  if [[ "${LOG_SET}" -eq 0 ]]; then
    LOG_LEVEL="$(read_choice 'Log level' 'normal' quiet normal extensive)"
    LOG_SET=1
  fi

  if [[ "${SKIP_SET}" -eq 0 ]]; then
    if read_yes_no 'Skip tool installs / credential writes (guides only)?' 0; then
      SKIP_INSTALL=1
    else
      SKIP_INSTALL=0
    fi
    SKIP_SET=1
  fi

  if [[ "${CLIP_SET}" -eq 0 && "${MODE}" != "existing" ]]; then
    if read_yes_no 'Copy bootstrap faucet address to clipboard?' 1; then
      NO_CLIPBOARD=0
    else
      NO_CLIPBOARD=1
    fi
    CLIP_SET=1
  fi

  if [[ "${need_ask}" -eq 1 ]] && ! assume_yes; then
    if [[ -t 2 && -z "${NO_COLOR:-}" ]]; then
      printf '\033[36m════════════════════════\033[0m\n' >&2
    else
      echo "════════════════════════" >&2
    fi
    echo "" >&2
  fi
}

prompt_demo_flags
DNS_CLI="$(resolve_dns_cli)"

case "$(echo "${LOG_LEVEL:-normal}" | tr '[:upper:]' '[:lower:]')" in
  quiet|q|0) VERBOSE=1 ;;
  extensive|debug|verbose|v|ext|4) VERBOSE=4 ;;
  *) VERBOSE=2; LOG_LEVEL=normal ;;
esac

ARGS=(-v "${VERBOSE}" demo run --demo-root "${DEMO_ROOT}")
[[ -n "${MODE}" ]] && ARGS+=(--mode "${MODE}")
if [[ "${MODE}" != "existing" && -n "${PROVIDER}" ]]; then
  ARGS+=(--provider "${PROVIDER}")
fi
[[ -n "${TLD}" ]] && ARGS+=(--tld "${TLD}")
[[ -n "${SLD}" ]] && ARGS+=(--sld "${SLD}")
ARGS+=(--log-level "$(echo "${LOG_LEVEL}" | tr '[:upper:]' '[:lower:]')")
if assume_yes; then
  ARGS+=(--yes)
fi
[[ "${SKIP_INSTALL}" == "1" ]] && ARGS+=(--skip-install)
[[ "${NO_CLIPBOARD}" == "1" ]] && ARGS+=(--no-clipboard)

echo "dns-cli ${ARGS[*]}"
exec "${DNS_CLI}" "${ARGS[@]}"
