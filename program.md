# autoresearch-regex

You are an autonomous researcher optimizing Go's standard library regexp package.

## Setup

1. Propose a run tag based on today's date (e.g. `apr14`).
2. Create branch: `git checkout -b autoresearch/<tag>` from main.
3. Read ALL files in `regexp-opt/` for full context. Key files:
   - `regexp.go` — main struct, Compile, public API
   - `exec.go` — doExecute dispatcher, NFA match loop
   - `backtrack.go` — bitstate backtracker
   - `onepass.go` — one-pass DFA
   - `syntax/compile.go` — parse tree → program
   - `syntax/prog.go` — instruction representation
4. Read `bench/bench_test.go` to understand what is being measured.
5. Run `bash harness/prepare.sh` to download test data (if not present).
6. Initialize `results.tsv` with header row.
7. If `go test` cannot write to the default build cache, export `GOCACHE=/tmp/autoresearch-go-build`.
8. Confirm setup, then start experimentation.

## Experimentation

### What you CAN do
- Modify ANY file in `regexp-opt/` and `regexp-opt/syntax/`.
- Add new files within these directories.
- Change: matching algorithms, instruction encoding, compile-time
  analysis, literal prefiltering, character class representation,
  NFA/DFA execution strategy, memory layout, constant tuning.

### What you CANNOT do
- Modify `bench/`, `harness/`, or `program.md`.
- Add external dependencies (stdlib only).
- Break the linear-time guarantee. No exponential backtracking.
- Use `unsafe` package or cgo (pure Go only).
- Use assembly or SIMD intrinsics (pure Go only — for now).

### The goal
**Minimize `geomean_nsop`** — the geometric mean of ns/op across all
benchmarks. Lower is better.

### Correctness gate
Before recording ANY result, you MUST run BOTH of these. They catch
different things, and passing only one is not passing the gate.

```bash
# 1. The vendored unit tests.
env GOCACHE="${GOCACHE:-/tmp/autoresearch-go-build}" go test ./regexp-opt/... 2>&1 | tail -5

# 2. Differential check against the standard library.
bash harness/verify.sh 2>&1 | tail -20
```

If either fails, the experiment is invalid. Fix or revert.

**Why both.** The unit tests alone cannot see a large class of bug in
exactly the code these experiments touch. Every input they use is a few
hundred bytes, and `regexp` routes inputs of up to
`maxBacktrackVector/len(prog.Inst)` bytes to the bitstate backtracker —
about 29 KB for a 9-instruction program. The NFA path, which is where
prefilters and skip-scans live, is therefore barely exercised for
correctness by the suite at all.

This is not hypothetical. A start-rune filter once advanced `pos`
without recomputing the empty-width context flag, so every pattern with
a leading `\b`, `\B`, `^` or `$` silently lost matches on large inputs —
`\bSherlock\b` returned 0 of 97 matches on sherlock.txt. `go test
./regexp-opt/...` passed for ten commits while a benchmark measured the
resulting wrong work as a speedup.

`harness/verify.sh` closes that gap: it compares ~50 patterns against
the standard library over inputs from 60 bytes to 512 KB, deliberately
straddling the backtracker cutoff, and checks `FindAllStringIndex`,
`FindAllIndex`, `FindStringSubmatchIndex`, and `MatchString`. It prints
the pattern, the input, and the surrounding text at the first
divergence. It needs no test data and runs in about 12 seconds.

**A faster benchmark that changes observable match behaviour is not an
optimization.** If `verify.sh` fails, the result does not count, no
matter what `geomean_nsop` says.

### Known optimization directions
These are starting points, not an exhaustive list:

1. **Literal prefiltering**: If the regex starts with or contains a
   literal string, scan for that literal first using bytes.Index or
   similar before engaging the NFA. This is the single biggest win
   available — it's what makes rust/regex and hyperscan fast.
2. **DFA caching**: Build a partial DFA from frequently-visited NFA
   states. Cache state transitions. Go's current impl rebuilds NFA
   state sets from scratch on every step.
3. **Prefix/suffix analysis at compile time**: Extract more information
   from the pattern during compilation (required prefixes, minimum
   match length, anchor detection) and use it to skip work at runtime.
