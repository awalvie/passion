---
name: Route logging flow design
description: Fast in-session per-route logging — inheritance, outcome quick-actions, Focus/Thoughts
type: project
---

The per-route logging form (run_ticks.html / ClimbingTick model) had ~9 fields all visible per entry. Goal: cut per-burn tapping.

Model already supports everything (no migration): ClimbingTick has Kind, Setting, Subtype, Grade, Focus (pre-climb intention), Thoughts (post-climb reflection), Style, RopeStyle, Attempts, Sent, Stars. Venue lives on the climbing Exercise (VenueID), outcome data on the tick — this constant/per-climb split is correct, lean into it.

Recommendations given (ranked):
1. **Inherit constants from previous tick** in same RunID+ExerciseID (Kind/Setting/Subtype/RopeStyle/VenueID/Grade), collapse them into one editable summary line. Highest leverage, lowest cost. This is the core of the ask. Edge case: first tick in run has nothing to inherit.
2. **Outcome quick-action row** `Flash · Send · +Attempt · Working` replacing separate Sent/Attempts/Style. Infer Style server-side from button + attempt count; keep editable in expanded view.
3. **Grade as a tap-chip strip** centered on last logged grade, not a dropdown.

Focus vs Thoughts: per-climb Focus (pre-climb intention) is genuinely novel — no mainstream climbing app surfaces structured pre-climb intention (8a/MP have only post-hoc comment; Crimpd/Lattice have session-level feel checks). Two design forks, USER MUST DECIDE before build:
- Two-phase pending tick (create with Focus before burn, complete with outcome+Thoughts after) — matches stated intent, medium-high cost.
- Single collapsed notes disclosure (Focus + Thoughts together) — low cost, but Focus isn't truly "before".

Anti-patterns to avoid: required fields in fast path (only outcome required), modals that interrupt run view (use inline append), blank-form-per-route, mandatory Style dropdown.

App references for this domain: board apps (Kilter/Tension/Stokt/KAYA) = lowest friction because wall IS the route context + they inherit gym/wall across consecutive logs. Mountain Project tick = minimal (route is the constant). 8a/Vertical-Life = high friction but batch post-session. Crimpd/Lattice = pre-fill-from-plan, tap-to-confirm.
