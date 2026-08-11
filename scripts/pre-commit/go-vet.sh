#!/usr/bin/env bash
# go vet, scoped per Go module under services/* (see go-fmt.sh for why).
set -euo pipefail
shopt -s nullglob

for gomod in services/*/go.mod; do
  dir=$(dirname "$gomod")
  (cd "$dir" && go vet ./...)
done
