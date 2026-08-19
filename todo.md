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

- [ ] **Per-day drill assignment** — the open question. 72 drills catalogued by source;
      24 referenced by nothing. Notably: the five `Kettle: *` menus are referenced by zero
      sessions, and 16 of 20 Power Company drills are unused. Proposal on the table was
      four day-specific menus plus a `Flash Tactics` menu in the board day's *main block*
      (`Flash Execution`, `False Start`, `One Size Fits All`, `Creative Repeats`,
      `Mirrored Repeat`, `3 Strike Repeat` — all unused, all serve the flash goal).
- [ ] **Author the missing exercises** — `Traverse Circuit` (timed_reps, 3×3:00 → 6:00),
      `Fall Practice`, `Easy Lead Mileage`, `Mock Lead (Auto-Belay)`, and optionally
      `Redpoint Lead Attempts`. Nothing else is missing — the four drills the owner
      described are all already in the catalog.
- [ ] **Build the session templates** — Endurance, Lead, Strength & Stretching. Limit
      Bouldering already exists and needs no changes. Wire `Antagonist & Prehab` (built,
      referenced by nothing) into the strength day, and add a `Cooldown Stretch` template
      around the existing stretches.
- [ ] **Use the Ondra warm-up** — `Synovial & Fascia Warm-Up` is now one attributed
      activity template but is referenced by no session. It is the natural joint-prep
      block for the new sessions.
- [ ] Then create the cycle itself with the guided builder.

## Catalog hygiene (found during the attribution sweep)

- [ ] **Six session templates inline their exercises instead of `ref:`-ing the library** —
      `Flow & Power`, `Foundations`, `Reading & Sport Tactics`, `Strength Base Session`,
      `Strength Project Day`, `Upper Body Strength Day` define every exercise inline, so
      they inherit no source, notes, or video and drift from the library copy. This breaks
      the Library → Activity → Session hierarchy. Convert to `ref:` where a library entry
      already exists; promote the rest into the library first.
- [ ] **`Taylor's Pose` is probably `Tailor's Pose`** — the anatomical name for that seated
      adductor position. Referenced by nothing, so a rename is free (the prune handles the
      old row).
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
