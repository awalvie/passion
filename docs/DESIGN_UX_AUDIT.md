# Passion — Design / UI / UX Deep-Dive Audit

*Two-expert review — `pixel` (design-system consistency + accessibility) and `scout` (user journeys + UX quality, benchmarked against real climbing/training apps). Headline new findings independently verified against the code. Generated 2026-06-30.*

---

## 1. Executive summary

The design system and the UX foundation are both **stronger than the gaps suggest** — the problems cluster, and most are cheap to fix because the *correct pattern almost always already exists in the tree*. Two themes dominate:

1. **Passion has built more good UX than it exposes.** The journal/reflection page, the previous-run comparison, tick inheritance, and conflict-detection are all well-made — they just aren't wired into the highest-frequency paths. The single highest-leverage fix in the whole app (route a finished guided run to the summary/journal page) is *two redirects to a page that already exists*.

2. **A small set of token/markup slips produce visible breakage.** Undefined CSS variables (`--danger`, `--card`, `ring-ring`) render UI unstyled or transparent, hardcoded hex breaks dark mode, and developer copy ("MVP next:") is shipping to users. These are mechanical, low-risk fixes.

Underneath, two **systemic** issues cut across many findings: (a) **accessibility is under-served** — no `h1` on half the pages, icon-only controls without accessible names, hover-only actions invisible to touch/keyboard; and (b) **the app's signature data (climbing ticks) is collected but never reviewed** — History has zero grade/send analytics, orphaning the data that makes Passion a *climbing* app rather than a generic logger.

**Verdict:** no architectural rework needed. A short P0 of "make broken UI render," then a P1 that wires existing infrastructure into the daily flows (finish→journal, climbing analytics, state-aware Start), then a systematic consistency + a11y sweep.

> **Confidence note:** the four headline *new* design bugs (§3.A) were verified by grep against the source. Two pixel overstatements were corrected: the `--danger` banner is **unstyled, not invisible** (text falls back to body color), and the `run.html` tick `<section>` **works today** (HTMX defaults the target to the element itself) — it is a compliance/future-proofing fix, not a live page-break.

---

## 2. Cross-cutting themes

| Theme | Where it shows up | Fix shape |
|---|---|---|
| **Built-but-unexposed UX** | Guided run never reaches the journal/summary (`run_actions.go:53`); History ignores all `ClimbingTick` data | Wire existing pages/queries into the flow |
| **Undefined tokens → broken render** | `--danger` (login/signup), `--card` (calendar, base), `ring-ring` (calendar select) | Swap to the real token (S each) |
| **Hardcoded hex breaks dark mode** | dashboard, history, new_cycle, profile, training_log_new, passion.css | Use `--tick-*` / `--destructive` (M) |
| **Accessibility under-served** | No `h1` ×12 pages; icon-only buttons; hover-only actions; non-button controls | Mechanical sweeps (M) |
| **State-unaware controls** | Start shown on Done sessions; duplicate runs; no run-action feedback | State-aware buttons + reuse `#run-feedback` (M) |
| **Climbing data orphaned at review** | History has no grade/send/pyramid view; no PR markers | New aggregate query + section (M) |

---

## 3. Findings by area

Severity scale: **HIGH** = blocks/badly degrades a core flow or breaks visible UI · **MEDIUM** = notable friction or consistency/a11y gap · **LOW** = polish. Effort: **S/M/L**.

### A. Broken / invisible UI — undefined tokens & leaked copy *(NEW, verified)*

These render incorrectly **right now**. All are one-to-few-line fixes.

