---
name: border-token-fails-as-chip-background
description: var(--border) as a text-chip background fails AA contrast against --muted text in both themes; the muted-chip pattern must use var(--card-muted)
type: feedback
---

## Rule
When a new "muted"/neutral status chip is built with `background:var(--border);color:var(--muted)`,
check contrast before approving it. Computed contrast of `--muted` text on `--border` background
is ~3.9:1 in both the light and dark palettes (as of the tokens in `static/passion.css` at time of
writing) — below the 4.5:1 AA threshold for the small (`text-[11px]`) text these chips use.

The documented muted-chip pattern in `docs/DESIGN.md` uses `background:var(--card-muted)`, not
`var(--border)`. `--card-muted` is deliberately set equal to `--bg` in both themes so it reads as a
faint tint against `--panel`, while staying high-contrast with `--muted` text (~4.6:1 light, ~5.5:1
dark). `--border` is a hairline/divider color, not a text-background color — it's visually close to
`--card-muted` in light mode (easy to reach for by mistake) but meaningfully darker in dark mode,
which is why the failure shows up in both themes despite looking like a small change.

## How to apply
Any time a template introduces `style="background:var(--border)...` on a chip/badge that carries
text, flag it and propose `var(--card-muted)` instead (or `var(--accent-bg)`/`--warn-bg`/a
`--tick-*-bg` if the chip is meant to carry semantic color, not muted neutral). Confirm visually
with a screenshot in both light and dark — the faintness is visible even before doing the math.

## Where this came from
Found reviewing the "Edited" catalog chip added to exercise_library.html, templates.html and
activity_templates.html (Sept 2026) — all six occurrences used `background:var(--border)` instead
of the documented `--card-muted` pairing.
