# Passion Codebase — Technical Audit Report

*Multi-expert adversarial audit | Go + Chi + GORM/SQLite + HTMX | Generated 2026-06-30*

---

## 1. Executive Summary

Passion is a **fundamentally sound** server-rendered Go application. The architecture is coherent (Chi router, GORM models, HTMX partials), owner-scoping is applied consistently on data reads, auth uses a sensible HttpOnly + SameSite=Lax + Origin-checked cookie scheme, and the design system is mostly token-driven. The audit produced **no confirmed remote-code-execution, SQL-injection, or cross-tenant data-disclosure vulnerabilities.** Most "critical/high" headline findings were downgraded under adversarial verification because the app's real threat model is a small self-hosted personal/few-user climbing tracker, not a high-value SaaS.

That said, several **cross-cutting root causes** recur across the findings. Fixing the root cause resolves many leaf findings at once:

1. **Method-agnostic routing is the single largest structural risk.** Routes are registered with bare `pr.HandleFunc` covering *all* HTTP methods, so the only thing stopping a state-changing `GET` is whether each individual handler author remembered an `if r.Method != POST` guard. Several handlers forgot. This one decision is the root of the calendar-event CSRF finding, the `{action}`-dispatch finding, the `training-log/new` GET-mutation finding, and is the structural enabler behind the CSRF-middleware findings. **Multiple security and backend lenses converged here** — that convergence raises confidence that this is the highest-leverage fix in the report.

2. **Non-atomic multi-write sequences.** Mutations that touch several tables (run completion, template deletion, venue deletion, session start, cascade deletes) are mostly *not* wrapped in transactions, and several discard `.Error` entirely. **Backend, database, and correctness lenses all independently flagged this**, again raising confidence. Individually most are low-probability on single-writer SQLite, but as a class they are a consistency-hygiene debt.

3. **SQLite is opened with raw GORM defaults** — no `busy_timeout`, no WAL, unbounded connection pool. This is the latent multiplier behind every "what if a write fails mid-sequence" finding. Cheap to fix, and it de-risks the entire transaction class above.

4. **Cascade-delete coverage is incomplete and inconsistent.** Three different delete paths (`handleRunDelete`, `deleteTemplate`, `DeleteDraftRun`) cascade *different* subsets of the run-linked tables. The canonical, complete cascade already exists in `db.DeleteDraftRun`; the user-facing paths don't call it.

5. **Hardcoded hex colors bypass the design token system.** A handful of templates inline light-only hex values with no dark-mode counterpart, directly violating `DESIGN.md`'s "never hardcode hex" rule. Purely cosmetic, dark-mode-only, but a consistent house-rule violation.

The one finding that is **unconditionally worth treating as P0** is the default JWT secret hard-fail gap (see §2.1) and the week-override correctness bug (§2.4), which is a genuine "feature silently doesn't work" defect.

A note on confidence: findings below are marked **CONFIRMED** (verifier reproduced the defect) or **PLAUSIBLE** (logic is sound but the verifier could not fully confirm impact — re-check before investing). Severities shown are the verifier's *adjusted* severity, not the original reviewer's.

---

## 2. Findings by Area

### 2.1 Authentication & Secrets

#### Default JWT secret does not hard-fail startup — **CRITICAL (CONFIRMED)**
`cmd/passion/main.go:109`
The app only `slog.Warn`s when `Auth.JWTSecret == "change-me-in-production"`; `config.Validate()` (`config/config.go:90`) rejects only an *empty* secret. A deploy that copy-pastes `passion.example.yaml` runs on a publicly-known HS256 key, letting an attacker forge `authClaims{UserID: <victim>}` for any account. The token is verified with `[]byte(s.jwtSecret)` (`auth.go:226,254`).
- **Mitigating context:** `passion.example.yaml` also ships `DevAuthBypass: true`, which short-circuits verification (`auth.go:186`), so the forge-a-token exploit requires bypass *off* + secret left at default. Still: the code invites the misconfiguration.
- **Fix:** In `config.Validate()` (covers both file and env paths), `os.Exit(1)` when `JWTSecret == "change-me-in-production"` or is shorter than ~32 bytes, unless `DevAuthBypass` is explicitly on. Generate a random secret for local dev rather than shipping a constant. **Effort: S.**

