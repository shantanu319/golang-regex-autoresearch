// Command verify is the differential correctness gate: it runs a corpus of
// patterns through both the standard library's regexp and the package under
// optimization, and reports any disagreement.
//
//	go run ./harness/verify
//
// The vendored unit tests in regexp-opt/ cannot catch a whole class of bug on
// their own. Every input they use is a few hundred bytes, and regexp routes
// inputs of up to maxBacktrackVector/len(prog.Inst) bytes to the bitstate
// backtracker. The NFA path — which is what the prefilters and skip-scans in
// this project actually change — is therefore barely exercised for correctness
// by the suite. A start-rune filter that silently dropped matches for any
// pattern with a leading \b passed `go test ./regexp-opt/...` for ten commits.
//
// So this gate deliberately drives inputs past that cutoff, and checks results
// against the standard library rather than against a fixture. Anything that
// changes observable match behaviour fails here.
package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	std "regexp"

	opt "autoresearch-regex/regexp-opt"
)

// Patterns worth disagreeing over. Zero-width assertions come first: they are
// the ones that depend on match-position context, which is what a prefilter
// that skips positions is most likely to invalidate.
var patterns = []string{
	// zero-width assertions
	`\b\w{4,}\b`, `\bSherlock\b`, `\b\w+ing\b`, `\Bell\b`, `\bcafe\b`,
	`(?m)^\w+`, `(?m)\w+$`, `^Sherlock`, `Holmes$`, `\byes\b|\bno\b`,
	`\b[A-Z]\w*\b`, `\B\w\B`, `(?m)^\s*$`, `\bthe\b.{0,20}\bof\b`,

	// literals and case folding
	`Sherlock Holmes`, `(?i)sherlock holmes`, `ZZZZZNOTFOUND`, `Watson`,
	`(?i)WATSON`, `(?i)the`,

	// alternation
	`Sherlock|Watson|Holmes|Moriarty|Lestrade`, `abc|def|ghi`,
	`(?i)sherlock|watson`, `foo|foobar`,

	// character classes and repeats
	`[a-zA-Z]+@[a-zA-Z]+\.[a-zA-Z]+`, `[a-z]{2,4}ing`, `[0-9]{2,5}`,
	`[^aeiou]{5,}`, `[[:alpha:]]{6,}`, `\d+\.\d+`, `\w+-\w+`,
	`[a-z]*ing`, `a+b*c?`, `(ab){2,3}`,

	// groups, submatches, anchors
	`(\w+)@(\w+)`, `(a|b)(c|d)`, `((?:ab)+)c`, `(?:foo)?bar`,
	`^(\w+)\s+(\w+)$`, `(\d{4})-(\d{2})-(\d{2})`,

	// unicode
	`\p{L}{5,}`, `\pN+`, `[é-ü]+`, `naïve|café`, `.{3}é`,

	// dot and unanchored scans
	`a.c`, `(?s)a.c`, `.*`, `x*`, `(?i)[a-f0-9]{6}`,
}

// input is one haystack to run the corpus against.
type input struct {
	name string
	text string
}

func inputs() []input {
	var in []input

	in = append(in, input{"small", "Sherlock Holmes, telling tales.\r\n  spelling bell cafe naïve\n"})

	// Sized to sit either side of the bitstate cutoff. For a 9-instruction
	// program that boundary is 262144/9 = 29127 bytes, so 4 KB stays on the
	// backtracker and 64 KB / 512 KB are firmly on the NFA.
	in = append(in,
		input{"4KB", synthetic(4 << 10)},
		input{"64KB", synthetic(64 << 10)},
		input{"512KB", synthetic(512 << 10)},
	)

	// The real corpus, when prepare.sh has been run.
	if b, err := os.ReadFile("bench/testdata/sherlock.txt"); err == nil {
		if len(b) > 512<<10 {
			b = b[:512<<10]
		}
		// Trim to a rune boundary so the slice stays valid UTF-8.
		for len(b) > 0 && !utf8.Valid(b) {
			b = b[:len(b)-1]
		}
		in = append(in, input{"sherlock.txt", string(b)})
	}
	return in
}

// synthetic builds text with the features that break position-dependent
// prefilters: runs of two or more non-word bytes before a word (", ", "\r\n",
// quotes), mixed line endings, multi-byte runes, and matchable e-mail shapes.
func synthetic(n int) string {
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		fmt.Fprintf(&b, "Sherlock Holmes, telling %d tales.\r\n", i)
		b.WriteString("  spelling bell cafe naïve résumé words here\r\n")
		fmt.Fprintf(&b, "\"quoted\" -- watson%d@example.com;  end\r\n", i)
		b.WriteString("plain line ending in a word\n")
		fmt.Fprintf(&b, "2011-04-%02d ZXCVB12345 not-an-email\n", i%28+1)
	}
	s := b.String()
	if len(s) > n {
		s = s[:n]
		for len(s) > 0 && !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}

