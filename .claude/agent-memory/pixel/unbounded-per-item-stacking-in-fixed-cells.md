---
name: unbounded-per-item-stacking-in-fixed-cells
description: Rendering one element per data row inside a fixed-size grid cell (calendar day, heatmap cell) needs a cap + overflow indicator, or the whole grid row distorts
type: feedback
---

**Rule**: Whenever a template renders "one visual element per item" inside a cell that lives in a CSS grid row (month calendar day cells, weekly heatmap cells, any `grid-cols-N` layout), the count is unbounded user data — always cap the rendered items and fall back to a "+N" affordance past the cap. Never assume "in practice there are only 1-2 per cell."

**Why**: Verified live (2026-08-18, dashboard training-history change) — stacking one `.calendar-session-dash` bar per session in a month-calendar day cell works fine at 1-3 sessions, but with 7 sessions in one day (3 scheduled seed + 4 logged-in-review ad-hoc "unplanned" runs) the cell's content grew past the `min-h-[56px]`, and because CSS grid rows default to `align-items: stretch`, *every other cell in that same week row* got stretched to match — the whole row visibly ballooned (56px → 74px) even though only one day had extra data. The stacked bars also visually read as a "barcode" rather than legible per-session information once there were more than ~3.

**How to apply**: When reviewing a calendar/heatmap-style cell that now renders N items instead of a single aggregate:
1. Ask "what happens at 5, 10, 20 items?" — don't just check the seed data's typical case.
2. Check whether the cell's container is a CSS grid — if so, one tall cell distorts the whole row, not just itself.
3. Propose a cap matching the existing chip-overflow rule (DESIGN.md: max ~4 chips, then "+N more") rather than an uncapped stack.