#### Stateless 30-day JWT, no revocation — **LOW (CONFIRMED)**
`auth.go:135`
720-hour TTL (`passion.prod.yaml`) with sliding renewal (`auth.go:207`), no token-version/`credentials_updated_at` claim. Logout only clears the cookie client-side (`auth.go:135`); a captured token stays valid up to 30 days. Well-mitigated for browser flows (HttpOnly + Secure + SameSite=Lax + HS256 pinned), so theft requires a non-web path. **Fix:** add a per-user token version checked on every request, bumped on logout-all/password change; shorten the access TTL. **Effort: M.**

#### No login rate limiting / lockout — **MEDIUM (CONFIRMED)**
`auth.go:50`
`handleLogin` does unbounded email + bcrypt checks with no per-IP/per-account throttle, lockout, or backoff (`auth.go:38-66`). bcrypt cost 10 caps online guessing and the generic error blocks enumeration, but sustained credential-stuffing is unthrottled. **Fix:** per-IP and per-account rate limiting with exponential backoff/temporary lockout; log bursts. **Effort: M.**

### 2.2 CSRF & Method-Agnostic Routing *(root-cause cluster — 4 reviewers converged)*

This is the report's strongest signal: backend and security lenses independently arrived at the same root cause. The findings are **deduped** here.

#### Mutating handlers reachable via GET — **HIGH (CONFIRMED)**
`http/server/calendar_events.go:152` (+ `:44`, `:99`)
`handleCalendarEventCreate/Update/Delete` never check `r.Method` and are registered with bare `pr.HandleFunc` (`core.go:80-82`), so they execute on *any* method including GET. `handleCalendarEventDelete` doesn't even call `ParseForm` — a GET `/calendar-events/{id}/delete` soft-deletes the event. The same class appears at `training_log_manual.go:15` (`handleTrainingLogNew` creates a ScheduledSession + draft run on a plain GET).
- **Verifier's correction to the PoC:** the "zero-click `<img src>` CSRF" framing is **wrong** — SameSite=Lax does *not* attach the cookie to a cross-site subresource GET. A working attack needs a top-level navigation (a click), so it's noisy/visible, the delete is a recoverable GORM soft-delete, and create/update need valid query params. Real but narrower than billed.
- **Fix:** register all routes with explicit methods (`pr.HandleFunc("POST /calendar-events/{id}/delete", ...)` — the pattern already used at `core.go:124,130`) so a missing guard fails closed at the router. Make `/training-log/new` create the draft on POST only. **Effort: M.**

#### CSRF middleware allows missing-Origin requests; GET fully exempt — **LOW–MEDIUM (CONFIRMED)**
`auth.go:162`
`csrfMiddleware` exempts all GET/HEAD/OPTIONS (`:157-160`) and passes any unsafe request with an absent `Origin` (`:163-166`). There is no synchronizer/double-submit token. **Verifier's key correction:** the empty-Origin POST branch is effectively *dead code* from a CSRF standpoint — browsers attach `Origin` to all cross-site POSTs, and Lax cookies aren't sent on cross-site fetch/XHR/form-POST; the branch only serves legitimate non-browser same-site clients (which lack a session cookie anyway). So the genuinely exploitable surface is *only* the GET-mutation handlers above. The two findings (severity reviewer "medium", security reviewer "low") describe the same gap — **deduped to: the real fix is "never let GET mutate state."** A defense-in-depth CSRF token + fail-closed-on-missing-Origin is worthwhile hardening but not load-bearing. **Effort: M (token) / S (fail-closed).**

