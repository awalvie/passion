# Implementation prompt — Passion design/UX remediation

> Hand this to an LLM coding agent working in the Passion repo. It is self-contained: it carries the context, the house rules, and the exact, code-verified fixes (ordered by priority) with acceptance criteria. Findings were produced by a two-expert audit and then adversarially re-verified against the source — the corrections are folded in.

---

## Role & context

You are implementing UI/UX fixes in **Passion**, a climbing-training web app at the repo root. Stack: **Go 1.25, Chi router, GORM + SQLite, server-rendered Go HTML templates, HTMX, Tailwind (Play CDN — no build step), Lucide icons.** Templates live in `templates/` (pages) and `templates/fragments/` + `templates/layouts/` (partials); they are compiled at startup by `pages/pages.go`, so a malformed template fails the server on boot. Styles are in `static/passion.css`. Handlers are in `http/server/`.

## House rules (non-negotiable — from CLAUDE.md / docs/DESIGN.md)

1. **Never hardcode hex colors.** Use the semantic CSS tokens in `static/passion.css` (`--text, --muted, --bg, --panel, --card-muted, --border, --accent, --accent-bg, --accent-border, --destructive`) and the climbing-tick palette (`--tick-*`). Both light and dark themes are defined; any color you add must work in both.
2. **`<select>` elements** always get `class="input text-sm"` (or `class="input"`).
3. **HTMX:** any lazy-load element inside a `<form>` must set `hx-target="this"` (HTMX inherits `hx-target` from ancestors and will swap into the wrong place otherwise).
4. **Comments:** none unless the *why* is non-obvious. **Error handling:** don't handle impossible states. **Prefer editing existing files** over creating new ones.
5. **Docs:** after a change, update `readme.md` / `docs/DEVELOPMENT.md` / `docs/DESIGN.md` if the change touches config, structure, workflows, or design tokens. (New tokens → add to the DESIGN.md token table.)
6. **Verification:** after each batch, run the app and confirm templates parse and the change renders. Use the `run-passion` skill (build + run + screenshot). A passing `go build ./...` is necessary but NOT sufficient — template/CSS errors only surface at runtime.
7. **Do not `git commit` or `git add`** unless explicitly asked. Implement and stop.
8. **User-facing copy** you add (footer, empty states): keep it concise and consistent with the app's plain, low-chrome voice. Flag any copy you invent so it can be reviewed.

Work top-down by priority. Each item lists the **exact location**, the **verified fix**, and **acceptance criteria**. Tackle one priority band at a time; verify before moving on.

---

## P0 — Broken render & leaked dev copy (do first; all small, low-risk)

### P0.1 — Auth error banners render unstyled (`var(--danger)` is undefined)
- **Files:** `templates/login.html:7`, `templates/signup.html:7`
- **Problem:** Both use `style="border-color: var(--danger); color: var(--danger)"`. `--danger` is defined nowhere (the token is `--destructive`, defined for both themes at `static/passion.css:32,72,112,152`). The undefined var falls back to inherited/initial values, so the error box renders in plain body color with a default border — **it does not read as an error** (not invisible, just unstyled).
- **Fix:** replace both `var(--danger)` occurrences on each line with `var(--destructive)`.
- **Accept:** a failed login/signup shows a red-bordered, red-text error banner in both light and dark themes.

### P0.2 — Transparent surfaces (`var(--card)` is undefined)
- **Files:** `templates/calendar.html:103`, `templates/layouts/base.html:31` and `:32`
- **Problem:** `var(--card)` is undefined (tokens are `--panel` and `--card-muted`). The calendar event-sidebar cards and the markdown editor's writing surface render with a transparent background.
- **Fix:** replace `var(--card)` with **`var(--panel)`** in all three spots. (Justification: the editor *writing surface* is the input analog, and `.input` uses `var(--panel)` at `static/passion.css:323`; the editor toolbar correctly stays `--card-muted`. The calendar item is a card → `--panel`. So `--panel` is correct for all three — do **not** use `--card-muted` for the writing surface.)
- **Accept:** calendar event cards have a solid panel background with their colored left border; the journal markdown editor's text area has a solid background distinct from its toolbar, in both themes.

