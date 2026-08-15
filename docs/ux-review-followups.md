# UX Review — Follow-up Plan

Status: **Tiers 1–3 implemented & reviewed; Tier 4 not started.** Derived from the full user-journey audit (library → templates → scheduling → live/manual logging → training log → analytics) plus the Boardsesh-import feasibility question. Items marked ✅ DONE are complete and verified; nothing is committed.

The work splits into four tiers. Tiers 1–2 are contained fixes. Tier 3 surfaces data already captured. Tier 4 is design work with a decision still open (see "Open design decision" at the end). A separately-requested feature (quick notes) is recorded at the end.

---

## Tier 1 — Likely bugs (small, low-risk, verify-then-fix)

### 1.1 Library session duration drops Hours & Seconds — ✅ DONE
- **Confirmed:** the library editors (`edit_exercise_library.html:119-127`, `new_exercise_library.html:150-158`) post three fields `session_duration_hours`/`_minutes`/`_seconds`, but `parseSessionDurationSeconds` read only minutes (e.g. "1h 30m" → 30 min). Every *other* caller posts a single minutes field, so only the library editor lost data.
- **Fixed:** `parseSessionDurationSeconds` (`http/server/templates.go`) now sums H·3600 + M·60 + S, backward-compatible with the minutes-only callers. Tests added in `templates_helpers_test.go`.

### 1.2 `climbing` kind is a UI ghost — ❌ NOT A BUG (audit was wrong)
- **Corrected finding:** `climbing` IS an intentional, tested library kind. See the comment at `exercise_library.go:30` ("it previously omitted 'climbing'") and the regression test `exercise_library_test.go:29` that creates a `climbing`-kind library exercise. It's also handled at `training_log_manual.go:338` and `training_log.go:478`. It correctly falls through to the reps_and_sets branch because it has no numeric fields of its own — climbing data comes from *ticks* at runtime. No change needed; the sub-agent's "UI ghost" claim was a false positive.

---

## Tier 2 — Scheduling correctness (high impact)

### 2.1 Duplicate-run guard on "Start" — ✅ DONE
- **Confirmed:** `handleScheduledSessionsByID` `start` case loaded the scheduled session then unconditionally `Create`d a `SessionRun`. No check for an existing run.
- **Fixed:** the handler now queries for an existing `SessionRun` on (owner_id, scheduled_session_id) ordered by started_at DESC; if one exists it redirects there instead of creating a second. Tests in `scheduled_sessions_test.go` (creates-when-none, no-duplicate, completed-not-rerun).
- **Redirect target:** running run → `/runs/{id}` (resume the live player); completed run → `/runs/{id}/summary`. The summary reflects `run.Status` directly, which sidesteps a subtlety QA surfaced: `renderRun` derives its "complete" screen from step completion, not `run.Status`, so a run finished *early* via "Finish this session?" (status=completed but exercises unchecked) would otherwise render the live player. Routing completed runs to `/summary` avoids that.
- **Known latent gap (not fixed):** two truly-simultaneous first-time Start POSTs could both pass the existence check before either creates, racing to two runs. There's no transaction/unique constraint around check-then-create. Low real-world risk (a single HTMX client won't fire twice in parallel). Proper fix if it ever matters: a DB-level unique guard or a serializable transaction — not a flaky concurrency test.

### 2.2 Done sessions should not show a live "Start" — ✅ DONE
- **Fixed:** the week session card now branches on `.Done`. Done → an always-visible ghost-bordered "View" link to `/runs/{CompletedRunID}/summary`; not-Done → the existing btn-primary "Start" (hover-gating dropped). The completed run's ID is captured in `handleDashboard` (`completedWeekRunID` map) and exposed as `DashboardSession.CompletedRunID`. Running sessions still surface separately in the Active Runs banner, so the card only needs Start-vs-View.

### 2.3 Preview dialog dead-end — ✅ DONE
- **Fixed:** the preview fragment now renders a bottom-footer "Start session" (btn-primary) when `ScheduledSessionID` is set. `TemplateFragmentData` gained that field; the preview handler passes it. Reuses the guarded start path from 2.1.