#### `{action}`-dispatch handlers duplicate method policy per-case — **LOW (PLAUSIBLE)**
`scheduled_sessions.go:24`
`handleScheduledSessionsByID` switches on an `action` URL param; `start`/`move` guard POST but `preview` (read-only) doesn't — which the verifier notes makes the *cited line a counterexample*, not a defect. The real point is architectural: method policy lives in hand-written switches, so a forgotten guard becomes a silent GET mutation (exactly the calendar bug). **Fix:** move method policy to chi method-aware registration; split `{action}` mega-handlers into one handler per concrete route. Resolves this *and* the calendar finding at the source. **Effort: M.**

#### Open redirect via unvalidated Referer — **LOW (PLAUSIBLE)**
`calendar_events.go:92` (+ `:145`, `:165`)
Post-action redirect goes to the raw `Referer` with no local-path check, unlike `handleRunDelete` (`run_actions.go:93`) which enforces `HasPrefix(returnTo, "/")`. **Verifier:** not realistically exploitable — Referer is browser-set, a cross-site POST is killed by the Origin check first, and a self-curl redirects only the caller. Consistency nit. **Fix:** validate the target like `handleRunDelete` does. **Effort: S.**

### 2.3 Transactions, Cascades & Data Integrity *(root-cause cluster — backend + database + correctness converged)*

#### `deleteTemplate` orphans SessionRun rows — **HIGH (CONFIRMED)**
`templates.go:354`
Hard-deletes `ScheduledSession` rows without touching the `SessionRun` rows that reference them via `ScheduledSessionID` (`not null`, no cascade — `models.go:259`). **Verifier narrows the blast radius:** history is *not* erased (SessionRuns persist) and the training-log list degrades gracefully via a zero-value map lookup (`training_log.go:114`); only the `/runs/{id}` detail/replay page 404s (`renderRun` → `GetScheduledSessionWithTemplate` → `ErrNotFound`). So: graceful degradation + one broken page, not "erased history." Still a real orphaning bug. **Fix:** wrap `deleteTemplate` in a transaction that also deletes/soft-deletes the dependent SessionRuns and their children — or switch to soft-delete throughout. **Effort: M.**

#### `handleRunDelete` cascade is incomplete *and* unchecked — **LOW (CONFIRMED orphaning / PLAUSIBLE error-handling)**
`run_actions.go:83-91`
Two findings dedupe here. (a) The cascade deletes only `RunExerciseChoice` + `RunExerciseCompletion`, leaving `ClimbingTick`, `SessionJournal`, `ManualExerciseSetLog`, and `ClimbingExerciseMeta` (all carry `run_id`) orphaned forever. (b) All four deletes ignore `.Error` and unconditionally set `HX-Redirect` + 200. The complete, transactional cascade **already exists** in `db.DeleteDraftRun` (`queries.go:563`) — this path just doesn't call it. **Verifier:** no FK enforcement means no integrity violation today and reads are owner+run-scoped, so impact is storage growth + a permanently-unreusable journal row (global unique on `run_id`, see §2.5). **Fix:** route `handleRunDelete` through the same transactional cascade as `DeleteDraftRun`; check each `.Error`; emit `HX-Redirect` only after commit. **Effort: M.** Resolves both findings.

#### Run/session multi-write ops not transactional — **MEDIUM (CONFIRMED)**
`runs.go:363`
`completeRunExercise` does set-log upserts → `Create(comp)` → re-query → `Save(&run)` as independent writes; `startTrialRun` (`templates.go:458`) and `handleStartOpenSession` (`open_session.go:82`) create ScheduledSession + SessionRun as two separate `Create`s. Only 4 `.Begin()` across ~41 `Create` calls. A mid-sequence failure leaves a completion recorded but run un-flagged, or a dangling ScheduledSession. **Verifier:** states are recoverable/cosmetic on a low-write single-user app. **Fix:** wrap each logically-atomic operation in `store.DB.Transaction(...)`, passing `tx` to the `db.*` helpers. **Effort: M.**

