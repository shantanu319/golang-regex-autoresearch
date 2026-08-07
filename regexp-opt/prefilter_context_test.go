// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package regexp

import (
	"fmt"
	stdregexp "regexp"
	"strings"
	"testing"
)

// The prefilters added to machine.match (start-rune filter, inner-literal
// prefilter) skip candidate positions, and they only run on the NFA path.
// Inputs of up to maxBacktrackVector/len(prog.Inst) bytes are handled by the
// bitstate backtracker instead, which is why the rest of the test suite — whose
// inputs are all small — never exercises these filters. Anything testing them
// has to use an input big enough to leave the backtracker behind.
//
// Skipping ahead invalidates the empty-width context of the position being
// skipped from, so zero-width assertions are the interesting case.

var zeroWidthPatterns = []string{
	`\b\w{4,}\b`,
	`\bSherlock\b`,
	`\b\w+ing\b`,
	`\Bell\b`,
	`\bcafe\b`,
	`(?m)^\w+`,
	`(?m)\w+$`,
	`\b\w+@\w+\.\w+\b`,
}

// largeZeroWidthText builds an input well past the backtracker cutoff. The
// interesting positions are words preceded by two or more non-word bytes
// (", " and "\r\n"), since a filter that skips a run of them lands on a word
// start carrying the context of wherever the skip began.
func largeZeroWidthText() string {
	var b strings.Builder
	for i := 0; b.Len() < 256<<10; i++ {
		fmt.Fprintf(&b, "Sherlock Holmes, telling %d tales.\r\n", i)
		b.WriteString("  spelling bell cafe naive words here\r\n")
		fmt.Fprintf(&b, "\"quoted\" -- watson%d@example.com;  end\r\n", i)
		// Bare \n as well, so (?m)$ has somewhere to match: with \r\n line
		// endings the \r blocks any \w+$ match.
		fmt.Fprintf(&b, "plain line %d ending in a word\n", i)
	}
	return b.String()
}

func TestZeroWidthAssertionAfterPrefilterSkip(t *testing.T) {
	text := largeZeroWidthText()

	for _, pat := range zeroWidthPatterns {
		re := MustCompile(pat)

		// Confirm this input really does bypass the backtracker; if it did not,
		// the test would silently stop covering the path it exists to cover.
		if maxBitStateLen(re.prog) >= len(text) {
			t.Errorf("%q: input of %d bytes still fits the backtracker (limit %d)",
				pat, len(text), maxBitStateLen(re.prog))
			continue
		}

		want := stdregexp.MustCompile(pat).FindAllStringIndex(text, -1)
		got := re.FindAllStringIndex(text, -1)

		if len(got) != len(want) {
			t.Errorf("%q: got %d matches, want %d", pat, len(got), len(want))
			continue
		}
		for i := range want {
			if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
				t.Errorf("%q: match %d = %v, want %v", pat, i, got[i], want[i])
				break
			}
		}
	}
}

// TestZeroWidthAssertionEngineAgreement checks the NFA against the backtracker
// directly: the same pattern over a growing prefix must report the same matches
// within the shared region, whichever engine the length happens to select.
func TestZeroWidthAssertionEngineAgreement(t *testing.T) {
	text := largeZeroWidthText()
	compared := 0

	for _, pat := range zeroWidthPatterns {
		re := MustCompile(pat)
		cutoff := maxBitStateLen(re.prog)
		if cutoff <= 0 || cutoff >= len(text) {
			continue
		}
		// One length just inside the backtracker, one just outside it.
		short := re.FindAllStringIndex(text[:cutoff], -1)
		long := re.FindAllStringIndex(text[:cutoff+1024], -1)

		n := 0
		for n < len(short) && n < len(long) && short[n][1] <= cutoff-64 {
			if short[n][0] != long[n][0] || short[n][1] != long[n][1] {
				t.Errorf("%q: engines disagree at match %d: backtracker %v, NFA %v",
					pat, n, short[n], long[n])
				break
			}
			n++
		}
		compared += n
	}
	if compared == 0 {
		t.Fatal("compared no matches; the test is not covering anything")
	}
}
