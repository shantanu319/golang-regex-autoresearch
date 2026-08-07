# autoresearch-regex

An autonomous-research harness in which an LLM agent tries to make Go's standard
library `regexp` package faster, scored by a fixed benchmark suite.

The agent is given a vendored copy of `regexp` at `regexp-opt/`, a frozen
benchmark suite it is not allowed to touch, and a single number to minimize:
the geometric mean of ns/op across all benchmarks. It then runs a closed loop —
edit, test, benchmark, keep or revert — recording each experiment as a commit.
`program.md` is the agent's operating manual.

Current state: **7.16× faster** than stock `go/regexp` on the suite's geomean,
across 10 accepted experiments. One of the nine benchmarks is invalidated by a
correctness bug the harness did not catch — see
[Correctness](#correctness-one-benchmark-is-invalid), which is important for
reading the numbers below.

---

## Results so far

`results.tsv`, `run.log`, and `test.log` are all in `.gitignore`, so the
agent's original measurements were never committed — the only surviving record
of the run is the commit messages. **The numbers below were regenerated** by
checking out each commit in turn and re-running the harness on one machine, back
to back, so they are internally comparable even though they are not the agent's
original readings.

### Geomean progression

`harness/score.sh` reports the geometric mean of ns/op over all 27 samples
(9 benchmarks × `-count=3`). Lower is better.

| # | Commit | geomean ns/op | vs. previous | vs. baseline | Experiment |
|---|--------|--------------:|-------------:|-------------:|------------|
| 0 | `9f13eb0` | 3,645,377 | — | 1.00× | baseline — unmodified `go/regexp` |
| 1 | `d428c38` | 2,688,571 | +35.6% | 1.36× | complete-literal fast path via `strings.Index`/`bytes.Index` |
| 2 | `60cc9ec` | 1,562,434 | +72.1% | 2.33× | start-rune filter (NFA + backtracker) |
| 3 | `77ac2be` | 1,161,106 | +34.6% | 3.14× | inner-literal prefilter, bounded offset + caching |
| 4 | `210f1df` | 1,169,537 | −0.7% | 3.12× | pre-check `startCond` before adding start state |
| 5 | `34da8aa` | 1,028,423 | +13.7% | 3.54× | byte-level start-filter scan for string inputs |
| 6 | `1a263ac` | 1,004,028 | +2.4% | 3.63× | skip redundant start-state add when `firstRunePC` queued |
| 7 | `d4a00ac` | 1,064,095 | −5.6% | 3.43× | fast ASCII byte decode for `r1` prefetch |
| 8 | `24959aa` | 1,015,709 | +4.8% | 3.59× | avoid interface dispatch after byte filter finds ASCII |
| 9 | `64ccec5` | 961,950 | +5.6% | 3.79× | inline `MatchRune` for 1/2/4-pair char classes |
| 10 | `589b7f3` | **509,323** | +88.9% | **7.16×** | backward walk from inner literal over start-filter run |

Commit `7ba8a08` is omitted: it edited `program.md` and `harness/prepare.sh`
only, with no change to `regexp-opt/`.

Two commits (`210f1df`, `d4a00ac`) came out **slower** than their predecessor on
re-measurement, by 0.7% and 5.6%. The agent's loop kept them, so they were
faster in its own run. Both are micro-optimizations well inside run-to-run
variance; the honest reading is that neither is a measurable win.

Four commits do most of the work — `d428c38`, `60cc9ec`, `77ac2be`, and
`589b7f3` compound to 5.93× on their own. The six micro-optimizations between
them contribute 1.21× combined.

### Per-benchmark, baseline vs. current

Median of 3 runs, `-benchtime=2s`.

| Benchmark | Pattern | Baseline ns/op | Current ns/op | Speedup |
|-----------|---------|---------------:|--------------:|--------:|
| CharacterClass | `[a-zA-Z]+@[a-zA-Z]+\.[a-zA-Z]+` | 104,690,394,060 | 259,974,570 | **402.69×** |
| AlternationMedium | `Sherlock\|Watson\|Holmes\|Moriarty\|Lestrade` | 52,064,309 | 2,313,129 | 22.51× |
| BoundedRepeat | `[a-z]{2,4}ing` | 37,971,835 | 2,441,816 | 15.55× |
| LiteralCaseInsensitive | `(?i)sherlock holmes` | 23,287,793 | 3,819,999 | 6.10× |
| LiteralMatch | `Sherlock Holmes` | 97,233 | 41,561 | 2.34× |
| NonMatch | `ZZZZZNOTFOUND` | 19,250 | 19,062 | 1.01× |
| UnicodeWordBoundary ⚠️ | `\b\w{4,}\b` | 49,383,481 | 30,809,224 | 1.60× (invalid) |
| CompileSimple † | compile `[a-z]+` | 5,208 | 1,349 | not meaningful |
| CompileComplex † | compile IPv4 pattern | 35,626 | 10,952 | not meaningful |

**CharacterClass dominates.** It is the only benchmark running against the
2.4 GB haystack, and stock `go/regexp` takes **105 seconds per iteration** on it.
Nearly all of that win arrives in the very last commit — the benchmark sat
between 86 s and 105 s for the first ten commits and dropped to 0.26 s at
`589b7f3`. That single change is why the geomean halves at the end.

† The two compile benchmarks should be read as *no signal*. They are
microsecond-scale and run in the same process as benchmarks holding a multi-GB
live heap, and their run-to-run spread reaches 3.45× (CompileSimple) and 3.64×
(CompileComplex) — versus ~1.03× for the matching benchmarks. Every change here
*adds* compile-time analysis (start filter, inner literal, first-rune PC), so a
genuine 3–4× compile speedup is not a plausible reading of these numbers.

---

## Correctness: one benchmark is invalid

`BenchmarkUnicodeWordBoundary` measures `\b\w{4,}\b`, and the optimized package
**silently returns fewer matches than it should** for that pattern.

On `sherlock.txt`, checked against the Go standard library as ground truth:

```
stdlib    57,367 matches
regexp-opt 53,008 matches     — 4,359 missing (7.6%)
```

The other six matching patterns in the suite agree with stdlib **exactly**, on
both `sherlock.txt` and a 200 MB slice of `mixed-content.txt`. So the bug is
confined to zero-width assertions, and the rest of the speedup stands.

**Introduced by** `60cc9ec` (the start-rune filter). `9f13eb0` and `d428c38` are
correct; every commit from `60cc9ec` onward drops the same 4,359 matches.

**Why the test suite passes.** The divergence only appears on inputs larger than
**29,127 bytes**. That is the bitstate-backtracker cutoff for this pattern
(9 instructions × 29,127 ≈ 262,144 = `maxBacktrackVector`). Below it `regexp`
uses the backtracker, whose filter advances one position at a time and is
correct. Above it the NFA path runs, and only that path is broken. Every input
in `regexp-opt`'s vendored test suite is far below the cutoff, so `go test
./regexp-opt/...` passes clean while the benchmark silently measures wrong work.

**Root cause.** In `machine.match` (`regexp-opt/exec.go`), the start-rune filter
and inner-literal prefilter jump `pos` forward and refresh `r`, `width`, `r1`,
and `width1` — but never recompute `flag`, the lazily-evaluated empty-width
context. The subsequent `m.add` therefore evaluates `\b` against the context of
wherever the scan *started*, not where it landed. Matches whose start is
preceded by two or more non-word bytes (`", "`, `"\r\n"`, a BOM) are the ones
lost.

Confirmed by patching a context recompute in before the add:

```go
flag = i.context(pos)   // recompute stale context after a filter skip
if !m.matched && (startCond == 0 || flag.match(startCond)) {
```

With that line, `\b\w{4,}\b`, `\bSherlock\b`, and `\b\w+ing\b` all return
exactly the stdlib counts. **The fix is not applied on this branch** — it costs
performance on every iteration and would invalidate the table above, so it needs
its own benchmark round.

Excluding the broken benchmark, the suite geomean goes 2,637,348 → 305,163 ns/op,
an **8.64×** speedup over baseline.

Two things this suggests about the harness itself: the correctness gate should
compare match *results* against stdlib rather than only running the vendored unit
tests, and it should exercise inputs above the backtracker cutoff.

---

## What the optimizations actually do

All ten changes are variations on one theme: **avoid entering the NFA at
positions that cannot possibly start a match.** Stock `go/regexp` already does
this for a required literal prefix; the agent extended it in several directions.

1. **Complete-literal fast path** (`d428c38`) — when the pattern is nothing but
   a literal and no submatches are needed, `doExecute` skips the engine entirely
   and calls `strings.Index`/`bytes.Index`.

2. **Start-rune filter** (`60cc9ec`) — at compile time, walk the syntax tree
   (`collectStartRunes`, `canMatchEmpty`) to build a bitmap of runes that can
   begin a match: `[4]uint32` for ASCII plus a "non-ASCII possible" flag.
   At run time, skip any position whose rune isn't in the set. Applied to both
   the NFA loop and the backtracker. *This is the change that introduced the
   word-boundary bug.*

3. **Inner-literal prefilter** (`77ac2be`) — many patterns have no usable prefix
   but do contain a mandatory literal (`@` in the email pattern).
   `findInnerLiteral` picks the longest non-folded literal in a concatenation,
   and `maxByteLen` bounds how far the match start can be from it. `strings.Index`
   then jumps straight there, with the found position cached so the scan isn't
   repeated.

4. **Backward run walk** (`589b7f3`) — when the inner literal's offset is
   *unbounded* (as in `[a-zA-Z]+@...`, where `[a-zA-Z]+` can be any length), step
   3 can't compute a skip target. This walks backward from the literal across
   bytes that pass the start-rune filter to find the start of the run containing
   it. This is what turns CharacterClass from 105 s into 0.26 s.

5. **Dispatch and decode micro-optimizations** (`210f1df`, `34da8aa`, `1a263ac`,
   `d4a00ac`, `24959aa`, `64ccec5`) — scan raw bytes instead of going through the
   `input` interface, decode ASCII inline rather than via `utf8.DecodeRuneInString`,
   skip a start-state add when the first rune PC is already queued, and inline
   `MatchRune` for character classes of 1, 2, or 4 rune-pairs. Together ~1.09×.

---

## Layout

```
program.md            the agent's operating manual — setup, rules, the loop
regexp-opt/           vendored go/regexp + regexp/syntax; the only mutable code
  exec.go             doExecute dispatcher and the NFA match loop
  backtrack.go        bitstate backtracker
  onepass.go          one-pass DFA
  regexp.go           Regexp struct, Compile, public API, compile-time analysis
  syntax/             parser, simplifier, compiler
bench/bench_test.go   9 benchmarks — frozen, agent may not edit
harness/prepare.sh    fetches sherlock.txt, builds the ~2.4 GB mixed haystack
harness/score.sh      runs the suite, prints geomean_nsop
```

Gitignored: `results.tsv`, `run.log`, `test.log`, `bench/testdata/*.txt`.

## Running it

**Go 1.25 or newer is required.** The vendored `regexp-opt/syntax/parse.go` uses
`unicode.CategoryAliases` and `unicode.Cn`, which do not exist before 1.25 — on
1.24 the package fails to build. Note that `go.mod` still declares `go 1.22`,
which understates the real requirement.

```bash
bash harness/prepare.sh          # downloads sherlock.txt, builds ~2.4 GB haystack
export GOCACHE=/tmp/autoresearch-go-build

go test ./regexp-opt/...         # correctness gate — must pass
bash harness/score.sh            # full suite, prints geomean_nsop
```

`prepare.sh` needs ~2.4 GB of disk. A full `score.sh` run takes roughly 3–8
minutes depending on how fast the current code is — the stock-`regexp` baseline
is the slow end, because CharacterClass alone costs 105 s per iteration.

## Rules the agent works under

**May**: change anything in `regexp-opt/` and `regexp-opt/syntax/`, add files
there, alter matching algorithms, instruction encoding, compile-time analysis,
prefiltering, character-class representation, memory layout, constants.

**May not**: touch `bench/`, `harness/`, or `program.md`; add dependencies
(stdlib only); use `unsafe`, cgo, assembly, or SIMD; break the linear-time
guarantee.

**Gate**: `go test ./regexp-opt/...` must pass before any result is recorded.

## Measurement notes

Numbers above were produced on a single machine, one commit at a time with no
concurrent load, in an isolated `git worktree` so the checkouts never touched
the main tree:

- Intel Xeon @ 2.80 GHz, 4 vCPU, 16 GB RAM, Linux 6.18
- Go 1.25.1 linux/amd64
- `harness/score.sh` defaults: `-benchtime=2s -count=3`
- `harness/` and `bench/` pinned at `589b7f3` for every run, so only the package
  under test varies

Caveats worth keeping in mind: three samples per benchmark is thin, the compile
benchmarks are dominated by variance (see †), and differences under ~5% between
adjacent commits should not be treated as real.