### P0.3 — Calendar selected-day highlight uses an undefined utility (`ring-ring`)
- **Files:** `templates/dashboard.html:252` (remove) and `:262` (add); add a rule to `static/passion.css` (next to `.cell-base`, ~line 337).
- **Problem:** the JS toggles Tailwind classes `ring-2 ring-ring`. There is no Tailwind config (Play CDN), so `ring-ring` resolves to no color; `ring-2` alone renders Tailwind's **default blue** ring — i.e. the selected day is highlighted in the *wrong* (non-accent) color, not invisible.
- **Fix:** add a real selected-state class and toggle it at **both** JS sites:
  - In `static/passion.css` after `.cell-base { … }`:
    ```css
    .calendar-cell-selected {
      outline: 2px solid var(--accent);
      outline-offset: -2px;
    }
    ```
    (`--accent` is the documented token for "selected/active" state.)
  - `dashboard.html:252`: `selectedCell.classList.remove("ring-2", "ring-ring")` → `selectedCell.classList.remove("calendar-cell-selected")`
  - `dashboard.html:262`: `cell.classList.add("ring-2", "ring-ring")` → `cell.classList.add("calendar-cell-selected")`
- **Accept:** clicking a calendar day outlines it in the app accent color; clicking it again or selecting another clears the previous outline (no stale outline). Works in both themes.

### P0.4 — Developer copy shipping to users (two sites)
- **Files:** `templates/training_cycles.html:58`; `templates/layouts/fragments/footer.html:4-5`
- **Problem:** `training_cycles.html:58` empty state reads "MVP next: create your first cycle…". `footer.html` renders, on **every page**, "Workout session tracker MVP (Go + HTMX + Tailwind)." and "Next: training cycles + guided session runs." — internal/dev language that also advertises already-shipped features.
- **Fix:** replace with plain user-facing copy. Suggested (review/adjust tone):
  - `training_cycles.html:58` → "No training cycles yet. Create one to generate a weekly schedule of sessions."
  - `footer.html` → a single concise line, e.g. "Passion — plan, run, and log your climbing training." (drop the "Next:" roadmap line).
- **Accept:** grep the templates for "MVP" and "Next:" → no user-facing matches. Footer reads cleanly on every page.

---

## P1 — High-impact UX: wire existing infrastructure into the daily flows

### P1.1 — Guided runs never reach the journal/summary (highest leverage)
- **Files:** `http/server/run_actions.go:53` **and** `http/server/runs.go:408` (BOTH redirect sites), plus the "Workout complete" card `templates/run.html:21-28`.
- **Verified context:** `handleRunStop` already creates an empty journal for every run (`run_actions.go:44-51`); `run_summary.html:71-90` already lazy-loads that journal form + a previous-run comparison; `handleRunJournal` has no open-only gate, and `handleRunSummary` renders completed guided runs correctly (its only redirect guard bounces *in-progress non-open* runs). So routing finished guided runs to the summary is safe.
- **Problem:** two paths dead-end instead of going to the summary: (1) the **Finish** button → `/stop` → currently redirects guided runs to `/dashboard` (`run_actions.go:53`); (2) **auto-complete on the last step** currently redirects to the run page `#run-current-step` (`runs.go:408-411`). If you fix only one, the other still skips the journal.
- **Fix:** route both completion paths for guided (non-open) runs to `/runs/{id}/summary`. Update the `RunCompleted` card's only action to point at the summary too.
- **Also (fold in):** `run_summary.html:86` journal lazy-load lacks `hx-target="this"`. It self-targets today, but since this page becomes the post-run surface for *every* run, add `hx-target="this"` per the house rule.
- **Accept:** finishing a guided run (via Finish OR by completing the last exercise) lands on `/runs/{id}/summary` with the journal form (RPE/sleep/energy/notes) and previous-run comparison visible. Open and manual runs are unaffected.

