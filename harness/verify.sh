#!/bin/bash
#
# Differential correctness gate. Compares regexp-opt against the standard
# library's regexp over a corpus of patterns, including inputs large enough to
# leave the bitstate backtracker and run on the NFA. Exits non-zero on any
# disagreement.
#
#   bash harness/verify.sh

set -euo pipefail

cd "$(dirname "$0")/.."

export GOCACHE="${GOCACHE:-/tmp/autoresearch-go-build}"

go run ./harness/verify "$@"
