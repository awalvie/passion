# Passion — TODO

Running list of outstanding work. Newest planning at top.

## NEXT: harden the deploy (agreed 2026-08-19, after a 224-restart outage)

Context: grouping `catalog/exercises/` into program folders took production down. `scp -r`
never deletes, so the old flat files stayed on the server alongside the new subfolders —
both declaring the same exercise names — the importer rejected the duplicates and startup
exited 1 into a restart loop. Recovery took three attempts because modern (SFTP-backed)
`scp` also refuses to create nested destination directories.

Only the copy mechanism is fixed so far (`337817c`, tar stream + wholesale replace). Four
weaknesses remain, to be done in this order:

- [ ] **1. Release directories + symlink flip** — atomicity and rollback.
      ```
      /opt/passion/releases/<sha>/{passion, catalog/, templates/, static/}
      /opt/passion/current -> releases/<sha>
      /opt/passion/passion.yaml   # config stays outside releases
      /opt/passion/passion.db     # data stays outside releases
      ```
      systemd runs `/opt/passion/current/passion` with `WorkingDirectory=/opt/passion/current`.
      Deploy = upload a fresh release dir, flip the symlink, restart. Keep the last 5.
      Structurally removes stale files, binary/asset mismatch, and `rm -rf` on live state.
- [ ] **2. Health gate + auto-rollback** — poll for HTTP 200 for ~30s after restart; on
      failure flip the symlink back, restart, and fail the workflow. Never stay down.
- [ ] **3. Validate the catalog in CI** — run the real importer against `catalog/` into a
      temp DB and fail on duplicate names, unresolved `ref:`, or parse errors. This check
      currently only happens in production, at startup.
- [ ] **4. Import failure stops being fatal** — on import error with a non-empty DB, log
      ERROR, skip the import and keep serving the existing catalog. Stay fatal only when
      the DB is empty. Containment for when 1–3 miss something.

Deliberately not doing: bounding the systemd restart loop. With auto-rollback it is
redundant, and it only converts "down and retrying" into "down and stopped".

## In progress: first training cycle (Cycle 1)

Plan doc (living): the Cycle One artifact — 4 weeks, 4 days/week, Bechtel nonlinear
rotation. Decisions already made: 4 training days, weekly lead access, 4-week wave
(3 load + 1 deload), variable session length, sub-max hangs only this cycle, cycle target
flash 6c on TB2 (7a is the season goal), fall practice leads the lead day.

- [x] **Per-day drill assignment** — settled 2026-08-24 after a deep research pass
      (55+ sources). Structure: one **daily constant** carrying the learning load in
      every session's warm-up (Gresham's five-rung ladder, `Movement Practice`), plus
      one **per-day drill** matched to that day's terrain. Rationale: four parallel
      drills would give each only 4 exposures across 16 sessions, below Bechtel's
      8-10 threshold — the constant gets 16.
      Key evidence: contextual interference does not survive contact with applied
      sport (rotation is not justified); fatigue degrades motor *learning* for ~2
      subsequent sessions (so skill work goes first, always); stacked-constraint
      drills are integration tests, not lessons.
- [x] **Author the missing exercises** — `Traverse Circuit`, `Fall Practice`,
      `Easy Lead Mileage`, `Mock Lead (Auto-Belay)`, plus `Fluid Style` and
      `Increase Pace` (Gresham ladder rungs 3 and 4, which the catalog lacked).
      `Redpoint Lead Attempts` deliberately skipped — the lead day is already full
      with fall practice plus mileage.
- [x] **Build the session templates** — `Cycle 1: Limit Bouldering`, `Endurance`,
      `Lead`, `Strength & Stretching`. Built as new `cycle1_*` templates rather than
      editing `Limit Bouldering`, so the Paradigm-published session keeps its
      attribution intact. Added `Cooldown Stretch` and `Movement Practice` activity
      templates.
- [x] **Use the Ondra warm-up** — `Synovial & Fascia Warm-Up` now opens the
      Strength & Stretching day. `Antagonist & Prehab` is wired into the same day.
      Both were referenced by nothing before.
- [ ] Then create the cycle itself with the guided builder.

## Catalog hygiene (found during the attribution sweep)

- [ ] **`Final Exam I` and `Final Exam II` are not Kettle's names** — verified via a
      channel-scoped search of his own YouTube (returns no content for "exam", while
      controls "big three" and "sloth" correctly return `17 The Big Three` and
      `46 The Sloth`). The structure is real: they are exercises **13 and 14** in the
      1st edition, **24 and 25** in the 2nd, under a section heading "Final
      Challenges". Rename to his numbering or to "Final Challenge I/II" — we are
      currently attributing invented names to a named coach.