### P1.2 — "Working" climbing result renders no badge (silent data)
- **Files:** `http/server/climbing_ticks.go:15-33` (`tickStyleDisplay`); form at `templates/fragments/run_ticks.html:469`.
- **Problem:** the form offers `value="working"` but `tickStyleDisplay` has no `"working"` case, so it returns empty strings → the tick shows no style chip and no badge (only the faint `tick-card--working` tint), indistinguishable from a half-logged tick. (`styleImpliesSent("working")==false` is correct — leave send semantics alone.)
- **Fix:** add a `case "working"` to `tickStyleDisplay` returning a label ("Working"), a CSS class, and an icon (e.g. `hammer` or `loader`), consistent with how `hangdog` is handled. Ensure the class has a tick-palette style if the others do.
- **Accept:** logging a climb with result "Working" shows a visible "Working" badge in the tick list, in both themes.

### P1.3 — `Start` creates duplicate runs; shown on already-Done sessions; no resume
- **Files:** `http/server/scheduled_sessions.go:40-67` (the `start` action); `templates/dashboard.html:149-158` (Start button) + `:129-133` (Done badge).
- **Problem:** `start` creates a `SessionRun` unconditionally with no check for an existing run on that scheduled session; the dashboard Start button has no `{{ if not .Done }}` guard and is `hx-swap="none"` with no debounce, so a double-tap or a revisit creates duplicate RUNNING runs.
- **Fix:** in the `start` handler, look up an existing run for that `scheduled_session_id`; if one is RUNNING, redirect to it instead of creating a new one (prefer an app-level check over a DB unique index, to allow legitimate re-runs later). On the card, make the action state-aware: Start (none) → Resume (running) → View/Repeat (completed). The dashboard already loads `activeRuns` and `completedWeekSSIDs` (`dashboard.go:240-256`) — use them.
- **Accept:** double-tapping Start yields one run; a Done session shows Resume/View, not Start; resuming returns to the in-progress run.

### P1.4 — History shows zero climbing analytics
- **Files:** `http/server/history.go` (handler `:33`, aggregates `:189-334`); `templates/history.html`.
- **Verified:** `history.go` never references `ClimbingTick` (grep confirms zero matches) — all stats derive from `SessionRun` completions. The app's signature data is collected but never reviewed.
- **Fix:** add a climbing-analytics section to History over `ClimbingTick` rows joined to runs within the existing range filter: a **grade pyramid** (count by grade, attempted vs sent), a **hardest-sent-grade trend**, **send rate**, and indoor/outdoor + board/wall splits. No schema change; reuse the range filter and existing per-tick query patterns. Follow the low-chrome, data-dense History style; use `--tick-*` tokens for grade/outcome colors.
- **Accept:** a user with logged ticks sees a grade pyramid and hardest-grade trend on History for the selected range; a user with no ticks sees a clean empty state.

### P1.5 — State-changing run actions give no feedback
- **Files:** `templates/run.html:638,646` (stop/delete/finish), `dashboard.html:32-42`; the existing `#run-feedback` element + handler at `run.html:1310-1332`.
- **Fix:** reuse the `#run-feedback` toast for stop/finish/delete (show "Finishing…/Deleting…" on submit), and add a shared `hx-indicator` spinner (define `.htmx-indicator` in `layouts/base.html`) applied to primary form CTAs app-wide.
- **Accept:** triggering Finish/Stop/Delete shows immediate visual feedback before the redirect lands; a double-tap doesn't produce a confusing error.

---

## P2 — Consistency & accessibility sweeps (batch by kind)

