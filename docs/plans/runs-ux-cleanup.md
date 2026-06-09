# Plan — /runs page UX cleanup

Grab-bag of UX issues on the run player, mostly in the **open-session** view
([open_session.html](../../templates/open_session.html)) and the run player
([run.html](../../templates/run.html)). Investigated against the live page at
`/runs/47` (an open session). Grouped by confidence.

## Confirmed issues + fixes

### 1. Open session has no way to go back / exit
**Where:** open-session view. The active-run hero has a back arrow
([run.html:432](../../templates/run.html)), and the completed state has "Return
to dashboard" (run.html:26), but the open-session step view has no exit
affordance — you're stuck unless you finish/discard.
**Fix:** add a back/exit control. Two parts:
- A back arrow in the step header (consistent with the active-run hero's
  `arrow-left` icon) that returns to the open-session overview
  (`/runs/{id}` without `?exercise=`).
- From the overview, a clear "Back to dashboard" link (the session auto-saves;
  it's not the same as Finish).
**Decision needed:** should leaving an open session mid-step just navigate to the
overview (progress kept, session stays open), or is there an expectation it
"pauses"? Default: plain navigation, session stays open — no new state.

### 2. Stray "weird circle" — literal `○` glyph
**Where:** [open_session.html:55](../../templates/open_session.html) — a pending
exercise card renders `<span aria-hidden>○</span>` as a status bullet, but the
card is also a `passion-disclosure` which injects its own chevron `::before`.
Result: a chevron AND a floating circle, the circle "hanging in space".
**Fix:** remove the `○` span. The disclosure chevron already signals
expand/collapse; the bullet is redundant and visually orphaned. (If a
done/pending status marker is wanted, use a proper `circle`/`check-circle`
lucide icon aligned in the row, not a bare text glyph.)

### 3. "Session steps" toggle does nothing (open session)
**Where:** the `#run-playlist-toggle` button + its handler exist only in the
active-run branch ([run.html:671](../../templates/run.html), handler ~858). On
an open session the toggle isn't present / the playlist panel isn't wired, so
the button (if shown) is dead.
**Fix:** confirm which view shows a "Session steps" button with no working
panel, and either (a) wire it to the existing `run-playlist-open` toggle, or
(b) remove the button in the view where there's no panel. Likely the
open-session overview *is* the step list, so the button is redundant there →
remove it.

### 4. Finish / Discard awkwardly stacked
**Where:** [run.html](../../templates/run.html) `run-session-actions` (added in
the prior change) stacks "Finish session" (full-width bordered button) directly
above a "Discard session" text link, centred — they read as a cramped stack.
**Fix:** give them clearer separation and hierarchy. Options:
- Finish as the full-width primary action; Discard as a smaller, muted text
  link with more top margin (e.g. `mt-3`) and reduced weight, clearly
  secondary/destructive. OR
- Put Discard inline-right and Finish inline-left on one row at wider widths.
Recommend the first (vertical, but with breathing room + de-emphasised Discard).
**Decision needed:** confirm the visual hierarchy (primary Finish + subtle
Discard) is what you want.

### 5. Empty space beside the timer when no playback buttons
**Where:** [run.html](../../templates/run.html) run transport. For climbing /
session steps (`run-transport--session`) the skip-back/skip-forward buttons are
hidden, leaving only the centred play/pause — but the transport row still
reserves the side slots, so the timer/controls area has dead space.
**Fix:** when the step has no skip buttons, collapse the transport to centre the
play control (or drop the empty slots) so there's no orphaned gap. Verify
against `.run-transport--session` CSS.

### 6. Form fields run together — no separators between sections
**Where:** the tick form (run_ticks.html) stacks Grade / Result / Attempts /
Focus / Thoughts / Stars with uniform `space-y-3` and identical tiny uppercase
labels — no visual division, so it reads as an undifferentiated wall.
**Fix:** add light separation between field groups — either a hairline rule
(`border-top var(--border)`) between logical sections, or stronger label
treatment. Keep it subtle (design system is density-first), but enough that
Grade/Result/Notes read as distinct blocks.

### 7. Bare floating text — `.run-session-strip` template name + "Add notes" toggle
**Where:** [run.html](../../templates/run.html)
- `.run-session-strip span.text-xs.muted` — the template name floats bare at the
  top of the player with no container.
- `#run-notes-details summary.run-collapse-toggle` ("Add notes") — a bare text
  toggle.
**Fix:** give the session-strip name proper alignment/treatment (it's metadata —
left-aligned muted is fine, but it currently looks orphaned next to the mute
button). Confirm the "Add notes" disclosure has a consistent chevron + hit area.

### 8. Logged tick row has dead empty space (icon buttons invisible on touch)
**Where:** [run_ticks.html](../../templates/fragments/run_ticks.html) tick
summary. The "Log again" + "Remove" icon buttons were given
`opacity-0 group-hover:opacity-100` (a desktop hover-reveal). **Confirmed live:
on touch they render at `opacity:0` with 44px width** — so on mobile they're
invisible but still occupy the row, leaving dead space after a logged tick AND
no way to delete/log-again by touch.
**Fix:** drop the hover-reveal for a touch-first run UI — make the icon buttons
always visible (optionally a subtle muted resting state that brightens on
hover/focus on desktop, but never `opacity:0`). This both removes the "empty
space" and restores touch access to the actions.

## Other improvements worth considering (proposed, not requested)

- **Sticky Finish/exit on long open sessions** — once you've logged many
  drills, Finish is a long scroll away. A small persistent exit/finish in the
  header would help.
- **Open-session step header** — show which drill you're on + a count
  ("Drill 3") for orientation, mirroring the active-run progress bar.
- **Consistent disclosure chevrons** — the run player mixes lucide
  `chevron-right` rotation (`.run-collapse-chevron`) with `passion-disclosure`
  `::before`. Standardise.

## Files likely touched

| File | Change |
|---|---|
| [open_session.html](../../templates/open_session.html) | Remove `○`; add back/exit; capsule metadata; drop dead "Session steps" button |
| [run.html](../../templates/run.html) | Finish/Discard spacing; transport empty-slot collapse; back affordance |
| [static/passion.css](../../static/passion.css) | `run-session-actions` spacing; transport centring; any new capsule usage |

## Review

After implementing: **pixel** (open-session + run player layout), **copy** (any
new labels), **scribe** (if structure changes). No model/handler changes
expected — this is template/CSS only.

## Decisions (resolved with user)

1. Leaving an open session mid-step = plain navigate to overview, session stays
   open. ✓
2. Finish/Discard hierarchy = primary Finish + subtle muted Discard. ✓
3. "Hanging text" = field-label separators (#6), the `.run-session-strip` name +
   "Add notes" toggle (#7). ✓
4. Tick row "empty space" = the hover-reveal icon buttons being invisible on
   touch (#8) — make them always visible. ✓

All resolved — implementing.
