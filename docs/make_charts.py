#!/usr/bin/env python3
"""Regenerate the result charts in docs/ from the measured benchmark data.

    python3 docs/make_charts.py

Writes light and dark variants of each chart; README.md picks between them with
a <picture> element. No dependencies — the SVG is emitted directly.

The numbers below are medians of the three samples harness/score.sh takes per
benchmark, measured one commit at a time on a single idle machine. See the
"Measurement notes" section of README.md.
"""

import math
import os

# ---------------------------------------------------------------- data

# (commit, geomean ns/op) in chronological order.
GEOMEAN = [
    ("9f13eb0", 3645376.82),
    ("d428c38", 2688571.02),
    ("60cc9ec", 1562433.91),
    ("77ac2be", 1161105.69),
    ("210f1df", 1169536.66),
    ("34da8aa", 1028422.83),
    ("1a263ac", 1004027.95),
    ("d4a00ac", 1064095.49),
    ("24959aa", 1015709.38),
    ("64ccec5", 961950.16),
    ("589b7f3", 509322.52),
    ("fcc9bf2", 502852.85),
]

# Commits measured while the zero-width assertion bug was present: 60cc9ec
# introduced it, fcc9bf2 fixed it.
BUGGY_FIRST, BUGGY_LAST = 2, 10

# (benchmark, baseline ns/op, current ns/op). The two compile benchmarks are
# left out: their run-to-run spread reaches 3.6x, so they carry no signal.
DUMBBELL = [
    ("CharacterClass", 104690394060, 241726801),
    ("AlternationMedium", 52064309, 2236317),
    ("BoundedRepeat", 37971835, 2484725),
    ("LiteralCaseInsensitive", 23287793, 3707377),
    ("LiteralMatch", 97233, 36742),
    ("UnicodeWordBoundary", 49383481, 31398430),
    ("NonMatch", 19250, 18930),
]

# ---------------------------------------------------------------- theme

THEMES = {
    # series      = categorical slot 1, for the single-series column chart
    # before/after= the dumbbell's two shades of that one hue. In each mode the
    #               "after" end is the higher-contrast step, so the optimized
    #               value is the emphasis; that inverts in lightness between
    #               modes, which is why these are selected per theme rather than
    #               flipped automatically.
    "light": dict(
        surface="#fcfcfb", ink="#0b0b0b", ink2="#52514e", muted="#898781",
        grid="#e1e0d9", axis="#c3c2b7", series="#2a78d6",
        before="#86b6ef", after="#2a78d6",
    ),
    "dark": dict(
        surface="#1a1a19", ink="#ffffff", ink2="#c3c2b7", muted="#898781",
        grid="#2c2c2a", axis="#383835", series="#3987e5",
        before="#256abf", after="#9ec5f4",
    ),
}

# Single quotes inside: this is interpolated into a double-quoted SVG attribute.
FONT = "system-ui,-apple-system,'Segoe UI',Roboto,sans-serif"