- **Hardcoded hex → tokens** (dark-mode-breaking): amber warning UI → `--tick-amber-*` (`dashboard.html:77-85`, `new_cycle.html:12,14`); green Done/complete → `--tick-sent-accent`/`--tick-green-*` (`dashboard.html:130,292`, `history.html:161,164`, `passion.css:2783,2788,2832`); destructive → `var(--destructive)` (`profile.html:8`, `training_log_new.html:25`); heatmap legend → generate from the JS `tiers[]` array instead of hardcoded indigo (`history.html:47-50`); CSS one-offs (`passion.css:2415,2423,2451,2459`). If you introduce a warning token family (`--warn-bg/-fg/-border`), add it to all four theme blocks and document it in DESIGN.md.
- **Headings:** add an `h1 class="text-xl font-bold"` (or promote the top `h2`) on the 12 pages lacking one: `training_cycles, templates, exercise_library, profile, new_template, new_cycle, new_exercise_library, edit_exercise_library, login, signup, new_activity_template, activity_templates`.
- **Accessible names / keyboard:** add `aria-label` to icon-only controls (`training_log.html:48,56,63,68`, `open_exercise_panel.html:5`, `open_template_panel.html:5`, `dashboard.html:28`); convert the Resume `<span>` (`dashboard.html:43-45`) to an `<a href>`; make calendar day cells keyboard-operable (`dashboard.html:191` — `role="button" tabindex="0"` + Enter/Space, or `<button>`); add `aria-label` to library checkboxes (`exercise_library.html:49-51,72-76`); give meaningful `alt` to exercise thumbnails (`run.html:379,407`).
- **Hover-reveal actions:** add `focus-within:opacity-100` and a `md:` prefix so actions are reachable on touch + keyboard (`training_log.html:45`, `history.html:181-192`).
- **Empty-state pattern:** bring `templates.html` (renders a bare empty table), `dashboard.html:166`, and `training_cycles.html:57` to the documented `card card-pad text-center py-12` + muted-icon pattern.
- **Form polish:** normalize `new_cycle.html` labels to `text-xs font-semibold muted`; add `text-sm` to selects at `exercise_library.html:21,25,32`; add `btn` base class to small primary buttons missing it (`dashboard.html:66`, `history.html:179`).

## P3 — Polish

- `hx-target="this"` on the run tick `<section>` (`run.html:507`) and un-nest it from `<form id="complete-form">` (`run.html:499-516`) — invalid nested form, works only by parser luck.
- `hx-confirm` on cycle session removal (`training_cycle_detail.html:79,141`); replace native `onsubmit="return confirm()"` with `hx-confirm` (`exercise_library.html:130`, `templates.html:64`).
- Climbing-log **outcome quick-action row** (Flash · Send · +Attempt · Working) that submits and infers `Style` server-side, to cut taps-per-climb; keep the 5-way detail in the expanded edit view (`run_ticks.html:465-470`).
- PR / personal-best markers ("new hardest grade sent") on the run summary + a star in History.
- Manual-log draft created on GET leaks onto the dashboard (`training_log_manual.go:46-60`) — create the draft lazily on first add, or sweep empty drafts.
- Icon-size scale normalization (snap ad-hoc `0.65/0.7/0.8/0.85rem` to `0.6rem`/`0.75rem`; px→rem); shared back-link partial; `pencil`→`notebook-pen` for "add notes"; replace inline `onmouseover/onmouseout` with CSS hover.
- Sanity-check `handleRunSummary` previous-run % math (`run_actions.go:258-259`) — it compares a prior run's completed count against the *current* run's total.

---

## Definition of done (per band)

- App boots (`pages.NewPages` compiles all templates), the changed screens render correctly in **both** light and dark themes, and no hardcoded hex was introduced.
- House rules upheld (tokens, `select` class, `hx-target`, no gratuitous comments).
- Relevant docs updated if config/structure/tokens/workflows changed.
- Report what you changed, what you verified (and how), and anything you skipped or left for review (including invented copy). Do not commit.
