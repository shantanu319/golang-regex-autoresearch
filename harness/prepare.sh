#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")/.."

mixed_copies=4000
min_mixed_size=2000000000

mkdir -p bench/testdata

if [[ ! -f bench/testdata/sherlock.txt ]]; then
	curl -L --fail -o bench/testdata/sherlock.txt \
		"https://raw.githubusercontent.com/BurntSushi/rebar/master/benchmarks/haystacks/sherlock.txt"
fi

rebuild_mixed=false
if [[ ! -f bench/testdata/mixed-content.txt ]]; then
	rebuild_mixed=true
else
	mixed_size=$(wc -c < bench/testdata/mixed-content.txt)
	if (( mixed_size < min_mixed_size )); then
		rebuild_mixed=true
	fi
fi

if [[ "$rebuild_mixed" == true ]]; then
	cat > bench/testdata/mixed-content.txt <<'EOF'
Sherlock Holmes met watson@example.com near Baker Street.
Contact Inspector Lestrade at lestrade@yard.gov or Mrs. Hudson at hudson221b@home.uk.
Moriarty used aliases like arturo@naples.it and napier@oxford.edu in intercepted letters.
Unicode fragments: cafe naive jalapeno facade resume cooperate smorgasbord.
Noise: ZXCVB12345 not-an-email another+tag@example.com ignored_by_regex.
EOF
	for i in $(seq 1 "$mixed_copies"); do
		cat bench/testdata/sherlock.txt >> bench/testdata/mixed-content.txt
		printf '\nname%04d@example.com Lestrade Holmes Watson Moriarty BakerStreet\n' "$i" >> bench/testdata/mixed-content.txt
	done
fi
