#!/usr/bin/env bash
set -euo pipefail

CLI="${CLI:-./dns-cli}"
CONFIG="${CONFIG:-dns-cli.json}"
TLD="${TLD:-demo-$(date +%s)}"
PROOF="${PROOF:-examples/proof-bundle.json}"
OUT="${OUT:-artifacts/demo}"

"$CLI" config validate --config "$CONFIG"
"$CLI" registrar register-tld --config "$CONFIG" --tld "$TLD" --proof "$PROOF" --out "$OUT-register"
echo "Built: $OUT-register.unsigned.json (sign and submit before continuing)"