#### `MaterialiseTemplateExercises` discards completion-copy error — **LOW (CONFIRMED)**
`db/queries.go:922`
`tx.Create(&newComp)` drops its `.Error`; the transaction proceeds to set `exercises_materialised = true` (`:927`), permanently marking the run materialised with missing completions. **Verifier:** the "duplicate" failure mode is impossible (no unique index on those columns), the data is a copy of just-read valid rows, and the caller discards the function's error anyway (`_ = db.Materialise...`) — so realistic trigger is near-nil. One-line robustness fix. **Fix:** `if err := tx.Create(&newComp).Error; err != nil { return err }`. **Effort: S.**

#### `DeleteClimbingVenue`/`DeleteClimbingBoard` non-transactional + over-clear — **LOW (PLAUSIBLE)**
`db/queries.go:498` (+ `:520`)
Two-statement null-then-delete with no transaction. Genuinely valid sub-issue: `DeleteClimbingVenue`'s `Updates` at `:501` also nulls `board_id`, clearing a board reference it shouldn't. **Fix:** wrap in a transaction; null only `venue_id` in the venue path. **Effort: S.**

### 2.4 Correctness

#### Week overrides never applied at runtime — **HIGH (CONFIRMED)** ⚠️ genuine feature defect
`runs.go:621`
`buildRunSteps` builds its override map from `CycleExerciseOverride` (cycle-level only — verified at `runs.go:639-650`) and never loads `CycleExerciseWeekOverride`. The documented resolution order "**week override → cycle override → template default**" never executes. The cycle-detail UI (`training_cycle_overrides.go:73`) reads/displays week overrides, but that data **never feeds the run player**. A user who sets "Week 3: Deadhang 4×15s" always gets the cycle/template default. No test covers override resolution. The verifier searched for any week-number-from-date materialization elsewhere and **found none** — the feature is genuinely unwired.
- **Mitigating context:** only active when the opt-in `VariesByWeek` toggle is set, so blast radius is power users.
- **Fix:** in `buildRunSteps`, after cycle overrides, load `ListCycleExerciseWeekOverrides`, compute the week number of `ss.ScheduledDate` relative to cycle `StartDate`, and apply week-specific values when present. **Add an integration test.** **Effort: M.**

#### Double-submit creates duplicate RUNNING SessionRuns — **MEDIUM (CONFIRMED)**
`scheduled_sessions.go:53`
The `start` action `Create`s a SessionRun with no pre-check for an existing run on the same `scheduled_session_id`; no DB unique constraint (`models.go:255-276`). A double-click/retry yields two RUNNING runs with completion state split across two IDs. **Verifier corrects the scope:** the open-session and trial paths create a *fresh* ScheduledSession per request (1:1 by design), so only the cited `/scheduled-sessions/{id}/start` path is affected. No corruption — duplicates are visible and one-click-removable via the discard affordance. **This is the same root cause as the UX "duplicates pile up" finding (§2.6) — deduped.** **Fix:** before creating, query for an existing run on that `scheduled_session_id`; if RUNNING, redirect to it; if COMPLETED, confirm. Prefer an application-level check over a `(owner_id, scheduled_session_id)` unique index (legitimate re-runs are a plausible future need). **Effort: M.**

#### Cross-scale grade ranking — V16 loses to French 8a — **MEDIUM (CONFIRMED)**
`db/queries.go:428`
`buildGradeRanks` assigns ordinals by list position *within each scale*, then merges all four scales into one `map[string]int`. Namespaces collide: V16 = rank 17, French 8a = rank 18, so `GetClimbingSessionHeader` (`:466`, picks max rank) reports 8a as harder than V16. **Verifier:** only misfires when a single run mixes boulder + route systems at the specific upper-grade overlap; single-discipline sessions (the common path) are fine; "hardest grade across incommensurable disciplines" is arguably ill-defined; and the lexicographic-comparison half targets dead `MinGrade`/`MaxGrade` fields. **Fix:** key ranks by `(system, grade)`; only compare within a system, or report hardest-per-system. Add a V16-vs-8a test. **Effort: M.**

### 2.5 Cross-Tenant Squatting (DoS)

