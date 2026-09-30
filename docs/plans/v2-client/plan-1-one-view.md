Plan 1: one adaptive view

Checked against the installed svelte 5.57.1, @sveltejs/kit 2.70.3, tailwindcss 4.3.3 and svelte-check 4.7.6 (node_modules). Counts come from the repo on `rewrite`. The script for the CSS counts is impl-plans/css_usage.py.

1. Approach

- Every screen is one component tree. Phone and PC get the same parts, in the same order, with the same data and labels.
- The shell (the nav) switches with a media query. Parts switch with container queries, so a part lays itself out for the width it gets, whether that is a phone, a PC column or a side pane.
- A JS layout switch (`MediaQuery` from svelte/reactivity) only where CSS cannot do it. Its own doc comment says: "If you can use the media query in CSS to achieve the same effect, do that." (svelte/src/reactivity/media-query.js).
- The look lives in one file of tokens. Screens and parts read only tokens, so a new look (the blended fourth one too) changes that one file.
- Why: principle 11 asks for "One set of parts, in the gym and at home, on every platform", and says "Don't: a history card with a layout of its own." Principle 12 says "A larger screen shows more of the plan, not more controls."

2. Foundations

Tokens (new client/src/tokens.css, next to passion.css)
- Colour, one value per theme: ground, surface, surface-2, line, edge, ink, ink-2, ink-3, cta, cta-ink, ok, tick, rest (amber, rest only), bad, focus. The session's colour is `--session`, set inline from `template.color`, as the mockup's `--tpl`.
- Shape: radius-block, radius-control, radius-button, radius-check, radius-chip. Blocks: block-bg, block-rule, block-pad, block-shadow. With these, Logbook is "no card, a top rule" and Chalk is "a card", and no screen has to change.
- States: row-now-bg and row-now-ink (Logbook inverts the current set). Nav: nav-inset and nav-radius (Chalk's floating bar, Logbook's flush bar).
- Type: font-body and font-num. Sizes: label 12, body 15, input 16, head 20, title 30, phase 40, timer, timer-rest. These are the mockup's Logbook sizes, written in rem so that the phone's text size scales them (principle 10). title-case, title-stretch and num-weight are tokens too.
- Motion: ease, quick (at most 300 ms, principle 7) and press.
- Values come from the picked look's lines in passion-looks.html (lines 63–73).
- None of the new names exists in passion.css (grep), so old and new screens can run side by side.
- The limit: beyond its tokens, the mockup has 48 lines of Night board rules and 74 lines of Logbook rules. The move turns each of them into a token. Two of them cannot be a token: Logbook's ruled section list against Chalk's chips, and Night board's full-screen hang colour. Both are structure, and are fixed when the owner picks the look.

Light and dark
- tokens.css puts light on `:root` and dark on `:root[data-theme="dark"]`, so each token is written twice. passion.css writes each one 4 times (for example `--muted` at lines 16, 69, 121 and 171).
- app.html sets `data-theme` to the pinned choice, or else to the system's. A `matchMedia('(prefers-color-scheme: dark)')` change listener keeps it in step while nothing is pinned. Today the page reads the system once at load. Header.svelte `setTheme` pins on the first toggle, with no way back to System.
- `color-scheme` is set per theme, so native selects, date pickers and scrollbars match.
- Settings gets System, Light and Dark as one Chips group.
- `light-dark()` is not used. MDN says it is Baseline "since May 2024", which is later than Tailwind 4's floor (Safari 16.4, March 2023, tailwindcss.com/docs/compatibility).

Tokens into Tailwind
- Today the two are not connected. app.css imports tailwindcss, then passion.css into layer(components), and there is no `@theme` anywhere in client/src (grep). Markup reaches a token only through an inline style or an arbitrary value: 19 lines in 10 files, for example `hover:bg-[var(--card-muted)]` in ExercisePicker.svelte. Every `text-sm` and `rounded-md` is Tailwind's own default.
- In app.css, `--color-*: initial` comes first, so Tailwind's palette is gone. Then one `@theme inline` block maps names to tokens: `--color-ink: var(--ink)` gives `text-ink`, `bg-ink` and `border-ink`. The same goes for `--radius-*` and `--font-*`, and for `--text-title` with its `--line-height`, `--font-weight` and `--letter-spacing` sub-keys.
- `inline` is needed because a look overrides tokens on a subtree (Logbook's current-set row, Night board's hang). A utility must resolve `var(--ink)` where it is used, not at `:root`.
- Checked in 4.3.3: `@theme` takes the flags inline, reference, default and static (dist/lib.mjs). `--color-*: initial` clears the namespace (Theme.add in dist/chunk-5JIJA4QV.mjs). `text-*` reads the three sub-keys (lib.mjs). `@container` and the `@md:` and `@max-md:` variants come from `--container-*` (theme.css: md is 28rem, 4xl is 56rem). No markup uses the palette and passion.css has no `@apply`, so clearing the palette breaks nothing.
- New client/src/base.css (in `@layer base`) holds page-wide rules that old screens do not set: tap highlight and focus ring. passion.css's html and body rules stay until it is deleted.
- Each part keeps its own rules in a scoped `<style>` block, so deleting a part deletes its CSS. Svelte scopes rules inside `@container` (the Atrule handler in compiler/phases/3-transform/css/index.js) and warns with `css_unused_selector`. passion.css shows what global CSS turns into over time: the client uses 56 of its 436 class names.

