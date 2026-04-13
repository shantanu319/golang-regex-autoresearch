#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")/.."

go test -bench=. -benchtime=2s -count=3 ./bench/ 2>&1 | tee run.log

grep -E "^Benchmark" run.log \
	| awk '{print $3}' \
	| awk '{
		n++; sum += log($1)
	} END {
		if (n > 0) printf "geomean_nsop: %.2f\n", exp(sum/n)
		else print "geomean_nsop: ERROR"
	}'

grep -E "^Benchmark" run.log | awk '{printf "%s\t%s\t%s\n", $1, $3, $4}'