#### SessionJournal global-unique `RunID` + unverified run ownership — **MEDIUM (CONFIRMED)**
`training_log.go:744`
`handleTrainingLogForRun` and `handleRunStop` (`run_actions.go:45`) create `SessionJournal{OwnerID: caller, RunID: &runID}` with `runID` from the URL and **no check the run belongs to the caller**. `SessionJournal.RunID` is `uniqueIndex` on `RunID` alone, not composite (`models.go:354`). An attacker can pre-create a journal pointing at a victim's run IDs, so the victim's own stop/journal flow hits the unique constraint and errors — a cross-tenant DoS. **Verifier:** DoS only (no disclosure, no cross-tenant write, no escalation — the planted row is attacker-owned); requires an authenticated account + run-ID enumeration; recoverable by deleting planted rows. **Fix:** verify run ownership before creating a journal in both handlers (and across `/training-log/draft/{runID}/*`); change the unique index to composite `(owner_id, run_id)`. **Effort: M.**

#### `loadOpenSteps` lacks owner_id scope — **LOW (CONFIRMED)**
`runs.go:543`
Filters exercises by `session_run_id` only; the companion `ListExercisesForRun` (`queries.go:625`) includes `owner_id`. All three callers verify run ownership first, so no current IDOR — defense-in-depth only. **Fix:** add `AND owner_id = ?` or call `db.ListExercisesForRun`. **Effort: S.**

### 2.6 UX