### 2.4 Orphaned `/scheduled-sessions/add` endpoint + misleading empty-state — ✅ DONE
- **Decision:** wire it up (user chose to keep one-off scheduling).
- **Fixed:** new `templates/fragments/schedule_session_picker.html` dialog (date input + template select, mirrors `start_session_picker.html`). Triggered from the week-header action group AND the empty-state, both `btn-ghost` with a `plus` icon. Embedded in `dashboard.html` via `{{ template }}` (added to `baseFiles` in `pages.go`) and fed by the already-present `DashboardParams.Templates`. Verified end-to-end: schedule → appears with Start → preview Start → finish → card flips to View.
- **Note:** the `add` handler accepts any date (including past). Not restricting it for now — a past date is a legitimate "log something I forgot to schedule" case, and the manual-log flow already handles retroactive entries. Revisit if it causes confusion.

---

## Tier 3 — Surface captured-but-unused data

### 3.1 History run-summary shows no ticks — ✅ DONE
- **Fixed:** `handleRunSummary` now loads each climbing exercise's ticks (new `summaryTickViews` helper, reusing the existing `ticksToViews`) and exposes them on `RunSummaryExercise.Ticks`. A new read-only partial `fragments/run_ticks_readonly.html` renders them (grade/kind/setting/board chips, Sent badge, rope+result style, stars, focus, thoughts) under the exercise row. Wired into BOTH summary surfaces — the HTMX dialog (`fragments/run_summary.html`) and the full page (`run_summary.html`); the partial is registered in both parse sets in `pages.go`. Verified end-to-end on both.
- **Pixel fix applied:** the read-only tick initially reused the full `.card` class, producing a bordered box nested inside the already-bordered summary row/page-card (three nested rectangles). Replaced with a lighter `.run-summary-tick` treatment (muted fill + `tick-card--sent`/`--working` left accent only, no full border) per DESIGN.md's "list entry" guidance.

### 3.2 `ClimbingTick.RPE` is dead both ways — ✅ DONE (dropped)
- **Decision:** dropped the column (user choice). Session-level RPE already exists on `SessionJournal` with input + display + averaging, so per-climb RPE was redundant clutter for unused signal.
- **Fixed:** removed `ClimbingTick.RPE` (`db/models.go`). GORM `AutoMigrate` never drops columns, so the orphan `rpe` column remains harmless in existing DBs (it was never populated by any UI); new DBs won't create it. `SessionJournal.RPE` and all journal/manual-log RPE handling untouched.
- **Schema review:** confirmed safe. GORM maps by column name (never positional / never `SELECT *`), every `ClimbingTick` query is struct-based, and `UpdateClimbingTick` uses an explicit key map without `rpe` — so the orphan column can't be read or written accidentally. Optional future hygiene: a guarded `ALTER TABLE climbing_ticks DROP COLUMN rpe` following the `session_journals` precedent in `db/store.go:95-104` — deliberately NOT done, to avoid making the first exception to the AutoMigrate-only convention for a cosmetic cleanup on a small table.

### 3.3 Dead computed fields (cleanup) — ✅ DONE
- **Fixed:** removed `ClimbingTickSummary.MinGrade/MaxGrade` (defined + populated in `db/queries.go`, never read — and the population used broken lexical grade comparison anyway) and `MostUsedTemplate/MostUsedColor` (computed in `history.go`, never rendered). Kept `templateColors`/`templateCounts` — still used by the template breakdown.

### 3.4 Compact in-card action buttons undershoot the 44px touch target (app-wide)
- **Flagged by pixel during batch 2.** The compact card action buttons (e.g. dashboard Start/View, `px-3 py-1.5 text-xs` with `btn-primary`/`btn-ghost` but no base `.btn` class) resolve to ~26-30px height, under DESIGN.md's 44px/`2.75rem` touch-target rule (`static/passion.css:278`, checklist item 15). This is a **pre-existing, app-wide pattern**, not introduced by batch 2 — it applies equally to the old Start button.
- **Decision:** left as-is for now (user call). Fixing only some buttons would make them inconsistent with the rest.
- **Fix (if pursued):** decide whether compact in-card actions are an intentional exception (like chips/badges), or add `.btn` to the pattern everywhere as a dedicated consistency pass. Low priority; not a regression.

---

## Tier 4 — Design work (needs discussion, not just implementation)

### 4.1 Copy-by-value vs. live library references — SEE OPEN DECISION BELOW
This is the cross-cutting theme. Every path that adds an exercise to a template deep-copies (`newExerciseFromLibraryExercise` at `templates.go:16-39`, plus the fresh-typed and activity-template-import paths). `LibraryExerciseID` is stored but used only for provenance.

### 4.2 Per-set config consistency across surfaces
- `ExercisePlannedSet` exists only in the session-template activity editor (`reps_and_sets` only). Absent from the library editor, the activity-template editor, and the manual-log path (which uses a parallel `ManualExerciseSetLog`). Not copied by any import path.
- Ties to the deferred per-set-config design discussion.
- **Fix (post-decision):** make per-set config a first-class field group on every editor surface, and copy it in all import paths.

