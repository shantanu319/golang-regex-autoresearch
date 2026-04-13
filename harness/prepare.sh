#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p bench/testdata

if [[ ! -f bench/testdata/sherlock.txt ]]; then
	curl -L --fail -o bench/testdata/sherlock.txt \
		"https://raw.githubusercontent.com/BurntSushi/rebar/master/benchmarks/haystacks/sherlock.txt"
fi

if [[ ! -f bench/testdata/mixed-content.txt ]]; then
	cat > bench/testdata/mixed-content.txt <<'EOF'
Sherlock Holmes met watson@example.com near Baker Street.
Contact Inspector Lestrade at lestrade@yard.gov or Mrs. Hudson at hudson221b@home.uk.
Moriarty used aliases like arturo@naples.it and napier@oxford.edu in intercepted letters.
Unicode fragments: cafe naive jalapeno facade resume cooperate smorgasbord.
Noise: ZXCVB12345 not-an-email another+tag@example.com ignored_by_regex.
EOF
	for i in $(seq 1 4000); do
		cat bench/testdata/sherlock.txt >> bench/testdata/mixed-content.txt
		printf '\nname%04d@example.com Lestrade Holmes Watson Moriarty BakerStreet\n' "$i" >> bench/testdata/mixed-content.txt
	done
fi
