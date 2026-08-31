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

## DONE (2026-08-27): the cycle page rework, all five phases

Bugs (Phase 0): guided start date + next-Monday default, the missing blocking-event
conflict check, varies-by-week zeroing exercise targets, and calendar events orphaned by
a cycle delete. af945d6, 19c641f, 66f6420, e8384eb.

The page (Phases 2-4): goals read at the top instead of collapsed behind "cycle
details"; notes below the grid; exercise targets on their own page at
`/training-cycles/{id}/targets`, reached from the header and from the end of the builder
so they stay configurable at creation; a real progress line ("Week 2 of 4 · 6 of 8 done
· 2 left this week"); ticks on completed sessions and dim on missed ones; and a phone
layout that went from 4077px to 1261px — one week open at a time, rest days on one line,
one Add button per week, names that wrap. 90c5908, 588ddcd, c199777, 9d52a41, 272b63b.

Two traps worth remembering. `details-save` used to overwrite every metadata field and
hard-delete all goal rows on any submit, so splitting the panel into separate forms would
have deleted the goals the first time notes were edited — guarded with `Form.Has`, tests
in `training_cycles_test.go`. And the `keyup changed` autosave trigger had never fired,
because `changed` compares the value of the element carrying `hx-trigger` and a form has
none; notes only saved on blur.

## DONE (2026-08-27): cycle creation consolidated onto the guided builder

Superseded the earlier "bring the guided fields to `/training-cycles/new`" plan — the
decision went the other way: retire the manual page rather than bring it to parity.
`/training-cycles/new` now 302s to `/new/guided`; `new_cycle.html` is deleted.
Equipment and per-day Effort were deleted (nothing read them); `label` and `notes` are
real fields. Shipped in 4abc246, with the four Phase 0 bugs in af945d6, 19c641f,
66f6420, e8384eb. Remaining cycle work is Phases 2-4, planned in
`scratchpad/cycle_plan_phase234.md`.

## PLAN: import BoardSesh sessions into Passion

Raised 2026-08-31. Board sessions are logged in BoardSesh; the goal is to pull them in
as Passion sessions rather than re-typing them.

**Feasible.** BoardSesh has a real user data export (verified in
`github.com/boardsesh/boardsesh`, `packages/backend/src/handlers/user-data-export.ts`
and `services/user-data-export-format.ts`):

| Endpoint | Purpose |
|---|---|
| `POST /api/user-data-export?boardType=kilter` | request generation |
| `GET /api/user-data-export` | status — 202 while generating, 501 if unavailable |
| `GET /api/user-data-export/download` | the JSON |

Auth is `Authorization: Bearer <token>`, validated by `validateToken`. Note the public
docs at boardsesh.com/docs mention only NextAuth session cookies — the code shows a
bearer path on these endpoints, so trust the code.

### Field mapping onto `ClimbingTick`

| BoardSesh | Passion | Note |
|---|---|---|
| `status` (`flash`/`send`/`attempt`) | `Style` + `Sent` | their three-way split matches ours |
| `attemptCount` / `count` | `Attempts` | |
| `stars` / `quality` | `Stars` | already 0–3, nullable there |
| `grade` / `difficultyName` | `Grade` | `difficulty` is nullable by design |
| `climbed_at` | the run's date | |
| `climb` (name) | **missing** | needs a column |
| `angle` | **missing** | needs a column |

Board climbs are already `Kind: boulder`, `Setting: indoor`, `Subtype: board` here, and
`ClimbingExerciseMeta.BoardKind` can carry kilter/tension.

### Steps

- [ ] 1. **Resolve the open question first: how to get a bearer token outside their app.**
      Everything else is wasted if there is no non-interactive way to obtain one. If there
      isn't, file a feature request upstream — it is open source, and a personal access
      token is the durable answer.
- [ ] 2. Add two columns to `ClimbingTick`: a climb name and an angle. Additive and
      nullable; every other tick source leaves them empty. `schema` review before landing.
- [ ] 3. Read `docs/ascents-and-attempts.md` in their repo before choosing the grouping.
      They have their own Sessions concept; decide whether one Passion session is one day
      or their session. Their ticks are deliberately **not** deduplicated, so several goes
      on one problem must stay separate rows.
- [ ] 4. Parser + importer. Ticks need a `RunID` and an `ExerciseID`, so an import creates
      a manual `SessionRun` per group plus one climbing exercise to hang ticks on — the
      manual-log path (`is_manual`, `is_draft`) already does exactly this, so reuse it.
- [ ] 5. Import screen with a dry-run preview, and a dedupe key so importing twice does
      not double the logbook. Candidate key: (climbed_at, climb name, angle, board).
      There is no CSV or file-import code anywhere in Passion yet — this is a new surface.

### Rejected alternatives

- **BoardLib CSV** (`boardlib logbook kilter --username=... --output=...`). Talks to
  Aurora, whose Kilter backend Aurora shut down on 25 March; BoardSesh's Aurora proxy
  endpoints 404 from 2026-10-01. Do not build on it.
- **Storing BoardSesh credentials and reusing a session cookie.** Works until they touch
  NextAuth or CSRF, and puts a password in this app. Not worth it.
- **Aurora's emailed JSON export.** One-off, and only useful for pre-shutdown history.

## PLAN: richer History metrics

Raised 2026-08-27, deferred from the mobile-bug pass because it needs design, not patching.

Today `/history` shows: Activity heatmap, Weekly trend, By template, Climbing (grade
pyramid + per-discipline), Workouts. Computed in `http/server/history.go` (`handleHistory`,
`buildClimbingView`, `computeStreaks`).

**Steps**

- [ ] 1. Decide the questions the page must answer before choosing charts. Candidates:
      "am I training more or less than last block", "which grade am I converting",
      "is volume drifting toward one discipline", "how much of the plan did I actually do".
      Pick 4–5; everything else is noise on a phone.
- [ ] 2. Audit what is already derivable with no schema change. Ticks carry grade, style,
      attempts, sent, stars, duration, setting and subtype — that is enough for send rate,
      attempts-to-send, flash rate, indoor/outdoor split and quality distribution, none of
      which the page shows today.
- [ ] 3. Adherence needs the plan side: `ScheduledSession` vs `SessionRun` per week gives
      planned-vs-completed, and the skipped-step completions now recorded give
      per-session completion. No schema change needed.
- [ ] 4. Consult `scout` for how Crimpd / Lattice / Strong present training history before
      designing, and `pixel` for chart-in-a-card patterns at 390px.
- [ ] 5. Only then build, one metric per commit, each verified at 390 / 900 / 1440.

**Constraint from experience:** the exercise-library redesign took eight rounds because the
layout was chosen before the questions were settled. Settle step 1 in writing first.

## PLAN: comprehensive consistency review, desktop and mobile separately

Raised 2026-08-27: "there's a lot of small inconsistencies I keep finding all the time".
The pattern so far is that each one is found by hand, in production, on a phone. The goal
is to find them mechanically instead.

**Steps**

- [ ] 1. Build an automated sweep as a checked-in script (not a scratch file): walk every
      route at 375 / 390 / 768 / 900 / 1440 against a seeded DB and assert
      **page-level invariants** — no horizontal overflow, no element escaping the viewport,
      no wrapped button label, every tap target ≥ 44px, no clipped text (`scrollWidth >
      clientWidth`) outside declared scroll containers. Several of these already exist as
      throwaway probes; the value is in keeping them.
- [ ] 2. Seed fixtures already carry deliberately long names and 7-label rows — every
      layout bug found so far only reproduced with content like that. Keep the sweep
      pointed at that DB.
- [ ] 3. Separate desktop and mobile passes, as asked: mobile checks stacking, tap targets
      and single-column behaviour; desktop checks column alignment, table widths and
      whether space is used.
- [ ] 4. Then a **design-consistency** pass, which automation cannot do: dispatch `pixel`
      over the template tree for one-off patterns — heading levels that differ per page,
      chips vs plain text for the same field, action placement, empty-state wording,
      filter-bar shape. Feed the findings into `docs/DESIGN.md` as rules, so the next
      page inherits them instead of re-litigating.
- [ ] 5. Known undocumented patterns to write into DESIGN.md while doing this: the
      `md:`-table / mobile-card split layout, `--container-pad` as the single source of
      the page gutter, `min-width: 0` on every grid/flex child that holds text, and
      "labels render as one muted `·`-separated line, never as chips".

**Recurring root cause worth writing down:** four separate overflow bugs this week were
all the same thing — a grid or flex child with the default `min-width: auto` that cannot
shrink below its content. Any new grid column holding user text needs `minmax(0, 1fr)` or
`min-width: 0`.

## PLAN: give light mode a real ground-to-panel step (site-wide)

Sections are hard to tell apart in light mode. Measured on the shipped palette, not
guessed:

| Pair | Contrast ratio |
|---|---|
| `--bg` `#f9fafb` vs `--panel` `#ffffff` | **1.045** — visually identical |
| `--bg` vs `--card-muted` `#f9fafb` | byte-identical |
| `--border` `#e5e7eb` vs `--bg` | 1.185 |
| dark mode `--bg` `#161f2e` vs `--panel` `#1e2a3a` | 1.14 |

So in light mode a card is separated from the page only by its 1px border, and a
`.card-muted` panel sitting on the page ground is invisible. Dark mode has a real step;
light mode never did. Pre-existing, not a regression — confirmed no colour token has been
touched (`git diff` over the 2026-08-27 commits changes one breakpoint and nothing else).

Ground candidates, all keeping `--panel: #ffffff`:

| Ground | vs panel | Note |
|---|---|---|
| `#f4f6f8` | 1.083 | barely more than today |
| `#f1f3f5` | 1.112 | ≈ dark mode's step |
| `#eaedf1` | 1.174 | clear separation |
| `#e3e7ec` | 1.242 | strong; border then vanishes into the ground (1.003) |

- [ ] Pick a ground. `--bg` is used 3× in CSS and 0× in templates, so it is a safe change.
- [ ] `--card-muted` needs its own value regardless. It is used **108×** (52 CSS, 56
      templates), mostly as an inset tint *inside* white cards, where `#f9fafb` works —
      so it may need no change once the ground moves away from it. Check both contexts:
      inset on white, and standalone on the ground.
- [ ] Darkening the ground weakens `--border` against it. Either restrengthen the border
      or accept it, but decide deliberately — at `#e3e7ec` the border is gone.
- [ ] Verify light *and* dark after: the tokens are redefined in a `prefers-color-scheme`
      block and again under `html[data-theme="dark"]`, so a change in `:root` alone can
      silently apply to only one of the three theme states.
- [ ] Sweep the real pages at 390/768/1024/1440 in both themes; the offender that
      surfaced this was the cycle detail page, where every section is a white card.

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