### 4.3 Duplicate editor templates
- `fragments/exercises_container.html` vs `fragments/activity_template_exercises_container.html` are near-duplicates that have drifted (the AT editor lost media, per-set, save-to-library, catalog). Kind menus vary 5/4/3/2 across surfaces.
- **Fix:** extract shared partials; single kind source-of-truth. This is a prerequisite for making 4.1/4.2 consistent everywhere without N-place edits.

### 4.4 History vs Training Log redundancy
- Streak, week/month/total counts, and indoor/outdoor split are each computed twice with different denominators (`history.go` vs `training_log.go`), risking conflicting numbers.
- **Fix:** extract shared helpers; pick one canonical denominator and one indoor/outdoor source.

### 4.5 Trend charting (biggest analytics gap)
- Volume has a heatmap + weekly bars, but there is no grade-over-time trend and no RPE/Sleep/Energy time-series — only static pyramids and scalar averages.
- **Fix (larger):** weekly hardest-sent + volume series; weekly wellness series from `SessionJournal`. Use the `dataviz` skill for the chart work.

### 4.6 Boardsesh import (optional feature)
- Feasible; Boardsesh exports per-board logbook JSON (`AuroraJsonExport`: ascents/attempts with climb name, angle, grade, stars, count, climbed_at) that maps cleanly onto `ClimbingTick`.
- **Prerequisites:** add an `Angle` column to `ClimbingTick` (Passion has none today); rescale stars 1–5 → 0–3; group ascents by `climbed_at` into manual `SessionRun`s.
- **Scope:** schema change + JSON importer + upload UI or CLI command. Not a config toggle.

---

## Open design decision — the freeze-on-start boundary

The user's stated expectation: **the library is the source of truth — edit an exercise once and it flows everywhere.** That's a live-reference model (not the current copy-by-value).

The hazard: templates and *past logged sessions* currently share the same copied-value shape. Live propagation would retroactively rewrite what already-completed sessions say you *planned* to do, corrupting the training history analytics are built on.

**Recommended resolution:** templates (and future/unstarted scheduled sessions) hold **live references** to library exercises; a `SessionRun` **snapshots** the resolved values at the moment it starts. Edit the library → all plans update; already-run sessions stay frozen and honest.

**This boundary is the key thing to confirm before any Tier 4 implementation.** It determines the data model for 4.1, and 4.3 (unified editors) should land first as the foundation.

---

## Suggested sequencing
1. Tier 1 (verify + fix the two bugs) — independent, quick.
2. Tier 2 (scheduling correctness) — 2.1 first; 2.2/2.3 depend on it; 2.4 needs the one-off-session decision.
3. Tier 3.1 (ticks in run summary) — self-contained, high user value.
4. Confirm the freeze-on-start boundary, then Tier 4.3 (unify editors) → 4.1/4.2 → 4.4/4.5.
5. Tier 4.6 (Boardsesh) whenever desired; independent of the rest.

---

## Separately requested — Quick notes ✅ DONE

Not part of the original audit tiers; requested directly. Goal: let a user write a standalone log entry with no session attached ("I went climbing, didn't do a session, just want thoughts on paper").

- **Finding:** the data model already supported this — `SessionJournal.RunID` is a nullable `*uint`, the training log already splits and renders standalone (non-run) journals, and the edit/delete paths already special-case `RunID == nil`. The only gap was a lightweight *entry point*; the sole existing "Add entry" button (`/training-log/new`) always force-creates a draft `SessionRun` with the full session-log machinery.
- **Implemented:** new `GET|POST /training-log/quick` (`handleTrainingLogQuick` in `training_log_manual.go`, route in `core.go`). Minimal form — date + optional title + free-text notes (→ `SessionJournal.WentWell`), `RunID=nil`. New `pages.TrainingLogQuickParams` + `TrainingLogQuickPage`; template `training_log_quick.html`. A "Quick note" button (btn-ghost, pencil-line icon) sits next to the existing "Add entry" (btn-primary) in the training log header.
- **Reuses existing infra:** the created standalone journal interleaves in the log, and is editable/deletable via the existing `/training-log/{id}/edit` and `/delete` paths — no new edit/delete/render code.
- **Verified end-to-end:** button → form → save → redirect to entry view → appears in log; empty-notes rejected; entry confirmed standalone (editable title/date, not run-linked).
