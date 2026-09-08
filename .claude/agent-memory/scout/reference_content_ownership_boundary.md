---
name: App-content vs user-content boundary — verified behaviour
description: Cited, checked behaviour of Hevy/Strong/TrainingPeaks/Garmin/Crimpd/MacroFactor on immutable built-ins, fork points, update propagation and history immutability
type: reference
---

# Verified 2026-09-07 (web-checked, not recalled)

Research for the "how do apps model the app-content / user-content boundary"
question. These are the facts I could actually source. Reuse instead of re-searching.

## Hevy — REPORTED

- Built-in exercises are **not editable**. The only path is
  `⋯ → Duplicate Exercise`, which produces a *custom* exercise you then rename
  and re-tag. (hevyapp.com/features/custom-exercises, help.hevyapp.com 34264544382615)
- Hevy's own advice for "I want to reset an exercise's stats" is
  *duplicate it to get a new custom exercise with no previous data attached*.
  → history is a **reference** to the exercise row and they will not rewrite it.
  (help.hevyapp.com 35119576252951 "Reset Data")
- End-of-workout prompt **"Update Routine vs Keep Original Routine"** appears
  ONLY for structural change — reordering exercises, adding/removing exercises
  or sets. **It does not appear for reps or weight changes.**
  (help.hevyapp.com 38387296276375)
- Numbers are governed separately: save screen → `Routine Settings` →
  **Update Routine Values** toggle. And a *separate* setting chooses whether a
  run prefills from **previous workout values** or **routine values**.
  (help.hevyapp.com 34105442929943)
  → three independent strata, all user-visible: definition, structure, numbers.

## Strong — REPORTED

- Finishing a deviated workout offers **four** options, not two:
  `Update Template` / `Update Values Only` / `Update Template and Values` /
  `Keep Original Template`. Destructive variants shown in red.
  (help.strongapp.io/article/177) → the app itself models numbers as a
  different stratum from exercise list.
- Built-ins are **hidden, not deleted**. Recovery route for a removed exercise
  is *History → find a workout that used it → Edit → **Restore Exercise***.
  (help.strongapp.io/article/99, /article/250)
  → soft-delete + history as the durable index into the library.

## TrainingPeaks — REPORTED (article itself 403'd, snippet only)

- "Updating an existing plan will update it for all athletes that have already
  purchased or applied the plan." (help 204715504 — snippet, unverified body)
- **Dynamic Training Plans** push live to every athlete's calendar, but only
  for **today or future dates — never past dates**. (help 204072414)
  → an explicit immutability line drawn at "now".

## Garmin Connect — REPORTED

- Adaptive **Garmin Coach** plans cannot be edited at all; only *Self-Guided*
  plans are editable. Editing a scheduled workout does not touch completed
  activities — a completed Activity is a separate record from the planned
  Workout. (support.garmin.com; forums 431980)
  → cost of the clean split is a weak plan↔actual link; users ask on the forum
  how to attach a run to the plan at all.

## Crimpd+ — REPORTED

- Plan builder assembles from ~200 workouts + Skill Templates. **Weekly volume
  is edited in the plan** (Edit icon beside the workout name), not on the
  workout definition. Logs can be edited, deleted and **cloned**.
  (crimpd.com/docs/build-a-training-plan)

## MacroFactor vs MyFitnessPal — REPORTED (the anti-pattern citation)

- MyFitnessPal: ~20.5M unverified user-generated entries. "A single banana
  appearing ten times with different calories and macros"; popular foods
  accumulate dozens of near-identical entries "and nothing indicates which one
  the last 10,000 people used correctly."
- MacroFactor: ~1.15–1.36M items, from vetted research databases plus user
  submissions **a human reviewer cleared before publishing**.
  → the fix for namespace rot is a review gate, not a bigger database.
  (macrofactor.com/macrofactor-vs-myfitnesspal-2025)

## The convergence

Five of five apps put real numbers **downstream** of the definition, and two
(Hevy, Strong) surface that split as separate words in the UI. Nobody ships a
version-reconciliation / migration UI. Nobody forks on edit.

## Still INFERRED, not sourced

- Whether renaming a custom exercise retroactively renames old logs (follows
  from Strong's Restore Exercise, but no doc says it).
- Whether MacroFactor snapshots macros into a log entry.
- Tension/Kilter internals (stable problem id + separate user ticks +
  grade aggregated per angle rather than authored).
- That "restore to default" fails specifically because fork-on-edit leaves no
  default row to restore to. Mechanism is sound; no complaint cited.