Parts (flat in lib/, as today). Existing file in brackets.
- Screen: the frame, ground, ink, font, `@container`, safe areas and `100dvh` for the player [the `passion-container` in routes/+layout.svelte, and the card headers of all 9 pages].
- Nav: one `<nav>` with four links. It is a bottom tab bar below `lg` (64rem) and a left rail from `lg` on [Header.svelte, 121 lines, and its 19 site-header, site-nav and theme classes].
- Block: a titled section [the `card card-pad` sections].
- Button: primary (one per screen), outline, plain and icon. At least 48 px on gym screens [.btn and the btn-* classes, RowActions.svelte].
- HoldButton [new].
- Field: label, input, select, textarea, hint and error, all at 16 px [the .input rows in ExerciseForm, TemplateEditor, SectionEditor, ChoiceEditor, StepEditor, DurationInput, ExercisePicker, login and signup; FormError.svelte].
- Chips [new; the template colour row is the nearest thing today].
- Stepper: − value +, and the value can be typed [new].
- SetTable, with ClimbRow on the same grid [new].
- SessionMark: the session colour with planned, started, done or missed [the colour dot in templates/[id] and TemplateEditor; the left border and colour bar in the templates list].
- SessionOutline [TemplatePlan.svelte and its 9 preview-flow classes].
- WeekStrip, MonthGrid, Agenda (Agenda also serves History rows and an exercise's last sessions), Timer, Stats, BestCard, Chart and Toast [all new].
- ListRow: one row whose extra columns show from a container width up [the phone and PC forks in the exercises and templates lists].
- Kept and restyled when their screen moves: Icon.svelte (add lucide shapes, same licence), Notes, ExercisePicker and the four editors. No approach changes api.ts, session.ts, exercise.ts, template.ts or filters.svelte.ts.

3. Order of work: about 80 commits, one change each

- Each part lands in its own commit, just before its first user.
- A moved screen gets a drop commit after it only if it was the last user of some passion.css rules. css_usage.py shows 0 uses before a drop.
- By today's per-file class lists, the template list and the template page free nothing, so they get no drop commit. That gives 80. A drop commit more or fewer moves the total by one.

- Foundations, 1–8: follow the system theme until one is pinned · add the look's tokens · map the tokens into Tailwind in place of its palette · add page-wide base rules · serve Archivo (only on the owner's yes) · docs: describe the new look in DESIGN.md · docs: replace "The frontend follows V1" · passion.css: drop the rules no screen uses (owner's call).
- Routes, 9–10: move the signed-in pages into an `(app)/(shell)` group, with no URL change · render the header from the shell layout and from the sign-in pages. The player at `(app)/run/[id]` then has the sign-in guard but no nav.
- Player first, 11–43: let pages draw under the notch (`viewport-fit=cover`, which makes passion.css's 4 `env()` uses live) · part Screen · part Button · part StepHead · open a run in the player · part HoldButton · finish a run with Hold to end · keep the run clock right after the phone sleeps (time from the clock, mirrored to localStorage) · part Timer · play a timed-reps step · sound and buzz when a phase ends · keep the screen on while a timer runs · part Stepper · part SetTable · log a sets step filled in from last time (the head of `GET /exercises/{id}/history`) · count rest down on the Done button · show a set that did not save, with Retry · part Chips · log a climb · play an open step · pick from a choice when the player reaches it · skip · go back · swap · add · reorder · resume a run · part Stats · part BestCard · show the summary · part Field · write the journal · start a session from a template page.
- Today, 44–49: add a settings screen (theme, grade scales, sign out) · part Nav, in place of Header's links · part SessionOutline · part WeekStrip · show Today as the home screen, in place of the /templates redirect · a first Today for an empty account.
- Plan, 50–58: part SessionMark · part MonthGrid · part Agenda · show the plan · part Toast · move a session, with Undo · add a one-off session · edit a cycle · plan a new cycle.
- Look back, 59–61: History by week · part Chart · an exercise's history.
- Existing screens, 62–80: part ListRow · exercise list · drop the lib-row classes · exercise pages · drop lib-edit-grid, lib-edit-side and lib-edit-media · a read-only view for shipped exercises · template list · template page · template editor · drop template-color, passion-disclosure, preview-flow and lib-edit-sticky-bar · confirm before a step is removed · 48 px row buttons · sign-in pages · drop btn-accent · error page · drop btn, btn-ghost, card and card-pad, if it is their last user · remove Header · drop its rules · delete passion.css, with anything left over.

What the new screens wait on. Checked in server/api/api.go: runs, sets, climbs, finish, grades, cycles and scheduled sessions all exist. So the player, Today, Plan, Cycle, Summary and Exercise run on the live API.
- The run and cycle tables still wait for the owner's review (V2_DESIGN parts 3 and 4).
- History's "23 sets" needs counts that the run list does not return (runSummaryBody in run.go).
- "Not synced yet" waits on offline recording (rules 49 and 50) and on a run id chosen by the client, which V2_DESIGN leaves for later.
- Undo after deleting a run needs a restore that does not exist.
- Today's "≈ 75 min" needs a rule for estimating a session's length.
- The day list has no template colour, so the client reads the colour from the template list it already loads.

4. How phone and PC differ

- The Plan grid at 390 px: one column. The cycle Block holds a MonthGrid of 7 columns with 42 px cells, each with one SessionMark, and today is outlined. Then the legend, This week as an Agenda, and "Add a session". An Undo Toast sits above the bottom tab bar.
- The Plan grid at 1280 px: the same `<nav>` becomes a left rail. The page's container passes 56rem (`@4xl`, a proposal), so the grid moves to a wide left column and This week to a right column. MonthGrid cells grow tall and show the session's name next to its mark. The name is already in the DOM as the cell's accessible name. Nothing new is added: no control, no data.
- Log sets at 390 px: the player fills the screen with no nav. It shows StepHead, then a SetTable of 5 columns (set, last time, added, reps, a 48 px tick), two Steppers, Done (which becomes "Rest 1:58"), Up next, and Hold to end.
- Log sets at 1280 px: still no nav. The step column keeps its phone width, because on a gym screen the gym posture wins. Past 48rem the player adds a second column with SessionOutline, where you are in the session. On a phone that sits behind Up next. The Stepper value can be typed from a keyboard. The timer numeral is capped by `cqi`, so it fits its column. Tailwind's docs list `cqw` and `cqi` as arbitrary values.
- A JS switch, in two places at most. TemplateEditor's live plan should not render on phones: today `hidden md:block` still renders it on every keystroke. The second place is History list-and-detail, only if the owner asks for it.
- Where one view loses:
  - A table and cards are two structures. The library lists become one row markup and lose `<table>` semantics.
  - A list beside its detail does not fit CSS well, so it is left out.
  - Every part carries both layouts, so even a phone-only tweak must be checked at 1280.
  - The mockup draws no PC layout at all (d-*.png are phone frames in rows), so each wide layout is a new design call.

5. The cost of two views, measured

- Today there are 9 screens: 12 route .svelte files (9 pages, 2 layouts, 1 error page) and 13 components, 2,127 lines of .svelte in all. 6 .ts modules (405 lines) are shared under any design.
- A two-view design needs a second version of every file whose layout changes with width. That is 7 files: the root layout, Header, both lists, exercises/[id], TemplateEditor and ExerciseForm. Together they are 1,075 lines, half of all the .svelte lines. SectionEditor's `sm:` classes change padding only.
- The code already forks 139 lines: exercises 104–120 and 124–167, templates 92–118 and 121–171.
- Of the 10 mockup screens, 3 widen under the proposed PC default (section 8): Plan, History and Exercise. If Today, Cycle and Summary widen too, it is 6. The 3 player screens and Empty Today never widen.
- A new field on a form. Example: the template's `needs` field, `tpl-needs` in TemplateEditor.svelte plus the draft and labels in template.ts. One view: 2 files. Two views: 3.
- A new screen. Example: History on `GET /api/v1/runs`. One view: +page.ts, +page.svelte and a Nav link, so 3 places. Two views: +page.ts, 2 views, the switch between them and the Nav link, so 5.
- A bug fix in a list. One view: 1 place. Two views: 2. The code shows the two copies drift apart:
  - The exercises list names the kind for screen readers in the phone row (`sr-only`, line 111), but the table cell has only a `title` (line 143).
  - The templates list styles the source chip two ways (lines 106–111 against 158–163).
- A copy change. A string in markup, such as "Pick at least" in ChoiceEditor.svelte: 1 place against 2. A string in a module, such as "Rep time (s)" in `countLabels`: 1 in both. The templates list already says its count two ways: "3 sections" on a phone, and a bare number under "Sections" on a PC.
- A token change: 2 lines (light and dark) in both designs. The screenshots are the same 4, but under two views they test twice the code.
- Verdict: at today's size, two views would copy about 1,075 lines and build 3 to 6 new screens twice. The forks the client already has have drifted, even inside one file. Two views buy freedom for each layout. Nothing in the mockup needs that freedom yet, because it draws no PC layout. One view costs conditional CSS in some parts, and a check at both widths on every commit.

6. How each commit is checked

- `pnpm --dir client check`, which is svelte-check with the compiler's a11y and unused-CSS warnings, is clean. So is `pnpm --dir client build`.
- Screenshots come from the scratchpad's shot.mjs (headless Firefox over BiDi, which sets `passion-theme`). Every screen the commit touches, and every screen that uses a touched part, gets 4 shots: 390×844 and 1280×800, each in light and dark. An old screen's shots must not change on a new-screen commit or a CSS drop.
- Theme, in commit 1: with nothing pinned, the page follows an OS change without a reload. With a theme pinned, it does not.
- Keyboard: tab order follows the reading order, and every control shows a focus ring at 3:1. Esc closes a dialog, and there is no focus trap. A click on Hold to end that did not come from a held press (a keyboard, or a screen reader's double tap) opens a confirm dialog, as plan.md asks. Test it with both.
- Screen reader: Orca (installed at /usr/bin/orca) on the desktop. VoiceOver or TalkBack on the owner's phone for gym screens. From principle 10: run a hangboard session and log a climb, and check that the reader says "rest over" once. The digits are `role="timer"`, with a separate polite region for phase changes.
- Text at 200%: the phase and the time do not clip.
- Contrast for every token pair, in light and dark: 4.5:1 for text and 3:1 for edges.
- Hit areas of at least 48 px on gym screens.
- With reduced motion on, motion becomes a fade, and the hold still fills.
- The pixel agent reviews template and CSS changes, copy reviews new text, and simplify reviews each feature. A review every few commits.

7. Risks, and what is left out

- For many commits the app shows two looks side by side. That goes against principle 11 until the last screen moves.
- The new parts must not use the class names `.input`, `.card`, `.btn`, `.muted` or `.link` while passion.css exists.
- passion.css's html and body rules (the Inter font, the background) still reach new screens. Screen must set its own.
- A Tailwind utility that does not exist builds nothing and gives no error, for example `bg-blue-500` after the palette is cleared. The same goes for a `@md:` outside any `@container`. Only the screenshots catch these.
- Parts built on the run and cycle tables before the owner signs them off may need rework.
- Wake Lock needs HTTPS in production. iOS Safari is reported to have no vibration, from secondary sources only (research-svelte-mobile.md).
- Left out:
  - Server work: run-list counts, restoring a deleted run, a client-chosen run id, a planned weight.
  - Offline recording and a service worker.
  - A lock-screen or watch timer.
  - A PWA manifest and Capacitor.
  - List-and-detail on a PC, and drag to move on the calendar (tapping works).
  - Streaks, the heat map and weekly bars.

8. Owner decisions

- The look. tokens.css is the only file it changes. The blended fourth look is a new tokens.css.
- Archivo: a new font file for the client, served by our own server. Check its licence and measure its size before it goes in.
- The bottom tab bar (Today, Plan, History, Library) on phones, and the same four as a rail on a PC.
- Where settings live (theme, grade scales, sign out). The mockup has no entry point for them.
- Each PC layout, before it is built. Proposed default: the phone column, centred. Only Plan, History, Exercise and the Library widen.
- Library lists as rows with extra columns, giving up `<table>`. Recommended.
- Whether to drop now the 380 passion.css class names no screen uses (commit 8). Recommended: yes.
- The review of the run and cycle tables, which the player and Plan build on.
