---
name: Run player control-zone layout
description: Separate timer transport from set-completion by ROLE/ZONE; one primary action; notes below controls
type: project
---

Complaint driving this: "not sure if the play button, player controls and notes are organized well, it feels a tad bit clunky." Three separate control clusters stacked: (a) run-transport (big circular Play + skip-back/fwd chevrons), (b) run-nav-row ([‹][Done ✓][Skip][›]), (c) always-visible notes textarea — all crammed in/around .run-sticky-controls. Prior work (project_insession_notes) just made notes first-class/always-visible, which ADDED to the crowding.

Current run.html structure (~line 440-614): timer display + counters (REP/SET grid, UP NEXT row) live ABOVE the logging area; then .run-sticky-controls holds transport THEN nav-row; then the summary/set-log; then notes; then session lifecycle (Finish/Steps/Discard). Note: hero header ALREADY has a duplicate check button (#hero-complete-btn, line 421). Play uses --accent; Done uses --btn-primary (dark). Sticky only on mobile (desktop: position:static via md: block ~2128).

## Core finding: transport and completion are DIFFERENT ROLES — separate them by zone

Best apps NEVER mix timer transport with set-completion/nav in one cluster. Two distinct jobs:
- Transport (play/pause/skip) = controls the CLOCK, used repeatedly WITHIN a rep/rest. Lives WITH the timer, up top, near the number it controls (Peloton/Apple Fitness/Crimpd: giant timer, pause tap ON or directly under the dial).
- Completion/nav (Done/Skip/prev/next) = advances the PLAN, used ONCE at end of exercise. Lives at the bottom thumb zone (Strong/Hevy: "Finish exercise" / big set-check button anchored bottom).

The clunk = these two roles are adjacent (transport stacked directly on nav-row), so the eye can't tell "adjust the clock" from "I'm done." Fix is spatial separation by role, not restyling.

## Recommendations given (ranked)

1. HIGHEST IMPACT — move transport UP to the timer, leave ONLY completion in the bottom bar.
   - Play/pause + skip chevrons attach directly under the .run-main-timer display (top). It controls the clock; put it on the clock. Crimpd/Peloton/Apple Fitness all do this.
   - Bottom sticky bar keeps ONLY [‹] [ Done ✓ ] [Skip] [›]. One clear primary action per zone.
   - Removes the "two big round buttons fighting" problem. LOW-MEDIUM cost (move a template block + move ~2 CSS rules). Reuse existing #hero-complete-btn precedent.

2. ONE PRIMARY ACTION for timed exercises (the single most-confusing bit): don't show a big Play AND a big Done simultaneously as co-equal CTAs. Progressive primary:
   - Before start: Play IS the primary (accent, large). Done is de-emphasized (ghost/secondary) or absent.
   - While running: transport shows Pause; Done stays secondary.
   - On timer complete (or last rep): auto-swap primary to Done (accent), fade transport back. Apple Fitness / Peloton do exactly this — the button that matters is always the loud one, and it changes with phase.
   - This is what "feels clunky" most: two equally-loud buttons with no hierarchy. Give them a phase-driven hierarchy instead of separating further.

3. NOTES BELOW the controls, not between them (confirmed on-philosophy). Notes are post-hoc reflection ("how did it feel"), written AFTER the set/exercise, not during. Correct reading order = do the thing (transport) → log the thing (sets) → mark done (completion) → reflect (notes). So notes sit BELOW the sticky completion bar in scroll order, above session lifecycle. Keep always-visible (per project_insession_notes) but as its own quiet tier (run-tier hairline) — prominent by being permanent, not by being loud/high. Strong/Hevy put per-exercise notes inline in the exercise block, never wedged into the action bar. DO NOT move notes into the sticky bar.

## Proposed control-zone layout (top→bottom)
```
┌─ sticky header: exercise name + progress ─────┐
│  TIMER  00:42                                 │  ← number
│  ‹‹  ( ▶ / ❚❚ )  ››     ← transport ON the clock
│  UP NEXT · REP 1/5 · SET 2/3                  │
├───────────────────────────────────────────────┤
│  [ set-log rows / tick logger / summary ]      │  ← log
├───────────────────────────────────────────────┤
│  Notes ▁ (always-visible, auto-grow)           │  ← reflect
├─ sticky bottom bar ───────────────────────────┤
│  [‹]   [   ✓ Done   ]   Skip   [›]              │  ← ONE primary, phase-driven
└───────────────────────────────────────────────┘
Finish session · Steps · Discard   (below fold, not sticky)
```

## Sticky bottom bar: yes, keep it, but minimal
Sticky bottom action bar is the RIGHT pattern (Strong/Hevy/Peloton all anchor the primary action in the thumb zone on mobile). Rule: the sticky bar holds ONLY the completion/nav cluster — the single most-important next action. Everything else (transport→top, notes→inline, lifecycle→below) leaves it. Currently desktop already un-stickies it (position:static ~line 2128), which is fine.

## Anti-patterns flagged
- Transport + completion as co-equal loud buttons stacked = the reported clunk. Give hierarchy (phase-driven primary) OR separate by zone; ideally both.
- Notes wedged between/inside the control clusters — breaks the do→log→reflect reading order and steals thumb-zone real estate.
- Duplicate completion affordances (hero check at line 421 AND Done in bar). Pick one canonical Done or make the hero one clearly a shortcut; two identical checks in view = ambiguity.
- Skip styled as loud as Done — Skip should be visibly secondary (it's the exception path).
- A bottom bar that grows to 3 rows tall (transport + nav + notes) — eats half the mobile viewport; the log area (the actual work) gets squeezed.

## Complexity: recommendations 1 + 3 are LOW-MEDIUM (template move + CSS relocation, no model/handler change). Recommendation 2 (phase-driven primary swap) is MEDIUM — JS to retarget which button is .accent by timer phase; there's already phase JS (btn-play-pause, auto-advance) to hang it on. All within run.html + passion.css. No migration.