- [ ] **Four files have merged-notes damage** from a past dedup pass, found
      independently by two agents. Worst: `power_company/heavy_feet_drill.yaml`
      carries *Silent Feet*'s notes ("completely silent… zero noise"), so the wrong
      drill is live via `a_foundations`. Also `hip_shapes_drill.yaml` (two drills in
      one file), `one_touch_drill.yaml` (contradicts its own scope), and
      `sloth_monkey_drill.yaml` (monkey half undocumented).
- [ ] **`one_leg.yaml` and `single_leg_climbing.yaml` are the same drill** duplicated
      with different kinds (`climbing` vs `session` 480s). Both orphans.
- [ ] **All 36 Kettle drills remain unreachable** — the five `kettle_*` activity
      templates are referenced by no session template. Left that way deliberately:
      Cycle 1 uses Gresham's ladder instead. Worth wiring `kettle_feet` if you want
      `The Big Three` as a week-1 / week-4 technique metric.
- [ ] **No terrain/equipment field in the schema.** Board-only and lead-only drills
      are indistinguishable from commercial-boulder drills by metadata; the
      constraint lives only in prose. This is what made the per-day assignment
      manual. Needs a `schema` review.
- [ ] **Tag vocabulary cannot express three of the five elements** — no tag for
      Rhythm, Commitment, or Effort, which is the structural reason `label:` was
      unreliable for this audit.

- [ ] **`Strength Base Session` inlines a few generic lifts** — `Row + Band Prep`,
      `Ring Rows`, and the `Pull Exercise` catalog (`Lat Pulldown`, `Face Pulls`) are
      defined inline rather than `ref:`-ed. Left inline deliberately: these are generic gym
      lifts, the same class the attribution sweep chose not to promote or source. Revisit
      only if one becomes reusable across sessions. (The other five templates once flagged
      here — `Foundations`, `Flow & Power`, `Reading & Sport Tactics`, `Strength Project
      Day`, `Upper Body Strength Day` — already `ref:` their real exercises; their only
      inline entries are session-specific `Apply:` prompts, which belong inline.)
- [ ] **Legacy seed rows sit alongside catalog entries** — `Max Hangs` (seed) duplicates
      `Max Hangs (20 mm Edge)` (catalog), and `10m Open Mobility` is seed-only. Both are
      unreferenced. Confirm on prod and delete, so the library stops showing two Max Hangs.

## Prod data cleanup

- [ ] **Legacy renamed session templates** — `Limit Bouldering | Paradigm Climbing` and
      `Synovial & Fascia Warm-Up` survive in prod as rows the prune deliberately keeps,
      because they have scheduled sessions and logged runs attached. To remove them,
      repoint that history onto the current templates first.
- [ ] **Orphaned `exercise_media`** — foreign keys are enforced now and new orphans are
      prevented, but rows orphaned by historical template deletes remain. Run
      `PRAGMA foreign_key_check` on a snapshot and clean up.

## Tier 3 — catalog & product polish

- [ ] **Wire the antagonist/prehab set into a session** (referenced by nothing today) and
      expand it: pushing, wrist/finger extensors, scapular work.
- [ ] **Fix prehab metadata** — `external_rotations.yaml` has 5-minute rest and "80% 1RM"
      notes; should be light, ~60–90s. Audit the other prehab lifts for the same.
- [ ] **Add `session_duration_seconds`** to the ~4 untimed warm-up `session` exercises.
- [ ] **Trends over time** — month/cycle-bucketed grade-pyramid drift and send rate, not
      smoothed daily lines. Headline on History, full series on drill-in.
- [ ] **Fix `a_foundations.yaml`** — it sets sets/reps/rep_seconds on `Heavy Feet`, a
      `session`-kind exercise, which the runner silently ignores.

## Guided cycle builder — v2

- [ ] Conflict-detection parity with the manual flow (blocking calendar events).
- [ ] Explicit / future start date (currently assumes "starts now").
- [ ] Intensity-aware spacing — let the user tap which sessions are hard/finger days and
      avoid scheduling two within 48h. v1 is naive round-robin.
- [ ] Session-chip search once the template count grows.
- [ ] Guard `target_date` in the past.

## Deferred

- [ ] **Migration versioning** — a numbered `schema_migrations` table; adopt at the next
      data-transforming migration rather than as a standalone project.
- [ ] **Log template execution errors** — Go aborts a template mid-render and emits partial
      output, which is how three climbs silently rendered as one. `RenderFragment` should
      log the error rather than swallow it.
- [ ] **Shared catalog** — read-only shared catalog + per-user overrides instead of
      copy-per-user. Only worth it if users or catalog grow ~10×. Cheap interim wins:
      a "curated vs mine" chip from `source`, and an `alias`/`previous_names` YAML field so
      renames upsert the same row instead of creating an orphan.

## Explicitly NOT building (off the "doing over logging" ethos)

Social feed / kudos, HRV or readiness scores, nutrition tracking, board-app API sync,
per-set RPE auto-regulation, voice-announcement timers.