| Finding | Sev | Location | Fix |
|---|---|---|---|
| Start button shown on Done sessions; no resume/dup guard | MEDIUM (CONFIRMED) | `scheduled_sessions.go:53`, `dashboard.html:149` | Hide/disable Start when `.Done`; resume existing RUNNING run. *Same root cause as §2.4 double-submit.* |
| Guided run dead-ends on "Workout complete" — never offers journal | MEDIUM (CONFIRMED) | `runs.go:407` | On final completion redirect to `/runs/{id}/summary` (like `handleRunStop`) and ensure an empty journal row exists, so RPE/sleep/energy is captured in-flow. |
| State-changing run actions give no feedback (`hx-swap="none"` + HX-Redirect only) | MEDIUM (CONFIRMED) | `run.html:190` | Add a global `htmx:responseError`/`htmx:sendError` toast handler + `hx-indicator`; treat "already completed" double-tap as success. |
| Trial/template runs' back arrow drops user into the template **editor** | LOW (CONFIRMED) | `run.html:433` | Route workout-launched runs' back arrow to `/dashboard`; reserve "Back to template" for explicit template-preview via a dedicated flag, not overloaded `IsTrial`. |
| "Working" ascent result is invisible after saving (no badge) | LOW (CONFIRMED) | `climbing_ticks.go:15` | Add a `working` case to `tickStyleDisplay` (or collapse working/attempt); keep form options and display switch enumerating the same set. *Note: tick still gets the `tick-card--working` amber left-border, so not fully invisible.* |
| New-user onboarding is a near-empty dashboard | LOW (PLAUSIBLE) | `auth.go:116` | Add a first-run empty-state welcome card explaining template→cycle→run (the "Start open session" / library CTAs already exist). |
| Per-set actuals lossy: reps stored as integer average, only max weight | LOW (PLAUSIBLE) | `runs.go:342` | Per-set logs *are* persisted; surface them in the summary or label the aggregate as an average and avoid integer truncation. (Finding's cited `run_summary.html:57-58` actually shows *planned* sets — re-check.) |

### 2.7 Design System (hardcoded hex — root-cause cluster)

`DESIGN.md` explicitly says "never hardcode hex values," yet several templates inline light-only colors with no dark-mode token. All cosmetic, dark-mode-only, no functional impact.

| Finding | Sev | Location | Fix |
|---|---|---|---|
| Conflict-warning banner: 6 hardcoded hex, no dark path | MEDIUM (CONFIRMED) | `dashboard.html:77-85`, `new_cycle.html:12-14` | Add `--warning-bg/-fg/-border` tokens; or reuse existing `--tick-amber-*`. *Verifier: text always sits on its own light bg, so "illegible" claim is false — it's a bright island, not unreadable.* |
| "Done" badge + JS-emitted Done label hardcode green | MEDIUM (CONFIRMED) | `dashboard.html:130,292`; `history.html:161,164` | Use existing `--tick-green-bg/-fg` (already have dark variants); add a CSS class for the JS string. |
| Activity-heatmap legend swatches hardcode indigo | LOW (CONFIRMED) | `history.html:47-49` | Define `--heatmap-tier-*` tokens used by both legend HTML and JS, or have the heatmap JS also paint the legend. *Verifier: swatches sit on `--panel` not `--bg`; visible but low-contrast.* |
| Error banners hardcode `#dc2626` instead of `var(--destructive)` | LOW (CONFIRMED) | `profile.html:8`; `training_log_new.html:25` | Use `var(--destructive)`. *Verifier: the genuine dark-mode contrast issue is only in `training_log_new.html` (text color); `profile.html` is a decorative left-border only.* |
| Icon-only buttons have `title` but no `aria-label` | MEDIUM (CONFIRMED) | `training_log.html:48,56,63,68`; `open_exercise_panel.html:5` | Add `aria-label` to all icon-only buttons. **This is systemic** (`calendar.html`, `template_edit.html`, `history.html:181,190`, etc.) — the finding's cited "reference" (`history.html`) is *itself* non-compliant. Treat as one codebase-wide a11y cleanup ticket. |
| HTMX tick-list `<section>` lacks `hx-target="this"` inside a form | LOW (PLAUSIBLE) | `run.html:507` | Add `hx-target="this"` per CLAUDE.md rule. *Verifier: works correctly today (no ancestor `hx-target`); the finding's mechanism explanation is wrong. Future-proofing only.* |

### 2.8 Concurrency & Robustness

- **SQLite opened with GORM defaults — MEDIUM (CONFIRMED)** `db/store.go:13`. No `busy_timeout`, no WAL, unbounded pool. A concurrent write returns `SQLITE_BUSY` → 500. Low probability for the likely single-user deployment, but the **cheapest high-leverage fix in the report** — it de-risks every transaction finding in §2.3. **Fix:** `sqlite.Open("file:"+path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")` + `SetMaxOpenConns(1)`. **Effort: S.** *(Note: enabling `_foreign_keys=on` also activates the existing — currently cosmetic — `OnDelete:CASCADE` tags, which interacts with §2.3 cascades; test together.)*
- **DB queries never receive `r.Context()` — LOW (PLAUSIBLE)** `core.go:40`. No `.WithContext`. Verifier: immaterial on sub-ms local SQLite; `Shutdown` drains rather than cancels; doesn't meaningfully worsen locking. Adopt as Go best practice, low priority. **Fix:** `gdb := s.store.DB.WithContext(r.Context())` per request. **Effort: M.**

### 2.9 Code Quality (all LOW, all CONFIRMED unless noted)

Mostly duplication with no behavioral impact. Group into one or two cleanup tickets.

- **Dead code:** `GetClimbingTickSummaryForRun` (`queries.go:384`) has zero callers — delete it + its struct. *(Verifier: the "gives wrong results" claim is a red herring; it's inert.)* **S.**
- **Inert schema:** `ClimbingTick.RPE` (`models.go:417`) has no write path. *Verifier: do NOT drop it — GORM AutoMigrate never drops columns, so removal orphans the column forever; it's a documented forward-reservation in an actively-iterated area.* Leave as-is or document. **S.**
- **`ExercisePlannedSet` tag puts `not null` inside the `index:` tag (CONFIRMED)** `models.go:318` — silently dropped, so no NOT NULL DDL. Verifier confirms the ORM path can't produce NULL anyway (writes go through GORM, zero-valued uint → `0` not NULL). Defense-in-depth. **Fix:** `gorm:"not null;index:idx_exercise_planned_set,unique"`. **S.**
- **Duplication (extract a shared helper each):** tick form-parsing in `createExerciseTick`/`handleExerciseTickUpdate` (`climbing_ticks.go:271`); `ManualExerciseView` loop in `renderManualExercises`/`handleTrainingLogEdit` (`training_log_manual.go:308`); venue find-or-create in `finaliseManualEntry`/`updateJournal` (`training_log_manual.go:119`); `RunSummaryExercise` branches in `handleRunSummary` (`run_actions.go:147`). **Caution flags from the verifier:** the "missing closing brace" and "nil-clear bug" sub-claims are **fabricated/illusory** — don't chase them; `parseInt`'s default param exists deliberately (attempts defaults to 1, unlike `formInt`). **M total.**
- **Two `relativeDate` helpers** (`history.go:438` vs `time_helpers.go:40`) differ by one comma. Standardize to one. **S.**
- **Delegation wrappers** `loadTemplateWithGraph`/`listLibraryExercises`/`listTemplates` (`templates.go:384`) — PLAUSIBLE. *Verifier: do NOT delete — they normalize a swapped arg order and default filters; if anything, route direct callers through them.* **S.**

### 2.10 Documentation (all LOW after adjustment, all CONFIRMED)

- **`make reseed` silently broken — MEDIUM** `Makefile:22`: missing `PASSION_SEED=1`, so the seed block (gated at `main.go:59`) never runs and the server boots on an empty DB instead of exiting. **Fix:** prefix `PASSION_SEED=1`. **S.** *(Most user-impacting doc item — it actively misleads the documented dev workflow.)*
- `DEVELOPMENT.md:35` links to nonexistent `http/server/middleware.go` → should be `auth.go`. **S.**
- `readme.md:107` documents `PASSION_YAML_IMPORT_OWNER_ID`, but `main.go` ignores it (imports for every user). **S.**
- `readme.md:99` says JWT TTL default 168h; code default is 720h (`config.go:68`). Align docs and code. **S.**
- `readme.md:93` omits the `PASSION_CONFIG` env var. **S.**
- `DEVELOPMENT.md:31` cites `pages.MyFragment` (placeholder; real API is `RenderFragment`/named methods). **S.**
- `DESIGN.md:121` icons table has a phantom third column. **S.**

---

## 3. Prioritized Remediation Plan

### P0 — Must fix (security / correctness / silent feature failure)
| # | Item | Resolves | Effort |
|---|---|---|---|
| 1 | Hard-fail startup on default/short JWT secret (in `config.Validate`) | §2.1 critical | S |
| 2 | Apply week overrides in `buildRunSteps` + integration test | §2.4 week-override (feature broken) | M |
| 3 | Register all routes with explicit HTTP methods (router-level method policy); make all mutating handlers POST-only | §2.2 calendar GET-mutation, `{action}`-dispatch, `training-log/new` GET | M |
| 4 | Fix `make reseed` (`PASSION_SEED=1`) | §2.10 reseed | S |

### P1 — High-impact behavior / UX / data hygiene
| # | Item | Resolves | Effort |
|---|---|---|---|
| 5 | SQLite pragmas + `SetMaxOpenConns(1)` (WAL, busy_timeout, foreign_keys) | §2.8 locking; de-risks all of §2.3 | S |
| 6 | Idempotent session start (resume existing RUNNING run) + hide Start on Done | §2.4 double-submit, §2.6 duplicates | M |
| 7 | Route `handleRunDelete` + `deleteTemplate` through complete transactional cascade (align with `DeleteDraftRun`) | §2.3 incomplete/unchecked cascade, orphaned SessionRuns | M |
| 8 | Verify run ownership in journal-create handlers + composite `(owner_id, run_id)` unique index | §2.5 cross-tenant squatting | M |
| 9 | Guided-run completion → `/runs/{id}/summary` with journal in-flow | §2.6 dead-end journal | M |
| 10 | Wrap run/session multi-write ops in transactions | §2.3 non-atomic ops, venue delete | M |
| 11 | Login rate limiting / lockout | §2.1 brute-force | M |
| 12 | Fix cross-scale grade ranking (`(system, grade)` keys) + test | §2.4 grade ranking | M |

### P2 — Quality / consistency / hardening
| # | Item | Resolves | Effort |
|---|---|---|---|
| 13 | Introduce `--warning-*` tokens; replace all hardcoded hex in templates with tokens | §2.7 all 4 hex findings | M |
| 14 | Codebase-wide `aria-label` pass on icon-only buttons | §2.7 a11y | M |
| 15 | JWT revocation (token version) + shorter TTL | §2.1 stateless token | M |
| 16 | HTMX global error toast + `hx-indicator` on run actions | §2.6 silent feedback | M |
| 17 | One-line robustness fixes: `MaterialiseTemplateExercises` error check, `loadOpenSteps` owner scope, `ExercisePlannedSet` tag, Referer validation, `hx-target="this"` | §2.3/§2.5/§2.7/§2.9 | S |
| 18 | Add `working` tick display case; fix back-arrow routing for trial runs | §2.6 working/back-arrow | S |

### P3 — Polish / docs / dead code
| # | Item | Resolves | Effort |
|---|---|---|---|
| 19 | Doc fixes: `middleware.go` link, `PASSION_YAML_IMPORT_OWNER_ID`, JWT TTL default, `PASSION_CONFIG`, `pages.MyFragment`, DESIGN.md table | §2.10 | S |
| 20 | Delete dead `GetClimbingTickSummaryForRun`; consolidate `relativeDate` | §2.9 | S |
| 21 | Extract shared helpers (tick-form, ManualExerciseView, venue, RunSummary) — heed the verifier's caution flags | §2.9 duplication | M |
| 22 | First-run onboarding card; thread `r.Context()` into queries | §2.6 / §2.8 | M |

---

## 4. What's Done Well

An honest balance — the audit surfaced real debt, but the foundation is solid:

- **Consistent owner-scoping on reads.** Every verifier that probed for cross-tenant *data disclosure* came up empty. Query helpers (`ListExercisesForRun`, `GetSessionJournalByRunID`) thread `owner_id` through, and handlers verify run ownership before acting. The squatting finding (§2.5) is a *write-collision DoS*, not a read leak — the boundary holds.
- **Sensible auth primitives.** HttpOnly + Secure + SameSite=Lax cookie, HS256 pinned (no alg-confusion), Origin-checked CSRF middleware, bcrypt at cost 10, generic login errors that block username enumeration. The verifiers repeatedly *downgraded* security findings because these mitigations were genuinely effective.
- **Mostly token-driven design system** with documented dark-mode variants (`--tick-*`, `--destructive`) and a `DESIGN.md` that the hardcoded-hex findings are measured against — i.e., the rule exists and is mostly followed; the violations are the exception.
- **Graceful read-side degradation.** When `deleteTemplate` orphans a run, the training log renders blanks rather than crashing (zero-value map lookups, `if ssErr == nil` guards) — defensive read code limited the blast radius of a write-side bug.
- **The correct patterns already exist in-tree.** `db.DeleteDraftRun` is a complete transactional cascade; `core.go:124,130` shows method-prefixed routing; `handleRunDelete` validates redirect targets; `exercise_library.html` has correct `aria-label`s. Most fixes are about *propagating an existing good pattern*, not inventing one — which makes the remediation low-risk.
- **Adversarial verification meaningfully sharpened the findings.** Several headline PoCs were factually wrong (zero-click image CSRF, "illegible" banner text, fabricated missing braces, a misdiagnosed HTMX mechanism). The surviving, severity-adjusted set is a trustworthy fix list — but the team should still **re-confirm the four PLAUSIBLE items** (`handleRunDelete` orphaning impact, context-threading, `{action}`-dispatch, lossy per-set actuals) before investing effort, since the verifier could not fully close them.

**Bottom line:** No emergency. Land P0 #1 and #2 promptly (a forgeable-token misconfig and a silently-dead feature), batch P1 around the SQLite-pragma + transaction work since they reinforce each other, and treat the method-agnostic routing refactor (P0 #3) as the structural investment that retires an entire class of future bugs.