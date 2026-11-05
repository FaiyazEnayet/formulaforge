#!/usr/bin/env bash
# texsmoke.sh -- compile-check the generated .tex for every sheet.
#
# Exports each sheet with the formulaforge CLI and runs pdflatex over
# the result. This is the external half of the catalog audit's
# typeset check (docs/catalog-audit-contract.md); the structural half
# runs in the Go test suite. When pdflatex is not installed the
# script says so and exits 0, so it is safe to run anywhere.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN="${FF_BIN:-$(mktemp -d)/formulaforge}"
if [ ! -x "$BIN" ]; then
  go build -o "$BIN" ./cmd/formulaforge
fi

if ! command -v pdflatex >/dev/null 2>&1; then
  echo "texsmoke: pdflatex not found; skipping compile checks (structural smoke runs in go test)"
  exit 0
fi

OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT
fail=0
for sheet in $("$BIN" sheets); do
  tex="$OUT/$sheet.tex"
  "$BIN" export "$sheet" --format tex --output "$tex" >/dev/null
  if pdflatex -interaction=nonstopmode -halt-on-error -output-directory "$OUT" "$tex" >/dev/null 2>&1; then
    echo "texsmoke: $sheet OK"
  else
    echo "texsmoke: $sheet FAILED to compile (see $sheet.log while debugging)"
    fail=1
  fi
done
exit "$fail"
