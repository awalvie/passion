# Plan — Faster climbing session logging

## Problem

Logging a route mid-session is too much work. The current per-route form
([templates/fragments/run_ticks.html](../../templates/fragments/run_ticks.html))
shows ~9 fields at once — Type, Setting, Venue/Board, Method, Grade, Status,
Ascent style, Attempts, Thoughts, Stars — and you re-enter them on **every**
burn. But your real notes are mostly *"Rainbow → a line of thought."* Most of
those fields (Type/Setting/Venue/Method) are session constants you shouldn't be
touching route-to-route.

Separately, the model has a `Focus` field (pre-climb intention) that has **no
input in the UI** — yet your notes are built around per-climb intention vs.
post-climb reflection.

## Where this sits in the app

```
SessionTemplate → ScheduledSession → SessionRun → History → (Metrics, future)
                  (or open session,        │
                   or manual log)          └─ climbing Exercise (drill)
                                              └─ ClimbingTick (one per route/burn)  ← we improve this
```

We are only touching the **ClimbingTick** logging surface inside a run.

### Terminology: what a "drill" is

A **drill** is a single `Exercise` row with `Kind == "climbing"` inside one run
— a *named group of routes within one session*, e.g. "Rhythm Practise" or
"Focused Feet". It is **not** a session and never spans sessions; doing the same
drill next week is a new `Exercise` row in next week's run.

```
SessionRun                       ← one session (one gym visit)
 └─ Exercise (Kind="climbing")   ← a "drill"   e.g. "Rhythm Practise"
     └─ ClimbingTick             ← one route/burn   e.g. "Rainbow", "6a"
```

In the app these are generically called "exercises" (the same type covers
pull-ups, hangboard, etc.); "drill" is just the natural word for a climbing one.

## What best-in-class does (research summary)

The market splits in two, with a gap Passion is already aimed at:

- **Logbook apps (KAYA, CrushLog)** — excellent *fast capture*: KAYA
  auto-creates a session on the first logged climb, uses swipe-to-log
  (left = attempt, right = send), and defers optional detail behind icons.
  But they are pure send-trackers: **no intention, focus, reflection, or RPE.**
- **Coaching / journals (Lattice, Climb Strong / Bechtel, Power Company)** —
  rich *intention + reflection*: Lattice advises recording "what I focused on"
  and whether you hit the session aim, plus per-session RPE compared against
  intended intensity. But these are **paper products, not apps.**

**The gap:** nobody combines KAYA-grade fast capture with the paper journals'
intention/reflection loop. Passion's `Focus`/`Thoughts` fields already target
exactly this. The decisions below borrow KAYA's friction model and Lattice's
intention/reflection framing.

## Decisions (locked with the user)

- Capture grade + outcome **and** a thought, with far fewer taps.
- New-route forms **inherit constant fields** from the previously logged tick in
  the same run. The fields that actually live on `ClimbingTick` and can be
  inherited: **Kind, Setting, Subtype (commercial/board), RopeStyle, Grade.**
  (Venue and Board are *not* tick columns — see §3 for where they live.)
- Outcome via a **quick-action row** (Flash / Send / +Attempt / Working);
  derive `Style`/`Sent` server-side. Ascent style only expands to refine.
