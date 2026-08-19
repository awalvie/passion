---
name: Bechtel Logical Progression — verified mechanics
description: Sourced structure of Bechtel's nonlinear periodization (rotation, 3+1 wave, adaptation-by-session-count) so I don't re-research it
type: reference
---

Researched 2026-08 (the owner trains by this book, so it will recur). What is actually VERIFIED
from sources vs. what is practitioner interpretation:

## Verified from sources
- **Rotate stimuli session-to-session, not month-to-month.** Bechtel: "strength for one session,
  then rest a couple of days, train power for one session, rest a couple of days, then train your
  energy system." No blocks — every quality is trained all the way through, so you're never
  detrained and never sacrificing one quality to build another. That's the whole thesis: stay in
  a state of *balanced* fitness, ready to perform year-round.
- **3 weeks progressive, then deload = cut total training volume IN HALF.** Explicit and repeating.
  This is the concrete wave shape to use.
- **Adaptation is counted in SESSIONS, not calendar time.** "X number of sessions" to fully adapt
  to a stimulus; ~8-10 sessions of a quality before judging progress. A young athlete may adapt in
  5 weeks, an older one in 12 — depends on frequency, not the month.
- **Weekly frequency scales to the person's life.** 2x/week → alternate strength and power;
  5x/week → multiple sessions of each quality. The framework adapts to the schedule; it never
  demands compliance with a rigid calendar. (This is why it fits "adaptable to life" goals.)
- **Track climbing performance as the key factor**, with simple supporting metrics (hangboard load
  going up, reps going up). Average performance across a cycle before progressing; look for the
  lagging quality and shift emphasis toward it.
- Year-round with no built-in rest blocks, so practitioners must self-impose the taper/deload
  every 3-4 weeks. Non-negotiable, not optional.

## Practitioner interpretation (Mountain Project, NOT necessarily the book)
- Load STATIC across the 4-week block, VOLUME rises; add load for the *next* cycle. Example given:
  wk1-2 = 3-6-9s hangs x3-4 rounds, wk3 = x5 rounds, wk4 = 3-6-9-12s x3 rounds (matched TUT).
- After 3-4 cycles, move to smaller edges rather than adding ever more weight.

## Sources
- https://www.powercompanyclimbing.com/blog/2017/5/9/episode-39-logical-progression-pt-1-with-steve-bechtel
- https://www.mountainproject.com/forum/topic/113359355/anyone-out-there-experimenting-with-the-bechtel-logical-progression-format-nonli
- https://nerdclimbing.wordpress.com/2017/05/12/review-logical-progression-steve-bechtel/ (403 to fetch)
- Book: Bechtel et al., *Logical Progression*, ISBN 9781544119533

## How this maps to Passion's data model
Nonlinear periodization is ALREADY the shape Passion supports: a weekday→SessionTemplate map that
repeats weekly IS session-to-session stimulus rotation. What Passion does NOT express is the
volume wave (wk1-3 up, wk4 halved) — that is exactly what CycleExerciseWeekOverride /
CycleExerciseOverride are for, and the deload week is a calendar rest/deload event (per
project_cycle_metadata_and_equipment). So Logical Progression needs NO new model.