type failure struct {
	pattern, input, api, detail string
}

func main() {
	ins := inputs()
	var fails []failure
	checks := 0

	for _, pat := range patterns {
		sre, err1 := std.Compile(pat)
		ore, err2 := opt.Compile(pat)
		if (err1 == nil) != (err2 == nil) {
			fails = append(fails, failure{pat, "-", "Compile",
				fmt.Sprintf("stdlib err=%v, regexp-opt err=%v", err1, err2)})
			continue
		}
		if err1 != nil {
			continue // both rejected it; nothing to compare
		}

		for _, in := range ins {
			checks++

			// All non-overlapping matches, string path.
			want := sre.FindAllStringIndex(in.text, -1)
			got := ore.FindAllStringIndex(in.text, -1)
			if d := diffPairs(want, got, in.text); d != "" {
				fails = append(fails, failure{pat, in.name, "FindAllStringIndex", d})
				continue
			}

			// Same, byte path — a different input implementation.
			wantB := sre.FindAllIndex([]byte(in.text), -1)
			gotB := ore.FindAllIndex([]byte(in.text), -1)
			if d := diffPairs(wantB, gotB, in.text); d != "" {
				fails = append(fails, failure{pat, in.name, "FindAllIndex", d})
				continue
			}

			// Submatches take a different code path than a plain match.
			ws := sre.FindStringSubmatchIndex(in.text)
			gs := ore.FindStringSubmatchIndex(in.text)
			if !eqInts(ws, gs) {
				fails = append(fails, failure{pat, in.name, "FindStringSubmatchIndex",
					fmt.Sprintf("stdlib %v, regexp-opt %v", ws, gs)})
				continue
			}

			// And the boolean fast path.
			if sre.MatchString(in.text) != ore.MatchString(in.text) {
				fails = append(fails, failure{pat, in.name, "MatchString",
					fmt.Sprintf("stdlib %v, regexp-opt %v",
						sre.MatchString(in.text), ore.MatchString(in.text))})
			}
		}
	}

	fmt.Printf("verify: %d patterns x %d inputs, %d comparisons\n",
		len(patterns), len(ins), checks*4)

	if len(fails) == 0 {
		fmt.Println("verify: PASS — regexp-opt agrees with the standard library")
		return
	}

	fmt.Printf("\nverify: FAIL — %d disagreement(s) with the standard library\n\n", len(fails))
	for i, f := range fails {
		if i == 12 {
			fmt.Printf("  ... and %d more\n", len(fails)-i)
			break
		}
		fmt.Printf("  pattern %-28q input %-13s %s\n      %s\n",
			f.pattern, f.input, f.api, f.detail)
	}
	os.Exit(1)
}

// diffPairs compares two match lists and describes the first difference,
// including the surrounding text, so a failure points at a concrete position.
func diffPairs(want, got [][]int, text string) string {
	if len(want) != len(got) {
		// Find where they diverge, to make the count difference actionable.
		n := len(want)
		if len(got) < n {
			n = len(got)
		}
		for i := 0; i < n; i++ {
			if want[i][0] != got[i][0] || want[i][1] != got[i][1] {
				return fmt.Sprintf("stdlib found %d matches, regexp-opt %d; first differs at %d: %v vs %v near %s",
					len(want), len(got), i, want[i], got[i], context(text, want[i][0]))
			}
		}
		if len(got) < len(want) && n < len(want) {
			return fmt.Sprintf("stdlib found %d matches, regexp-opt %d; regexp-opt stops after %d, missing %v near %s",
				len(want), len(got), n, want[n], context(text, want[n][0]))
		}
		return fmt.Sprintf("stdlib found %d matches, regexp-opt %d", len(want), len(got))
	}
	for i := range want {
		if want[i][0] != got[i][0] || want[i][1] != got[i][1] {
			return fmt.Sprintf("match %d: stdlib %v, regexp-opt %v near %s",
				i, want[i], got[i], context(text, want[i][0]))
		}
	}
	return ""
}

func context(text string, pos int) string {
	lo, hi := pos-24, pos+24
	if lo < 0 {
		lo = 0
	}
	if hi > len(text) {
		hi = len(text)
	}
	for lo < len(text) && !utf8.RuneStart(text[lo]) {
		lo++
	}
	for hi < len(text) && !utf8.RuneStart(text[hi]) {
		hi++
	}
	return fmt.Sprintf("offset %d %q", pos, text[lo:hi])
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