- **Focus + Thoughts are visible by default** (two compact textareas), *not*
  hidden behind a disclosure — your notes carry both on nearly every climb, so
  hiding them adds a tap to the common case. What collapses instead is the
  inherited **constants** (which genuinely don't change burn-to-burn).
- **Ungraded "Rainbow"/"Traverse": no outcome needed.** When the grade is
  Rainbow or Traverse, hide the outcome row entirely — a technique rep is just
  Focus + Thoughts + Log. The point of those drills is *not* sending.
- Keep the logged data **structured** (not buried in prose) so future analysis
  isn't blocked — but build no metrics/charts now.
- **Inheritance is per-run** (not per-drill): the first logged route of the
  session sets the constants, and every route after inherits — including across
  drills — with a one-tap override. The collapsed constants summary stays
  *visible* (not hidden) so a stale-inherited value is glanceable.
- **"Send" always records `redpoint`** by default; the refine disclosure lets
  you pick repeat/onsight/hangdog the rare time it matters. (No guessing from
  prior ticks — same grade isn't the same route. `redpoint` here means only
  "sent, not a flash".)
- **One row per burn** — each attempt is its own tick with its own
  Focus/Thoughts (per-attempt reflection is the differentiator). A "Log again"
  shortcut keeps repeats fast. Consequently every tick stores `Attempts = 1`;
  attempt *totals* come from counting rows, not a per-tick counter.
- **New-route and edit forms share one model** — the edit form gets the same
  outcome row (explicit chips behind "refine"), per the consistency rule.
- **Add an `RPE int` column now** (`0 = not recorded`, matching
  `SessionJournal.RPE`), with no input UI — reserves the slot for future
  sessions. Input/charts out of scope.

---

## Build now (phased — see "Phasing" at the end)

> **Implementation note (HTMX mechanics).** The tick list is a self-replacing
> fragment. In each host page (run.html ~548, manual_exercises.html ~42) a
> `<section id="tick-list-{ExerciseID}" hx-trigger="load" hx-swap="outerHTML">`
> loads [run_ticks.html](../../templates/fragments/run_ticks.html), whose root is
> a `<div id="tick-list-{ExerciseID}">`. On first load the `<section>` replaces
> *itself* with that div; thereafter every create/update/delete posts with
> `hx-target="#tick-list-{ExerciseID}" hx-swap="outerHTML"` and re-renders the
> whole fragment. So:
> - **Anything that must persist or update across logs lives *inside* the
>   fragment** (run_ticks.html), not on the host-page `<section>`. The
>   `<section>` is gone after first load.
> - The fragment's trailing `<script>` IIFE **re-runs and re-binds on every
>   swap** (re-queries `root` via `getElementById`, guards `if (!root) return`).
>   This is the mechanism the grade strip, scroll, and header rely on — new
>   icons (`trash-2`, `mountain`, outcome icons) need no extra wiring (icon
>   render is already idempotent + double-invoked via base.html's afterSwap).
> - Because both run.html and manual_exercises.html embed the *same* fragment,
>   all fragment-level changes apply to both surfaces for free
>   (CLAUDE.md consistency rule). open_session.html is a session *picker*, not a
>   tick surface — nothing to change there.

### 1. Surface Focus + Thoughts (visible by default)

The `Focus` column already exists on `ClimbingTick`
([db/models.go:409](../../db/models.go)) — no migration. Add a **Focus** textarea
above the existing **Thoughts** textarea on both the new-tick and edit forms.
Both are **always visible** (compact, single-row auto-growing), *not* behind a
disclosure — nearly every climb in the user's notes has both, so hiding them
would add a tap to the common case. The handler already reads `focus` from the
form (`createExerciseTick` / `handleExerciseTickUpdate` in
[climbing_ticks.go](../../http/server/climbing_ticks.go)); just add the inputs.
Keep §4's no-auto-focus rule (don't pop the keyboard on swap).

### 2. Quick-action outcome row (on both new + edit forms)

Replace the separate Status toggle + Ascent style + Attempts groups with one
tap-row, on **both** the new-tick and edit forms (shared model, per the
consistency rule). On edit, pre-select the outcome derived from the stored tick
(`sent && flash → Flash`, `sent && other → Send`, `!sent → Working`).

```
Outcome:  [ ⚡ Flash ]  [ ✓ Send ]  [ + Attempt ]  [ Working ]
          (hidden entirely when grade = Rainbow / Traverse)
```

Server-side inference on create/update (extend `createExerciseTick` and
`handleExerciseTickUpdate`). Every tick is one burn, so `attempts` is always 1:

| Tapped | sent | style | attempts |
|---|---|---|---|
| Flash | true | `flash` | 1 |
| Send | true | `redpoint` | 1 |
| + Attempt | false | `attempt` | 1 |
| Working | false | `` (empty) | 1 |
| *(ungraded Rainbow/Traverse — row hidden)* | false | `` | 1 |

- **Attempts is always 1.** Attempt *totals* per climb come from counting rows
  (`GetClimbingTickSummaryForRun` already counts rows, not the column). The
  per-tick `Attempts` column is near-vestigial under one-row-per-burn; keep it
  only so the edit form can still hand-enter a number if ever needed.
- **Both-present precedence (refine).** The full ascent-style chips stay behind
  a "refine" disclosure. The outcome row sets a **base** (sent + default
  style); if a refine `style` is then chosen it **overrides** style *and
  reconciles* `sent` — onsight/flash/redpoint/repeat ⇒ sent=true,
  hangdog/attempt ⇒ sent=false. Handler computes base from `outcome` first,
  then applies refine style and recomputes `sent` from it. (Avoids the
  split-brain where style comes from one path and sent from the other.)
- **`redpoint` means "sent, not a flash"** — not literally a worked redpoint.
  Accurate onsight/repeat needs the refine tap.

### 3. Inherit constants from the previous tick

When rendering the **new-tick** form, prefill the tick-level constants —
**Kind, Setting, Subtype, RopeStyle, Grade** — from the most recent tick in the
same run, falling back to the user's defaults for the first tick of the run.

- **Source reconciliation (corrected from earlier draft):** Venue and Board are
  *not* ClimbingTick columns. **Kind/Setting/Subtype/RopeStyle/Grade** come from
  the latest `ClimbingTick`. **Board** (BoardID/BoardKind) lives on
  `ClimbingExerciseMeta` (per-exercise). **Venue** lives on
  `SessionJournal.VenueID` (per-run). "Method" is just the display label for
  `RopeStyle` — use the real field name. The inheritance query covers only the
  tick-level five; Board/Venue are read from their own sources (Board per-run =
  most-recent exercise's meta; Venue from the run's journal).
- **Query:** add `GetLatestClimbingTickInRun(gdb, ownerID, runID)` in
  [db/queries.go](../../db/queries.go), ordered **`created_at DESC, id DESC`**
  (NOT `OrderIndex` — that's assigned per-exercise via `MAX(order_index)+1` and
  collides across drills, so it can't order a cross-run query). The `run_id`
  filter is index-backed; the ordering is a filesort over the run's ~tens of
  ticks — negligible at session scale, no new index needed.
- `serveExerciseTicks` passes inherited values into `ExerciseTicksParams`; the
  template seeds the new-form chips/grade from them.
- **Collapse the constants** behind a single editable summary line
  (`Indoor · Lead ▸`) that expands to the chips. The summary line **stays
  visible** so a stale-inherited value is glanceable (the mixed-discipline case
  — auto-belay warmup → campus board in one session — relies on this + the
  one-tap override). Default visible form: **constants summary · Grade · Outcome
  · Focus · Thoughts.**
- **Edge case:** first tick of a run → user defaults (boulder + grade systems).

### 3a. Add the `RPE` column (no UI)

Add `RPE int` (`// 1–10 perceived exertion; 0 = not recorded`) to `ClimbingTick`
([db/models.go](../../db/models.go)) via auto-migration — **non-pointer int with
0-sentinel, matching `SessionJournal.RPE`** (db/models.go:363) so future read
sites reuse the identical `.RPE > 0` gating. No input, no display, no charts;
this only reserves the slot. GORM `ALTER TABLE ADD COLUMN` is safe on SQLite.

### 3b. "Log again" shortcut for repeats

Repeats stay **one row per burn**. To keep them fast, add a **"Log again"**
action on each logged tick that **re-seeds the single canonical new-tick form
server-side** from that tick's constants + grade (outcome + notes blank), then
scrolls to it.

- **Re-seed, don't spawn.** Implement as an `hx-post` that returns the
  re-rendered fragment with the bottom new-form pre-filled (same
  `hx-target="#tick-list-{ExerciseID}" hx-swap="outerHTML"` pattern as
  delete/update). Do **not** render a second new-tick form — all new-form field
  ids are keyed by `{ExerciseID}` (not tick id), so a second concurrent form
  collides on `getElementById` (steppers/stars/hidden inputs would target the
  wrong form).
- **Owner-scoped fetch.** Add `GetClimbingTick(gdb, ownerID, runID, tickID)`
  filtering `owner_id = ? AND run_id = ? AND id = ?` — the tick id is in the DOM
  and attacker-controllable; a naive `First(&t, id)` is an IDOR. Reuse the
  prevailing owner-scoping pattern from Delete/Update.

### 4. UI/UX polish

- **Grade chip strip** — replace the grade `<select>` with a horizontal,
  scrollable chip strip (reuse `.tick-style-toggle` pill style; data from
  `GRADE_LISTS`). Hidden input carries the value. **Populate it through the
  existing `populateGradeSelect` path** called from `syncTickKind` (so it
  rebuilds on kind change: boulder→font/v_scale vs route→french/yds), preserve
  the `data-current-grade` restore, and **clear the selection when the prior
  grade isn't in the new system's list** (the `<select>` got this for free).
  On render, **auto-scroll the selected/inherited chip into view**
  (`scrollIntoView({inline:'center'})`) — without this the inheritance win is
  lost for mid-scale grades that render off-screen. Each chip meets the 44px
  touch minimum (so a horizontal scroll is expected — that's fine; the
  inherited grade is pre-selected, so the common path needs no grade tap).
- **Rainbow / Traverse** — prepend `'Rainbow'`, `'Traverse'` to the grade strip
  (kind-agnostic; shown in every system, never filtered out on kind change).
  Stored in the existing `Grade` string — no model/kind change. **When selected,
  hide the outcome row entirely** (per the decision): the tick stores
  sent=false, style="", and the flow is grade → focus → thought → Log.
- **Live session header** — render a thin one-liner **inside the fragment**
  (first child of the `#tick-list` div) so it re-renders on every swap and stays
  current: `5 climbs · 3 sent · hardest 6a`. **Not** sticky (a static line that
  scrolls with the list — avoids colliding with the run player's bottom
  `.run-sticky-controls` and the top `.site-header`; introduces no new sticky
  pattern). **Hardest grade must NOT use `MaxGrade`** (lexicographic string
  compare: `6a > 10a`, `Traverse` beats all) — compute it from the
  `GRADE_LISTS` index order, **excluding Rainbow/Traverse**. Exclude
  ungraded/neutral ticks from the "sent" count too.
- **Scroll + quiet confirmation** — do the scroll **from inside the fragment's
  IIFE** (which re-runs on every swap), not via `hx-on::after-swap` on the
  `<section>` (that element is destroyed by its own swap and never fires).
  After re-init, scroll the new-tick form into view; gate it so the initial
  empty load doesn't yank. Optional `navigator.vibrate(10)`. **No toast** (covers
  the next tap target), **no auto-focus** on textareas, **stay put** (don't
  auto-advance).
- **Preserve open edit `<details>` across swaps** — every tick is a `<details>`
  whose open state is DOM-only, so any swap collapses an open editor (incl.
  saving an edit). Give each `<details>` a stable `id="tick-details-{ID}"` and,
  in the IIFE, capture the set of open ids before re-render and re-apply `open`
  after (via a data-attr on the root or sessionStorage keyed by exercise).
- **Empty state** — first climb shows a centered muted prompt (`mountain` icon +
  "No climbs logged yet") instead of blank, per DESIGN.md.
- **Touch targets** — delete button (~28px) and toggle pills (~18px tall) are
  under 44px. Enlarge pill padding + parent-row `min-height`; icon buttons to
  `min 2.75rem`.
- **Consistency fixes** — delete icon `x` → `trash-2` (and sweep the other
  delete affordances on these surfaces — manual_exercises.html, set-log rows —
  or explicitly scope to just the tick delete; clear with pixel); add
  `select.input { -webkit-appearance: none }` for iOS; thoughts preview in the
  tick summary `--text` → `muted`.

### 5. Bug fix (independent) — dark-mode tick colours

Tick badge colours and card border accents in
[static/passion.css](../../static/passion.css) (~3437–3503: `.tick-card--sent`,
`.tick-card--working`, `.tick-sent-badge`, `.tick-style--*`,
`.tick-rope-style--*`, star colour) are hardcoded light-mode hex — the "sent"
badge is blinding and the star invisible in dark theme. Switch to CSS variables
with `:root` (light) defaults and `[data-theme="dark"]` overrides. **A current
bug; ships first, standalone.**

### 6. Resulting per-route flow

- **Graded route:** constants inherited (0 taps) → grade pre-selected, adjust if
  needed → tap outcome → type focus + thought → Log.
- **Rainbow/Traverse drill rep (the dominant case):** tap "Log again" (or pick
  Rainbow) → outcome row hidden → type focus → type thought → Log. **~4 taps,
  reflection intact.**

### Scope boundary

This plan touches **only the climbing-tick editor**. It does **not** address the
deferred per-set exercise config (separate table + UI), and does not pre-empt
that design discussion.

---

## Plan for the future (do NOT build now)

Goal: don't lock the data into a shape that's painful to analyze later. These
are cheap to leave room for now; the analysis itself is out of scope.

- **RPE input + analysis** — the `RPE int` column is added in this build (§3a)
  but has **no input or display yet**. A future build adds the 1–10 picker
  (per-tick or per-run, TBD) and the intended-vs-actual review loop that
  Lattice/Bechtel describe. That review loop is the genuinely novel feature.
- **Numerically-sortable grade** — grades are stored as display strings
  (`"6a+"`, `"V4"`). Any future grade-trend view needs a numeric ordering
  across font/V/french/YDS. The grade *lists* already exist in JS
  (`GRADE_LISTS` in run_ticks.html); a normalized index could be derived later
  without re-logging. No schema change forced now.
- **Session rollups** — total attempts, sends, hardest grade, volume per run
  are all derivable from existing ticks via a query layer when a metrics
  surface is built. Nothing to store now.
- **Drill / focus-area tagging** — the research found no app tags climbs to
  drills (KAYA's "movement tags" describe style, not training intent). Passion's
  Exercise-as-drill structure already does this implicitly (ticks belong to a
  drill). A future "review intention vs. outcome across sessions" view is the
  genuinely novel, undigitized feature — noted as direction, not scope.

**Explicitly out of scope for this build:** any chart, metric, RPE *input* or
display, grade-index migration, or review dashboard. The build is the faster
logging flow plus the empty `RPE` column only.

---

## Files touched (build-now)

| File | Change |
|---|---|
| [templates/fragments/run_ticks.html](../../templates/fragments/run_ticks.html) | Focus + Thoughts visible (both forms); outcome row + refine on both forms; hide outcome for Rainbow/Traverse; collapsed-but-visible constants summary; seed new-form from inherited values; grade chip strip (replace `<select>`, via `populateGradeSelect` path, auto-scroll selected chip); Rainbow/Traverse grade options; live session header (inside fragment); scroll-via-IIFE + no-toast confirm; preserve open `<details>` across swaps; empty state; bigger touch targets; `trash-2` icon; muted thoughts preview |
| [http/server/climbing_ticks.go](../../http/server/climbing_ticks.go) | Accept `outcome` + `focus` on create **and** update; derive base from outcome then refine-override style + reconcile sent; pass inherited values + session-header totals into `ExerciseTicksParams`; "Log again" re-seed handler |
| [db/queries.go](../../db/queries.go) | `GetLatestClimbingTickInRun` (order by `created_at DESC, id DESC`); owner-scoped `GetClimbingTick(ownerID, runID, tickID)`; session-header aggregate (count / sends / hardest via GRADE_LISTS index, excl. ungraded) |
| [db/models.go](../../db/models.go) | Add `RPE int` (`0 = not recorded`) to `ClimbingTick` (no UI) |
| [static/passion.css](../../static/passion.css) | Dark-mode tick colour variables (bug fix); touch-target sizing; `select.input` appearance |
| pages params struct (`ExerciseTicksParams`) | Inherited-defaults fields; session-header totals; "log again" seed source |

> The host pages (run.html ~548, manual_exercises.html ~42) need **no edits** —
> the header + scroll + confirm all live inside the shared fragment, so both
> surfaces get them automatically.

## Post-implementation review (per CLAUDE.md)

- **pixel** — constants summary line, outcome row, grade strip, session header,
  touch targets, dark-mode variables
- **qa** — outcome→sent/style inference incl. both-present refine precedence;
  first-tick inheritance; cross-drill recency ordering; owner-scoped "Log again"
  (no IDOR); mixed-discipline session (auto-belay → campus) override;
  header consistency across run.html + manual_exercises.html
- **copy** — outcome labels, Focus/Thoughts placeholders
- **schema** — `GetLatestClimbingTickInRun` ordering/filesort; `RPE int`
  convention; owner-scoping on `GetClimbingTick`
- **simplify** — run_ticks.html JS is already large; watch for bloat
- **scribe** — update readme/DESIGN if the tick surface changes notably

## Resolved decisions

1. **Inheritance scope** — per-**run** (not per-drill); constants summary stays
   visible; one-tap override. Inherits only tick-level fields
   (Kind/Setting/Subtype/RopeStyle/Grade); Board/Venue from their own sources.
2. **Send → style** — always `redpoint` ("sent, not a flash"); refine for rest.
3. **RPE** — add `RPE int` (0-sentinel) now, no input UI.
4. **Repeats** — one row per burn (per-attempt reflection) + "Log again"
   re-seeds the single form server-side. `Attempts` always 1; totals by row
   count.
5. **Grade input** — horizontal chip strip (auto-scroll selected chip; rebuild
   via `populateGradeSelect` on kind change; clear on invalid).
6. **Rainbow/Traverse** — options *in the grade picker* (`Grade` string); when
   selected, **outcome row hidden** — just focus + thought + log.
7. **Focus + Thoughts visible by default** — collapse the *constants* instead.
8. **New + edit forms share the outcome row** (consistency rule).
9. **Polish** — live session header (static, inside fragment), scroll-via-IIFE
   confirm, preserve-open-details, empty state, touch targets; dark-mode bug fix.

## Phasing

Ship as separate PRs to keep review/rollback clean:

- **PR1 — dark-mode tick colours** (§5). Isolated CSS bug fix; ships first.
- **PR2 — additive, low-risk:** `RPE int` migration (§3a) · Rainbow/Traverse
  grade options (§5b within §4) · Focus textarea (§1). No logic risk.
- **PR3 — the logging-flow rewrite:** outcome row + handler inference on both
  forms (§2) · per-run inheritance query (§3) · "Log again" (§3b) · grade strip,
  session header, scroll-confirm, preserve-details, empty state, touch targets
  (§4). Depends on PR2. Consider splitting the handler/inference + query (qa /
  schema) from the pure-template grade-strip/header work (pixel).

PR1 and PR2 can land in parallel; PR3 depends on PR2.

All settled; nothing blocks implementation.
