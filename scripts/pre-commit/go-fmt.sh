#!/usr/bin/env bash
# gofmt, scoped per Go module under services/*, since this repo has one Go
# module per service rather than a single module at the repo root (the
# upstream dnephin/pre-commit-golang hooks assume the latter and don't fit).
set -euo pipefail
shopt -s nullglob

failed=0
for gomod in services/*/go.mod; do
  dir=$(dirname "$gomod")
  unformatted=$(gofmt -l -w "$dir")
  if [[ -n "$unformatted" ]]; then
    echo "$unformatted"
    failed=1
  fi
done
exit "$failed"
