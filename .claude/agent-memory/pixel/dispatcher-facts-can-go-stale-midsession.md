---
name: dispatcher-facts-can-go-stale-midsession
description: A dispatcher's briefed "hard facts" about a file can already be stale by the time the review runs, if the owner is iterating live
type: feedback
---

## What happened

Dispatched to review `/exercise-library` wide-view spacing. The brief stated as
measured fact: 72rem container, row carries only glyph+name+labels (source
field "deliberately removed, settled"), widest name 261px / widest label
199px on page 1. On read, the live file (owner mid-iteration, "~8 iterations
today") had already moved past this: page uses `passion-container-wide`
(84rem/1344px, correctly per DESIGN.md's "data-heavy pages" rule — the 72rem
claim was simply wrong), and Source had been reintroduced as its own **fixed
11rem grid track**, single-line, 4-column — which is exactly the "reserved
fixed track for a sparse field" anti-pattern the owner said they'd already
rejected. 84% of rows have a source, so it's not even that sparse, but the
16% without one still show a dead 11rem gutter — a live regression against
the owner's own stated constraint, discovered only by reading the actual
file instead of trusting the brief.

## Rule

Treat a dispatcher's "hard facts I have measured" as a starting hypothesis,
not ground truth, whenever the target file shows independent signs of active
iteration (recent modification, a `git status` diff, "N iterations today" in
the prompt). Re-derive the load-bearing numbers from the live file/rendered
page rather than the brief. When the live state contradicts the brief,
report the contradiction prominently before giving the recommendation —
it changes the arithmetic and may reveal the user has already reintroduced
a pattern they meant to keep out.

## How to apply

- Always read the actual current template + CSS before trusting any
  "current state" description in a dispatch prompt, even a detailed one.
- Measure real content widths via canvas text measurement (`ctx.measureText`
  with the element's computed font), not `getBoundingClientRect()` on a grid
  item — a grid item's box reflects its track size, not its content, once
  it's inside `display:grid`.
- Check sparsity/frequency of an optional field across the *whole* dataset
  (not just one page) before calling a field "sparse" — 84% present changes
  the argument from "rare hole" to "wastes a track most rows don't need
  reserved at all," which is a different and stronger point.