def esc(s):
    return (s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


def text(x, y, s, size, fill, anchor="start", weight="400", tabular=False):
    extra = ' style="font-variant-numeric:tabular-nums"' if tabular else ""
    return (f'<text x="{x:.1f}" y="{y:.1f}" font-family="{FONT}" font-size="{size}" '
            f'font-weight="{weight}" fill="{fill}" text-anchor="{anchor}"{extra}>'
            f'{esc(s)}</text>')


def ms(ns):
    return f"{ns / 1e6:.2f} ms"


# ---------------------------------------------------------------- chart 1

def chart_geomean(theme):
    t = THEMES[theme]
    W, H = 860, 478
    L, R, TOP = 78, 26, 104
    plot_h = 256
    base_y = TOP + plot_h

    # Round the axis top up to a clean number so the ticks land on whole ms.
    ymax = 4_000_000.0
    band = (W - L - R) / len(GEOMEAN)
    bw = 24.0

    o = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" '
         f'viewBox="0 0 {W} {H}" role="img" aria-label="Suite geomean ns/op '
         f'across each experiment, falling from 3.65 ms to 0.50 ms">',
         f'<rect width="{W}" height="{H}" fill="{t["surface"]}"/>']

    o.append(text(L, 38, "Suite geomean, stock go/regexp to now", 19, t["ink"], weight="600"))
    o.append(text(L, 60, "Geometric mean of ns/op over all 9 benchmarks. Lower is faster.",
                  13, t["ink2"]))

    # gridlines + y ticks
    for i in range(5):
        v = ymax * i / 4
        y = base_y - plot_h * (v / ymax)
        o.append(f'<line x1="{L}" y1="{y:.1f}" x2="{W-R}" y2="{y:.1f}" '
                 f'stroke="{t["grid"]}" stroke-width="1"/>')
        o.append(text(L - 12, y + 4, f"{v/1e6:.0f} ms" if i else "0", 11,
                      t["muted"], anchor="end", tabular=True))

    # bars
    for i, (sha, v) in enumerate(GEOMEAN):
        cx = L + band * (i + 0.5)
        x = cx - bw / 2
        h = plot_h * (v / ymax)
        y = base_y - h
        r = 4.0
        o.append(f'<path d="M{x:.1f},{base_y} L{x:.1f},{y+r:.1f} '
                 f'Q{x:.1f},{y:.1f} {x+r:.1f},{y:.1f} '
                 f'L{x+bw-r:.1f},{y:.1f} Q{x+bw:.1f},{y:.1f} {x+bw:.1f},{y+r:.1f} '
                 f'L{x+bw:.1f},{base_y} Z" fill="{t["series"]}"/>')
        o.append(text(cx, base_y + 17, sha, 10, t["muted"], anchor="middle"))

    # baseline rule
    o.append(f'<line x1="{L}" y1="{base_y}" x2="{W-R}" y2="{base_y}" '
             f'stroke="{t["axis"]}" stroke-width="1"/>')

    # selective direct labels: first and last only
    for i in (0, len(GEOMEAN) - 1):
        sha, v = GEOMEAN[i]
        cx = L + band * (i + 0.5)
        y = base_y - plot_h * (v / ymax)
        o.append(text(cx, y - 9, ms(v), 11, t["ink"], anchor="middle", weight="600",
                      tabular=True))

    # 7.25x callout spanning first to last bar, floated above the tallest bar
    x0 = L + band * 0.5
    x1 = L + band * (len(GEOMEAN) - 0.5)
    # Sits above the top gridline, clear of the tallest bar's own value label.
    yb = TOP - 12
    o.append(f'<line x1="{x0:.1f}" y1="{yb:.1f}" x2="{x1:.1f}" y2="{yb:.1f}" '
             f'stroke="{t["axis"]}" stroke-width="1"/>')
    o.append(f'<line x1="{x0:.1f}" y1="{yb:.1f}" x2="{x0:.1f}" y2="{yb+7:.1f}" '
             f'stroke="{t["axis"]}" stroke-width="1"/>')
    o.append(f'<line x1="{x1:.1f}" y1="{yb:.1f}" x2="{x1:.1f}" y2="{yb+7:.1f}" '
             f'stroke="{t["axis"]}" stroke-width="1"/>')
    mid = (x0 + x1) / 2
    o.append(f'<rect x="{mid-40:.1f}" y="{yb-11:.1f}" width="80" height="18" rx="3" '
             f'fill="{t["surface"]}"/>')
    o.append(text(mid, yb + 3, "7.25x faster", 12, t["ink"], anchor="middle", weight="600"))

    # annotation: the span measured with the bug present
    ax0 = L + band * (BUGGY_FIRST + 0.5) - bw / 2
    ax1 = L + band * (BUGGY_LAST + 0.5) + bw / 2
    ay = base_y + 34
    o.append(f'<path d="M{ax0:.1f},{ay+6} L{ax0:.1f},{ay} L{ax1:.1f},{ay} '
             f'L{ax1:.1f},{ay+6}" fill="none" stroke="{t["muted"]}" stroke-width="1"/>')
    o.append(text((ax0 + ax1) / 2, ay + 21,
                  "measured with the zero-width assertion bug present, fixed in fcc9bf2",
                  11, t["muted"], anchor="middle"))

    o.append("</svg>")
    return "\n".join(o)


# ---------------------------------------------------------------- chart 2