4. **Expand one-pass eligibility**: The one-pass engine is fast but
   only triggers for simple patterns. Identify patterns that could be
   made one-pass with minor transformations.
5. **Instruction encoding**: The current instruction representation
   may have cache-unfriendly access patterns. Consider restructuring.
6. **Character class optimization**: Large Unicode character classes
   could use binary search or bitmap lookup instead of linear scan.
7. **Backtracker threshold tuning**: The constants `maxBacktrackProg`
   and `maxBacktrackVector` control when the fast backtracker is used.
   These may not be optimal.

## Running an experiment

```bash
# 1. Make your code changes to files in regexp-opt/

# 2. Commit
git add -A && git commit -m "<description of change>"

# 3. Run correctness tests
env GOCACHE="${GOCACHE:-/tmp/autoresearch-go-build}" go test ./regexp-opt/... > test.log 2>&1
if grep -q "FAIL" test.log; then
    echo "TESTS FAILED — revert or fix"
    tail -20 test.log
    # revert if unfixable
    exit 1
fi

# 4. Differential check against the standard library (BOTH halves of the gate
#    are required — see "Correctness gate" above for why the unit tests alone
#    cannot see prefilter bugs on the NFA path). ~12 seconds.
if ! bash harness/verify.sh > verify.log 2>&1; then
    echo "VERIFY FAILED — regexp-opt disagrees with stdlib; revert or fix"
    tail -20 verify.log
    exit 1
fi

# 5. Run benchmark
bash harness/score.sh > run.log 2>&1

# 6. The benchmark harness can take around 250 seconds with the full mixed-content haystack.
#    Wait at least that long before assuming it is stuck. Sleeping is acceptable.

# 7. Extract metric
grep "^geomean_nsop:" run.log

# 8. If individual benchmarks are needed:
grep "^Benchmark" run.log
```

## Logging results

Log every experiment to `results.tsv` (tab-separated):

```text
commit	geomean_nsop	status	description
```

- commit: short git hash (7 chars)
- geomean_nsop: geometric mean ns/op (e.g. 12345.67), 0 for crashes
- status: `keep`, `discard`, `crash`, or `wrong` (failed the
  differential gate — changed observable match behaviour)
- description: what this experiment tried

Example:
```text
commit	geomean_nsop	status	description
a1b2c3d	45230.50	keep	baseline — unmodified go/regexp
b2c3d4e	38102.30	keep	add literal prefix scan via bytes.Index
c3d4e5f	39500.10	discard	switch char class to bitmap (no improvement)
d4e5f6g	0.00	crash	DFA cache caused infinite loop
```

## The loop

LOOP FOREVER:

1. Review `results.tsv` and the current state of the code.
2. Choose an experiment. Prioritize:
   - Ideas with high expected impact (literal prefiltering > constant tuning)
   - Ideas that build on previous successful changes
   - Diverse exploration (don't keep tweaking the same thing)
3. Edit files in `regexp-opt/`.
4. `git add -A && git commit -m "<description>"`
5. Run tests: `env GOCACHE="${GOCACHE:-/tmp/autoresearch-go-build}" go test ./regexp-opt/... > test.log 2>&1`
   - If FAIL: attempt fix (max 3 tries), else revert and log as `crash`
6. Run the differential gate: `bash harness/verify.sh > verify.log 2>&1`
   - If FAIL: the change alters observable match behaviour. Attempt fix
     (max 3 tries), else revert and log as `wrong`. Do NOT benchmark it —
     a speedup measured from dropped matches is not a result.
7. Run benchmark: `bash harness/score.sh > run.log 2>&1`
8. Wait 240 seconds before treating a long-running benchmark as hung.
9. Extract: `grep "^geomean_nsop:" run.log`
   - If empty: run crashed. `tail -50 run.log`, attempt fix or revert.
10. Compare to best known geomean_nsop.
    - If improved: `keep`. Branch advances.
    - If equal or worse: `discard`. `git reset --hard HEAD~1`
11. Log result to `results.tsv`.
12. Go to 1.

## Simplicity criterion

All else being equal, simpler code is better. A tiny improvement that
adds 50 lines of complexity is questionable. A tiny improvement from
REMOVING code is excellent. Weigh complexity cost against improvement
magnitude.
