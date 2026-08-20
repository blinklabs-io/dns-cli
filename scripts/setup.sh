#!/usr/bin/env bash
# =============================================================================
# setup.sh — Bootstrap dns-cli (requirements + bin/ + build)
# =============================================================================
# Repo-root helper for any dns-cli use. Does not scaffold demo/ (that is handled
# by `dns-cli demo run` via EnsureDemoLayout).
# From repo root: ./scripts/setup.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${ROOT}/bin"
BIN_OUT="${BIN_DIR}/dns-cli"
MIN_GO="1.25.13"

YES=0
SKIP_BUILD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--yes) YES=1; shift ;;
    --skip-build) SKIP_BUILD=1; shift ;;
    -h|--help)
      cat <<'EOF'
Usage: setup.sh [-y|--yes] [--skip-build]

Checks Go (>= 1.25.13), creates bin/, builds dns-cli.
Does not prepare demo/ — use: bin/dns-cli demo run
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
  if [[ "${ASSUME_YES:-}" =~ ^(1|true|yes|on)$ ]]; then
    return 0
  fi
  if [[ "${DEMO_ASSUME_YES:-}" =~ ^(1|true|yes|on)$ ]]; then
    return 0
  fi
  return 1
}

version_ge() {
  # return 0 if $1 >= $2 (semver-ish major.minor.patch)
  printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1 | grep -qx "$2"
}

go_exe_version() {
  local exe="$1"
  local out
  out="$("${exe}" version 2>&1 || true)"
  sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\(\.[0-9][0-9]*\)*\).*/\1/p' <<<"${out}" | head -n1
}

resolve_go_exe() {
  # Prefer a local go-toolchains / repo .tools install over an older system Go
  # so go.mod's toolchain pin does not trigger a download that can fail.
  local candidates=()
  if [[ -n "${LOCALAPPDATA:-}" ]]; then
    candidates+=("${LOCALAPPDATA}/go-toolchains/go/bin/go")
    candidates+=("${LOCALAPPDATA}/go-toolchains/go/bin/go.exe")
  fi
  candidates+=("${ROOT}/../.tools/go/bin/go")
  candidates+=("${ROOT}/../.tools/go/bin/go.exe")
  if command -v go >/dev/null 2>&1; then
    candidates+=("$(command -v go)")
  fi

  local exe ver
  for exe in "${candidates[@]}"; do
    [[ -n "${exe}" && -x "${exe}" ]] || continue
    ver="$(go_exe_version "${exe}")"
    [[ -n "${ver}" ]] || continue
    if ! version_ge "${ver}" "${MIN_GO}"; then
      echo "Skipping ${exe} (Go ${ver} < ${MIN_GO})"
      continue
    fi
    printf '%s\n' "${exe}"
    return 0
  done
  return 1
}

echo "dns-cli setup (root: ${ROOT})"

if ! GO_EXE="$(resolve_go_exe)"; then
  cat <<EOF >&2
Go >= ${MIN_GO} not found.

Install: https://go.dev/dl/
Then re-run: ./scripts/setup.sh
EOF
  exit 1
fi

GO_VER="$(go_exe_version "${GO_EXE}")"
# Pin GOTOOLCHAIN=local so an already-installed Go matching go.mod does not
# attempt a toolchain download (which fails when the proxy blocks it).
export PATH="$(dirname "${GO_EXE}"):${PATH}"
export GOTOOLCHAIN=local
echo "Go ${GO_VER} OK (${GO_EXE})"

if command -v aiken >/dev/null 2>&1; then
  echo "aiken found: $(command -v aiken)"
else
  echo "Note: aiken not on PATH (needed for system prepare / demo fresh). See https://aiken-lang.org/installation-instructions"
fi

mkdir -p "${BIN_DIR}"
echo "bin/ ready: ${BIN_DIR}"

if [[ "${SKIP_BUILD}" -eq 1 ]]; then
  echo "SkipBuild set; not compiling."
  exit 0
fi

echo "Building -> ${BIN_OUT}"
COMMIT="$(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
CONTRACTS="unknown"
if [[ -d "${ROOT}/../dns-contracts" ]]; then
  CONTRACTS="$(git -C "${ROOT}/../dns-contracts" rev-parse --short HEAD 2>/dev/null || echo unknown)"
fi
if [[ "${CONTRACTS}" == "unknown" || -z "${CONTRACTS}" ]]; then
  CONTRACTS="$(git -C "${ROOT}" log -1 --format=%h -- demo/fixtures/contracts 2>/dev/null || echo unknown)"
fi
PKG="github.com/blinklabs-io/dns-cli/internal/cli"
LDFLAGS="-X ${PKG}.GitCommit=${COMMIT} -X ${PKG}.BuildDate=${BUILT} -X ${PKG}.ContractRevision=${CONTRACTS}"
echo "ldflags: commit=${COMMIT} built=${BUILT} contracts=${CONTRACTS}"
(cd "${ROOT}" && "${GO_EXE}" build -ldflags "${LDFLAGS}" -o "${BIN_OUT}" ./cmd/dns-cli)

"${BIN_OUT}" version

cat <<EOF

Next:
  ${BIN_OUT} version
  ${BIN_OUT} dashboard --config dns-cli.json
  ${BIN_OUT} demo run
EOF