def chart_dumbbell(theme):
    t = THEMES[theme]
    rows = len(DUMBBELL)
    W = 860
    L, R, TOP = 196, 84, 104
    rh = 40
    plot_h = rows * rh
    H = TOP + plot_h + 66

    lo, hi = 1e4, 2e11
    def sx(v):
        return L + (W - L - R) * (math.log10(v) - math.log10(lo)) / (math.log10(hi) - math.log10(lo))

    o = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" '
         f'viewBox="0 0 {W} {H}" role="img" aria-label="Per-benchmark time before '
         f'and after optimization on a logarithmic scale">',
         f'<rect width="{W}" height="{H}" fill="{t["surface"]}"/>']

    o.append(text(30, 38, "Time per operation, before and after", 19, t["ink"], weight="600"))
    o.append(text(30, 60,
                  "Median ns/op, log scale. The two compile benchmarks are omitted: "
                  "their spread reaches 3.6x, so they carry no signal.",
                  13, t["ink2"]))

    # legend (2 series)
    lx, ly = 30, 82
    o.append(f'<circle cx="{lx+5}" cy="{ly-4}" r="5" fill="{t["before"]}"/>')
    o.append(text(lx + 17, ly, "stock go/regexp", 12, t["ink2"]))
    o.append(f'<circle cx="{lx+143}" cy="{ly-4}" r="5" fill="{t["after"]}"/>')
    o.append(text(lx + 155, ly, "optimized (fcc9bf2)", 12, t["ink2"]))

    # x gridlines
    unit = {4: "10 us", 5: "100 us", 6: "1 ms", 7: "10 ms", 8: "100 ms",
            9: "1 s", 10: "10 s", 11: "100 s"}
    for e in range(4, 12):
        x = sx(10.0 ** e)
        o.append(f'<line x1="{x:.1f}" y1="{TOP-8}" x2="{x:.1f}" y2="{TOP+plot_h}" '
                 f'stroke="{t["grid"]}" stroke-width="1"/>')
        o.append(text(x, TOP + plot_h + 20, unit[e], 11, t["muted"], anchor="middle",
                      tabular=True))

    for i, (name, before, after) in enumerate(DUMBBELL):
        cy = TOP + rh * i + rh / 2
        xb, xa = sx(before), sx(after)
        o.append(text(L - 16, cy + 4, name, 12, t["ink"], anchor="end"))
        # connector, then dots with a 2px surface ring
        if xb - xa < 12:
            # The two points nearly coincide, so a surface ring would hide the
            # "before" dot entirely and the row would read as missing data.
            # Nest them instead: a light halo around a dark core says "no change".
            o.append(f'<circle cx="{xb:.1f}" cy="{cy:.1f}" r="7" '
                     f'fill="{t["before"]}"/>')
            o.append(f'<circle cx="{xa:.1f}" cy="{cy:.1f}" r="4" '
                     f'fill="{t["after"]}"/>')
        else:
            o.append(f'<line x1="{xa:.1f}" y1="{cy:.1f}" x2="{xb:.1f}" y2="{cy:.1f}" '
                     f'stroke="{t["before"]}" stroke-width="2"/>')
            o.append(f'<circle cx="{xb:.1f}" cy="{cy:.1f}" r="5" fill="{t["before"]}" '
                     f'stroke="{t["surface"]}" stroke-width="2"/>')
            o.append(f'<circle cx="{xa:.1f}" cy="{cy:.1f}" r="5" fill="{t["after"]}" '
                     f'stroke="{t["surface"]}" stroke-width="2"/>')
        # direct label: the speedup, at the row end
        o.append(text(W - R + 14, cy + 4, f"{before/after:,.2f}x".replace(",", ""),
                      12, t["ink"], weight="600", tabular=True))

    o.append(text(W - R + 14, TOP - 14, "speedup", 11, t["muted"]))
    o.append("</svg>")
    return "\n".join(o)


# ---------------------------------------------------------------- main

def main():
    here = os.path.dirname(os.path.abspath(__file__))
    for theme in ("light", "dark"):
        for name, fn in (("geomean", chart_geomean), ("benchmarks", chart_dumbbell)):
            path = os.path.join(here, f"results-{name}-{theme}.svg")
            with open(path, "w") as f:
                f.write(fn(theme) + "\n")
            print("wrote", os.path.relpath(path, os.path.dirname(here)))


if __name__ == "__main__":
    main()