| # | Sev | Location | Problem | Fix |
|---|---|---|---|---|
| A1 | HIGH | [login.html:7](../templates/login.html#L7), [signup.html:7](../templates/signup.html#L7) | `var(--danger)` is undefined (only `--destructive` exists). Auth error banners lose their red styling — border/text fall back to normal body color, so **an error doesn't read as an error**. | `var(--danger)` → `var(--destructive)` (both props, both files). **S** |
| A2 | MEDIUM | [calendar.html:103](../templates/calendar.html#L103), base.html:31-32 | `var(--card)` is undefined (token is `--panel`). Event sidebar cards + the markdown-editor surfaces render with a **transparent background**. | `var(--card)` → `var(--panel)`. **S** |
| A3 | MEDIUM | [dashboard.html:252,262](../templates/dashboard.html#L252) | `ring-ring` is not a defined utility/token anywhere — the calendar **selected-day highlight** doesn't get the intended accent ring. | Replace with `outline: 2px solid var(--accent-border)` (inline in the JS) or a real `.calendar-cell-selected` class. **S** |
| A4 | MEDIUM | [training_cycles.html:58](../templates/training_cycles.html#L58) | Developer copy **"MVP next: create your first cycle…"** is shipping to end users. | Replace with real empty-state copy, e.g. "No cycles yet. Create your first cycle to generate scheduled sessions." **S** |

### B. User-journey & UX quality *(scout; prior-audit items marked ✔)*

#### B1 — Guided run dead-ends; the journal exists but is never shown — **HIGH ✔** · S
[run_actions.go:53](../http/server/run_actions.go#L53), [runs.go:408](../http/server/runs.go#L408), [run.html:21-28](../templates/run.html#L21)
`handleRunStop` already **creates an empty journal for every run** (`run_actions.go:45-51`), and [run_summary.html:71-90](../templates/run_summary.html#L71) already lazy-loads that journal form *plus* a previous-run comparison. But guided runs are redirected to `/dashboard`; only open/manual runs reach `/summary`. The climber finishes a session and is bounced away from the single most valuable habit a training app builds (RPE/sleep/feel/next-focus). **Fix:** route guided-run completion and `stop` to `/runs/{id}/summary` — the destination is already built and good. *Highest leverage, lowest cost in the app.*

#### B2 — History has zero climbing analytics — **HIGH** · M
[history.go:189-334](../http/server/history.go#L189), [history.html](../templates/history.html)
History computes streaks, a 12-week trend, a 365-day heatmap, and template breakdown — **all from `SessionRun` completions**. It never aggregates `ClimbingTick` (grade, style, send, attempts, stars). The climber who logged 10 boulders sees one heatmap dot and "1 session." They cannot answer *am I climbing harder? what's my send rate? what have I projected?* — climbing's equivalent of a strength app that never shows your 1RM. **Fix:** add a climbing-analytics section (grade pyramid attempted-vs-sent, hardest-sent-grade trend, send rate, indoor/outdoor & board/wall splits). The range filter and per-tick data already exist; this is one aggregate query + one rendered section, no schema change. *This is the feature that makes Passion a climbing app.*

#### B3 — "Working" ascent renders no badge — silent data — **HIGH ✔** · S
[climbing_ticks.go:15-33](../http/server/climbing_ticks.go#L15), [run_ticks.html:469](../templates/fragments/run_ticks.html#L469)
The form offers a `Working` result, but `tickStyleDisplay` has no `"working"` case (returns empty), and `styleImpliesSent("working")` is false. The most common boulder outcome — "worked it, didn't send" — stores fine but renders with **no chip and no badge** (only the faint amber tint), indistinguishable from a half-logged tick. **Fix:** add a `"working"` case to `tickStyleDisplay` (label + class + icon), or collapse Working onto Hangdog/Attempt. Every selectable result must produce a visible badge.

#### B4 — `Start` creates duplicate runs; shown on Done sessions; no resume — **HIGH ✔** · M
[scheduled_sessions.go:40-67](../http/server/scheduled_sessions.go#L40), [dashboard.html:149-158](../templates/dashboard.html#L149)
`start` creates a `SessionRun` unconditionally (no lookup for an existing run); the dashboard Start button has no `{{ if not .Done }}` guard and `hx-swap="none"` with no debounce, so a double-tap fires twice. **Fix:** if a RUNNING run exists for that scheduled session, redirect to it; make the card button state-aware Start→Resume→View. The data (`activeRuns`, `completedWeekSSIDs`) is already loaded at `dashboard.go:240-256`.

#### B5 — Trial/template-launched run's back arrow drops into the template **editor** — **HIGH ✔** · S
[run.html:433-434, 94-95](../templates/run.html#L433)
When `RunIsTrial`, the hero back-arrow points to `/templates/{id}/edit`. Mid-workout, "back" lands a chalky-handed user in a form editor for the template they're running. **Fix:** trial back-arrow → `/dashboard` (or `/templates`); keep an explicit "Edit template" link only if needed, never as the back gesture.

#### B6 — New user lands on a near-empty dashboard — **HIGH ✔** · S–M
[auth.go:116-122](../http/server/auth.go#L116), [dashboard.html:166](../templates/dashboard.html#L166)
Signup imports the exercise *library* but creates zero templates/sessions/cycles, so the first screen is an empty *planning* surface — not the "log what I'm about to do" the user came for, and the imported library is invisible. **Fix:** first-run empty state offering the two zero-setup actions that already exist ("Start an open session", "Log a past session") alongside "Create your first template"; optionally seed one example template.

#### B7 — State-changing run actions give no feedback — **MEDIUM ✔** · S
[run.html:638,646](../templates/run.html#L638), [dashboard.html:32-42](../templates/dashboard.html#L32)
Stop/delete/finish are `hx-swap="none"` + server `HX-Redirect`; the existing `#run-feedback` toast is wired only for complete/skip. On a slow link "Finish" looks dead → user re-taps → hits the "not in progress" 400. **Fix:** reuse `#run-feedback` ("Finishing…") on submit; add `hx-indicator` (see E4).

#### B8 — Climbing-log friction & tap count — **MEDIUM** · M
[run_ticks.html:465-470](../templates/fragments/run_ticks.html#L465)
The signature feature's bones are excellent (constant inheritance, windowed grade strip, live session header, "Log again", the novel per-climb **Focus** field). But "Result" is a 5-way radio (Onsight/Flash/Redpoint/Hangdog/Working) + a separate Attempts stepper — ~3 taps/climb minimum (board apps log a send in 1). **Fix:** an outcome quick-action row (`Flash · Send · +Attempt · Working`) that submits and infers `Style` server-side (the model already supports it), keeping the 5-way detail in the expanded edit view.

#### B9 — Manual-log draft created on page *view* leaks onto the dashboard — **MEDIUM** · M
[training_log_manual.go:46-60](../http/server/training_log_manual.go#L46)
`GET /training-log/new` immediately creates a draft `SessionRun` + trial `ScheduledSession` so HTMX routes are live. Abandon the page → a persistent "Unfinished log entry" nag accretes on the dashboard. The Continue/Discard recovery is a good safety net but shouldn't be *triggered by viewing the form*. **Fix:** lazily create the draft on first exercise/tick add, or sweep empty drafts on dashboard load.

#### B10 — No PR / personal-best markers — **MEDIUM** · M
Neither History nor the run summary flags a new hardest grade, first send of a grade, or hang-weight PR. Strong's star-on-PR is its most-loved feature. **Fix:** start with "new hardest grade sent" on the run summary + a star in History (compare against prior max).

*Scout also flagged structural smell **B11 (MEDIUM, S):** the climbing tick `<section>` is nested inside `<form id="complete-form">` ([run.html:499-516](../templates/run.html#L499)) — nested forms are invalid HTML and work only by parser luck. Move the tick section out to a sibling before it bites.*

### C. Design-system / token consistency *(pixel; prior-audit items ✔)*

#### C1 — Hardcoded hex breaks dark mode — **MEDIUM ✔** · M (batch)
Light-only values with no dark path, where a token already exists:
- Amber warning UI → use `--tick-amber-*`: [dashboard.html:77-85](../templates/dashboard.html#L77), [new_cycle.html:12,14](../templates/new_cycle.html#L12)
- Green "Done"/complete → use `--tick-sent-accent` / `--tick-green-*`: [dashboard.html:130,292](../templates/dashboard.html#L130), [history.html:161,164](../templates/history.html#L161), `passion.css:2783,2788,2832`
- Destructive → use `var(--destructive)`: [profile.html:8](../templates/profile.html#L8), [training_log_new.html:25](../templates/training_log_new.html#L25)
- Heatmap legend swatches hardcode indigo while JS cells switch to violet in dark mode → generate the legend from the same `tiers[]` array: [history.html:47-50](../templates/history.html#L47)
- CSS one-offs: `.catalog-dur-badge` (passion.css:2415,2423), `.catalog-dialog-hero/media` `#0f172a` (light-mode-wrong, passion.css:2451,2459)

#### C2 — Component pattern drift — **MEDIUM/LOW** · S–M
- Empty states diverge from the documented `card card-pad text-center py-12 + 30%-opacity icon` pattern: [training_cycles.html:57](../templates/training_cycles.html#L57), [dashboard.html:166](../templates/dashboard.html#L166), and `templates.html` (renders an empty `<table>` with no empty state at all).
- Event sidebar card hand-rolls its style instead of `class="card"` + left-border: [calendar.html:103](../templates/calendar.html#L103).
- Form label style: `new_cycle.html` uses `text-sm font-medium` vs the canonical `text-xs font-semibold muted` uppercase used everywhere else ([new_cycle.html:53,63,75](../templates/new_cycle.html#L53)).
- Selects missing `text-sm`: [exercise_library.html:21,25,32](../templates/exercise_library.html#L21).
- Detail-page title off-scale (`text-base` vs `text-xl`): [training_log_summary.html:12](../templates/training_log_summary.html#L12).

#### C3 — Icon-size drift off the documented scale — **LOW** · S
Ad-hoc `0.65rem` / `0.7rem` / `0.8rem` / `0.85rem` icons that should snap to `0.6rem` (badge) or `0.75rem` (action): training_log.html:118-130, history.html:182,191, run_summary.html:25,31, journal_form.html:13-37, training_cycle_detail.html, run_ticks.html. Also `14px` fixed-px icons (templates.html, exercise_library.html) → `0.875rem` to respect zoom.

### D. Accessibility *(pixel; systemic)*

| # | Sev | Location | Problem | Fix |
|---|---|---|---|---|
| D1 | MEDIUM | 12 pages (training_cycles, templates, exercise_library, profile, new_*, login, signup, activity_templates…) | No `h1` — top heading is `h2`. Fails WCAG 2.4.6; outline-navigation has no landing point. | Top heading → `h1 class="text-xl font-bold"` (matches training_log.html). **S–M** |
| D2 | MEDIUM | [training_log.html:48,56,63,68](../templates/training_log.html#L48), [open_exercise_panel.html:5](../templates/fragments/open_exercise_panel.html#L5), open_template_panel.html:5, dashboard.html:28 | Icon-only controls have `title` only (not announced on keyboard focus). | Add `aria-label`. **M (systemic)** |
| D3 | MEDIUM | [dashboard.html:43-45](../templates/dashboard.html#L43) | "Resume session" is a `<span>` — no focus, role, or keyboard path. | Make it an `<a href>` with `aria-label`. **S** |
| D4 | MEDIUM | [dashboard.html:191](../templates/dashboard.html#L191) | Calendar day cells are `<div onclick>` — `role`/`tabindex`/key handler absent; keyboard users can't pick a day. | `role="button" tabindex="0"` + Enter/Space handler, or `<button>`. **S–M** |
| D5 | MEDIUM | [training_log.html:45](../templates/training_log.html#L45), history.html:181-192 | Hover-reveal actions (`opacity-0 group-hover:opacity-100`) are invisible to **touch and keyboard**. training_log.html lacks even the `md:` prefix, so it hides on mobile too. | Add `focus-within:opacity-100` (global CSS rule possible) + `md:` prefix so mobile shows actions. **S** |
| D6 | LOW | [exercise_library.html:49-51,72-76](../templates/exercise_library.html#L49) | Select-all + row checkboxes have no accessible name. | `aria-label`. **S** |
| D7 | LOW | [run.html:379,407](../templates/run.html#L379) | Exercise thumbnails use `alt=""` though they carry meaning. | `alt="{{ .Name }}"`. **S** |
| D8 | LOW | [calendar.html:59](../templates/calendar.html#L59) | Event pills use `color:{{.Color}}` on an 8%-opacity tint of the same hue — user-picked mid-range hues can fail 4.5:1 contrast. | Prefer `--tick-*-fg` over `--tick-*-bg`, or enforce a min-contrast check. **M** |
| D9 | LOW | [dashboard.html:66](../templates/dashboard.html#L66), history.html:179 | Small primary buttons omit the `btn` base class → bypass the 2.75rem touch-target minimum. | Add `btn`. **S** |

### E. HTMX correctness *(pixel; prior-audit items ✔)*

- **E1 — LOW ✔ · S** [run.html:507-514](../templates/run.html#L507): tick `<section>` lazy-load inside a form lacks `hx-target="this"` (CLAUDE.md rule). **It works today** (HTMX defaults the target to the element itself; no ancestor overrides it) — this is compliance/future-proofing, *not* the "swaps into body" break pixel described.
- **E2 — MEDIUM · S** [training_cycle_detail.html:79,141](../templates/training_cycle_detail.html#L79): destructive "remove session from cycle" forms have no `hx-confirm`; hover-only + mobile mis-tap = silent irreversible removal. Add `hx-confirm`.
- **E3 — LOW · S** Inconsistent confirm mechanism: [exercise_library.html:130](../templates/exercise_library.html#L130) and [templates.html:64](../templates/templates.html#L64) use native `onsubmit="return confirm()"` while the app standard is `hx-confirm`. Standardize on `hx-confirm`.
- **E4 — MEDIUM · S–M** No template uses `hx-indicator` — form submits have no in-flight feedback (compounds B7). Add a shared `.htmx-indicator` spinner in the base layout and apply to primary CTAs.

### F. Misc polish *(pixel)*

- Icon semantics: `pencil` used for "add notes" (run.html:588) — `notebook-pen` is more precise (**LOW**).
- Inline `onmouseover/onmouseout` style mutation on active-session cards (dashboard.html:28-43) bypasses CSS transitions — move to Tailwind hover utilities (**LOW**).
- Back-navigation pattern is inconsistent (icon arrow-left vs text "← …") — a shared back-link partial would reduce drift (**LOW**).

---

## 4. Prioritized remediation plan

### P0 — Broken / embarrassing UI (ship immediately, all S)
| Item | Resolves |
|---|---|
| `--danger` → `--destructive` (login, signup) | A1 — auth errors render unstyled |
| `--card` → `--panel` (calendar, base) | A2 — transparent card backgrounds |
| Replace `ring-ring` with a real accent outline | A3 — invisible calendar selection |
| Remove "MVP next:" developer copy | A4 — dev copy in production |

### P1 — High-impact UX (wire existing infrastructure into daily flows)
| Item | Effort | Resolves |
|---|---|---|
| Route guided-run finish + completion → `/runs/{id}/summary` | S | B1 (journal dead-end) |
| Add `"working"` case to `tickStyleDisplay` | S | B3 (silent tick) |
| Trial back-arrow → `/dashboard` | S | B5 |
| First-run onboarding empty state with actionable CTAs | S–M | B6 |
| State-aware Start (resume existing run; hide on Done) | M | B4 |
| Climbing analytics on History (pyramid, hardest-grade trend, send rate) | M | B2 |
| Run-action feedback via `#run-feedback` + `hx-indicator` | S–M | B7, E4 |

### P2 — Consistency & accessibility sweeps
| Item | Effort | Resolves |
|---|---|---|
| Hardcoded hex → tokens (amber/green/destructive/heatmap/CSS) | M | C1 |
| `h2` → `h1` across the 12 pages | S–M | D1 |
| `aria-label` on icon-only controls; `<span>`→`<a>`; checkbox names | M | D2, D3, D6 |
| Keyboard-accessible calendar day cells | S–M | D4 |
| Hover-reveal: `focus-within` + `md:` prefix | S | D5 |
| Standard empty-state pattern (templates, dashboard week, cycles) | S–M | C2 |
| Form-label + select `text-sm` normalization | S | C2 |

### P3 — Polish
| Item | Effort | Resolves |
|---|---|---|
| `hx-target="this"` on tick section; un-nest from complete-form | S | E1, B11 |
| `hx-confirm` on cycle session removal; native→`hx-confirm` | S | E2, E3 |
| Climbing-log quick-action outcome row (taps-down) | M | B8 |
| PR / personal-best markers | M | B10 |
| Manual-log draft-on-GET leak | M | B9 |
| Icon-size scale normalization; px→rem; touch-target `btn` | S | C3, D9 |
| Back-link pattern, icon semantics, hover-JS → CSS | S | F |

---

## 5. What's done well

An honest balance — the experts repeatedly flagged genuinely strong work:

- **The run player is Crimpd-class.** Timer-centric layout, per-set log pre-filled from planned sets ("tap-to-confirm, not tap-to-enter"), and "up next" rep/set counters are the strongest part of the app.
- **The climbing-tick architecture is excellent.** Constant inheritance, the windowed grade-chip strip centered on the anchor grade, the live session header, "Log again", and the **novel per-climb Focus field** (pre-climb intention — no mainstream app structures this) are best-in-class. It needs the Working-badge fix and a taps-down pass, not a redesign.
- **Conflict detection at cycle creation** (blocking calendar events surfaced before generation, with "create anyway / skip conflicts") is more sophisticated than most consumer apps.
- **The reflection infrastructure already exists and is good** — journal form + previous-run comparison on the summary page. The fix is exposure, not construction.
- **The design system is mostly token-driven** with documented dark-mode variants; the hardcoded-hex findings are the exception against a real, followed rule (`DESIGN.md`).
- **Correct patterns already live in-tree** — `manual_exercises.html` has the right `hx-target="this"`, `exercise_library` has correct `aria-label`s, training_log.html already uses `h1`. Most fixes propagate an existing good pattern rather than inventing one, which keeps the remediation low-risk.
