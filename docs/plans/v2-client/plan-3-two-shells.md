Plan 3: shared parts inside two shells

1. Approach

- One set of parts and screen bodies. A phone shell (tab bar, one pane, full-screen player) or a
  desk shell (side nav, panes side by side) places them.
- A body is sized by its pane, never by the viewport: no `sm:`/`md:`/`lg:` class. Where it must
  adapt, it uses a container query (Tailwind 4.3.3 ships `@container` and `@`-variants, dist/lib.mjs).
- One layout, `(app)/(shell)/+layout.svelte`, renders `lib/shell/Shell.svelte`. Shell reads
  `new MediaQuery('min-width: 64rem')` (svelte 5.57.1, src/reactivity/media-query.js, live on
  `change`) and sets `data-shell="phone|desk"`. The player's `(app)/run/` layout uses Shell with no nav.
- Panes are nested routes. A route layout hands its list and its `children` to `Panes`. The URL
  names what the second pane shows (`/history/exercises/{id}`), so one URL means one thing in both
  shells. A row tap pushes history. Back goes to the list on a phone, to the last detail on desk.
- Tablet and resize: under 64rem the phone shell, from 64rem the desk shell, so a rotated tablet can
  swap shells. `children` renders at one fixed place in Shell, and Panes hides a pane with `hidden`
  instead of unmounting it. A rotation swaps the nav and the pane count, and a cycle draft or a
  running timer survives it.

2. Foundations

- Theme: `app.html` stores `light` or `dark` only when pinned; with no pin it follows
  `prefers-color-scheme` and listens for changes. Today it resolves once at load (app.html:7-15),
  so a system change mid-session is missed. It always sets `data-theme`, so CSS needs one light and
  one dark block. Tailwind's `dark:` follows the OS, not the attribute (dist/lib.mjs,
  tailwindcss.com/docs/dark-mode), so parts use tokens and never `dark:`.
- The look is one file, `client/src/look-logbook.css`: `:root {light}` and
  `:root[data-theme=dark] {dark}`, each with `color-scheme`. Another look is another file and one
  import line in app.css. Token names follow the mockup's roles and clash with nothing in
  passion.css (grepped): colour (ground, surface, surface-2, line, edge, ink, ink-2, ink-3, cta,
  cta-ink, ok, on-ok, rest, bad, glass; `--session` per element from a template's colour), shape
  (radius-card, -field, -button, -tick, -chip), motion (ease, one duration under 300 ms).
- Type scale: note 12, body 15, field 16 (fields only, so iOS does not zoom), title 20, head 30,
  hero 40, plus the timer's size, rest size and weight, and a title stretch for Archivo. Where looks
  differ in structure, a token holds the keyword (`--sections-dir: column`). Most of the mockup's
  77 lines of Logbook rules set radius, stretch, size, colour or shadow.
- Tailwind is not connected today: app.css has no `@theme`. The one token-aware utility is
  `hover:bg-[var(--card-muted)]` (ExercisePicker.svelte:48); screens reach tokens through 16 inline
  `style="…var(--…)"` in 9 files. New `client/src/theme.css` holds `@theme inline { --color-ink:
  var(--ink); … }`, which gives `text-ink-2`, `bg-surface`, `rounded-card`, `text-body`,
  `font-look`. `inline` is the documented form for a token that is itself a var, and an imported
  file is the documented way to share one (tailwindcss.com/docs/theme). The new names leave
  `text-sm` and the rest alone, so old screens keep their sizes.
- New `base.css` in `layer(base)`: focus ring, tap highlight, reduced motion, `100dvh`. Each part
  keeps its rules in a scoped `<style>`, as ExerciseForm.svelte does. Built scoped CSS sits outside
  every layer (server/web/dist/…/ExerciseForm.*.css), so it beats passion.css in `layer(components)`.
- Parts, and what each replaces or grows from:
  - `lib/shell/`: Shell, TabBar, SideNav, Panes, and nav.ts, one list for both navs. They replace
    Header.svelte and `passion-container` in routes/+layout.svelte.
  - Button (fill, outline, plain) replaces the 23 lines that use `btn-primary|btn-ghost|btn-accent`.
    IconButton (48 px hit area) replaces RowActions.svelte's buttons and the back arrows. Field (label, input,
    select or textarea at 16 px) replaces `.input` in the 7 form components, the list filters,
    login and signup. ChipGroup also takes over TemplateEditor's colour row.
  - Outline grows from TemplatePlan.svelte and serves the template, the day and the run.
    SessionCard and SessionMark come from the colour spans in templates/. Row and Agenda replace
    the `lib-row` cards and both list tables. Block replaces `card card-pad`.
  - New: HoldButton, Stepper, TimerFace, SetTable, ClimbList, Best, UndoBar, WeekStrip, CycleGrid,
    Dots, Chart (plain SVG), EmptyState. Kept and restyled: Icon.svelte, Notes.svelte, FormError.svelte.

3. Order of work: 56 commits, one change each, an agent review every 2 to 4

- Base, 1-9: (1) docs: DESIGN.md for the new look, parts, shells and the pane rule. (2) docs:
  replace "The frontend follows V1" in V2_DESIGN.md. (3) follow the system theme until one is
  pinned. (4) add the Logbook token file. (5) map the tokens into Tailwind. (6) vendor Archivo, only
  with the owner's yes. (7) let the page reach under the notch (`viewport-fit=cover`; passion.css's
  4 `env()` insets start to work). (8) add the new base styles. (9) move the header out of the root
  layout into an `(old)` group: a group does not change a URL (kit src/utils/routing.js:118-124),
  and a root-layout header would sit on the player.
- Player first, 10-28. The server has every route it needs: runs, sets, climbs, finish, grades and
  exercise history (server/api/api.go:53, 59-68). (10) start a run from a template's page. (11) the
  player frame: focus layout, elapsed pill, Skip, HoldButton. (12) the timed step, from a
  clock-derived timer in `lib/timer.svelte.ts`. (13) a sound and a double buzz when a phase ends.
  (14) keep the screen on while a timer runs. (15) resume after the phone sleeps (localStorage, as
  V1). (16) announce a phase change once, in a polite live region. (17) the sets step: last time,
  Stepper, Done turns into the rest countdown. (18) the climbing step, in the person's grade scale.
  (19) the open timed step. (20) pick from a choice on reaching it. (21) the run outline, where
  Panes lands: an aside on desk, `/run/{id}/steps` on a phone. (22) go back to a finished step.
  (23) swap a step. (24) add a step. (25) reorder what is left. (26) the summary, with one best from
  exercise history. (27) the journal. (28) readme: the player.
- Shell and Today, 29-33: (29) the phone shell, holding Today for a new person at `/`. (30) the desk
  shell's side nav. (31) Today with a planned session, started from its day. (32) this week and the
  last 14 days. (33) docs: "Add a screen" in DEVELOPMENT.md puts width logic in the shell.
- Plan, 34-39: (34) the cycle grid and this week. (35) a day (`/plan/{date}`), beside the grid on
  desk. (36) move a session, with Undo, which is a second PUT. (37) edit a cycle (`/plan/cycles/{id}`),
  beside the grid on desk. (38) add a one-off session. (39) readme: Today and Plan.
- History, 40-42: (40) History by week. Rows show the time only, because GET /runs sends no set or
  climb count (server/api/run.go:411-419). (41) one exercise (`/history/exercises/{id}`), beside the
  list on desk. (42) readme: History.
- Library, sign-in, cleanup, 43-56: (43) the exercise library in the shell: one Row markup replaces
  the card/table pair. (44) css: drop the styles it no longer uses. (45) the exercise form on Field.
  (46) a read-only view for a shipped exercise. (47) session templates in the shell: Outline
  replaces TemplatePlan. (48) the template editor on Field, with its preview in the Panes aside.
  (49) larger row buttons in the editor. (50) confirm before removing a step. (51) css: drop the
  template styles no longer used. (52) sign-in and sign-up. (53) delete the old header and the
  `(old)` group. (54) css: drop the header, nav and theme-switch styles. (55) css: stop importing
  passion.css. (56) docs: DESIGN.md drops what described V1.
- passion.css has 436 class names. About 55 of them match a word in client markup (measured). The
  rest go with the screen that used to need them.

4. How mobile and PC differ, by example

- Plan at 390 px: one pane with the cycle header, the 6 × 7 grid, the legend, this week, "Add a
  session", and Undo above the tab bar. A day tap pushes `/plan/2026-09-29`: SessionCard, Outline,
  Start, Move. Back returns to the grid.
- Plan at 1280 px: side nav, then two panes. The first is the same Plan body at the same sizes. The
  second shows the day, today by default. A click changes only that pane and the URL: "more of the
  plan, not more controls" (principle 12).
- Log sets at 390 px: full screen, no tab bar. SetTable, two Steppers, Done or "Rest 1:59", Up next,
  and Hold to end at the bottom. The outline is `/run/{id}/steps`, pushed on top.
- Log sets at 1280 px: no nav either. The same player body at the same sizes, because size follows
  the reading distance (principle 2). The run Outline sits in a second pane, where swap, add and
  reorder happen. Every part is the phone's markup.

5. The cost of two views, measured

- Today: 9 pages, 2 layouts, 1 error page, 10 load files; 13 components and 6 modules in lib/.
  8 files hold viewport-width code (52 `sm:`/`md:`/`lg:` classes). 3 screens already draw a second
  markup per width: exercises/+page.svelte:104 and :124, templates/+page.svelte:92 and :121,
  TemplateEditor.svelte:279.
- A full two-view design needs a second version of every page: the 9 of today, plus the 9 routes
  this plan adds (`/`, `/plan`, `/plan/{date}`, `/plan/cycles/{id}`, `/run/{id}`, `/run/{id}/steps`,
  `/run/{id}/summary`, `/history`, `/history/exercises/{id}`). That is 18, plus a second nav.
- This plan needs no second version. Width-aware code sits in the 4 components of lib/shell/. The
  5 route layouts that name panes (plan, history, exercises, templates, run) hold no width logic.
- Places a change touches (this plan / one view / two views):
  - a new field on a form: 1 / 1 / 2 per form. `per_side` sits in two forms today
    (ExerciseForm.svelte:89, StepEditor.svelte:48), so 2 / 2 / 4.
  - a new screen: 2 (route, body) / 1-2 / 3, plus one nav entry for a top-level screen, where two
    views need two. Exercise history is one: its API is built (api.go:53) and it has no screen yet.
  - a bug fix in a list: 1 / 1 / 2. The exercise list prints labels in both markups
    (exercises/+page.svelte:115, :152), so a truncation fix already takes 2 edits.
  - a copy change: 1 / 1 / 2, unless the text sits in a shared part. "No exercises match your
    search." is at exercises/+page.svelte:169.
  - a token change: 2 lines in one file under all three. Today `--muted` is written 4 times
    (passion.css:16, 69, 121, 171), because each theme is written twice.
- Verdict: two views about doubles the screen markup, and the exercise list already shows the drift
  that follows. This plan pays a fixed 4 files, plus one layout for each screen with panes. Next to
  one view it loses on single-pane screens (Summary, sign-in), where Panes adds nothing.

6. How each commit is checked

- `pnpm --dir client check` (svelte-check, with Svelte's a11y warnings) and `pnpm --dir client build`.
- Screenshots of each changed screen at 390 and 1280 px, light and dark, with the scratchpad's
  shot.mjs. Shell commits add 1023 and 1024 px, and a resize across them with a draft typed in, to
  prove that the page stays mounted.
- Keyboard: every control is reachable by Tab and shows a focus ring. After a pane change, focus
  goes to the new pane's heading. SvelteKit would otherwise focus `<body>` (kit
  src/runtime/client/client.js:3232). Hold to end works from the keyboard, through a confirm.
- Screen reader: run a hangboard session and log a climb. "Rest over" is read once, the digits
  never (principle 10). VoiceOver on the owner's iPhone. I have not checked for Orca here.
- Contrast on the look's own surfaces: text 4.5:1, control edges 3:1 (principle 8). Hit areas 48 px
  or more (principle 4). At 200 % text the phase and the time do not clip (principle 10).
- The pixel agent on each template or CSS commit.

7. Risks, and what this plan leaves out

- The desk shell has no design. At 1280 px the mockup shows phones side by side
  (build2/d-*.png). The side nav and panes need a mockup before commit 30.
- Old and new CSS load together until commit 55. passion.css's global rules reach new screens:
  html and body at lines 215-235, `details > summary` at 581, 16 px inputs under 768 px at 2876.
  Shell sets its own ground and font, and screenshots catch the rest.
- Hidden panes stay mounted. A phone on one run keeps the history list in the DOM, and the list's
  load runs on a deep link too.
- SvelteKit restores only window scroll (kit src/runtime/client/utils.js:26). Desk panes scroll on
  their own, so Panes resets the detail pane's scroll, and Back does not restore it.
- The token set grows with each look's structure. After the pick, it keeps only what that look uses.
- Left out: every server change. The screens wait on set and climb counts in GET /runs (History
  rows), a soft delete to undo a run delete, an undo for a cycle rebuild, and a planned weight (V2
  targets). Also left out: the offline queue behind "Not synced yet", a service worker and manifest,
  the lock-screen timer, streaks and weekly bars, and the run-detail screen
  (`/history/runs/{id}` has no mockup, so no commit here).

8. Decisions for the owner

- The look: Logbook leads, and a blend is being planned. Changing it later means one token file
  and one import line.
- Archivo: a new font file in the client. I have not measured its size or checked its licence.
  Without it, `--font` is the system stack.
- A desk-shell mockup at 1280 px before commit 30.
- The breakpoint: 64rem, Tailwind's `lg` (theme.css:329). Two 390 px bodies leave 244 px of
  1024 for the nav. A tablet held upright usually
  falls under it and gets the phone shell; I have not measured one.
- The four tabs (Today, Plan, History, Library), which plan.md section 5 calls a proposal, and
  where sign-out and the theme setting live on a phone.
- The library at 1280 px: one Row markup replaces today's desk table (exercises/+page.svelte:124).
  A desk-only table would break the rule this plan rests on.
- Icons: the lucide shapes already in Icon.svelte (my pick), or the mockup's own drawn set.
- An empty detail pane on desk (`/history`, `/exercises`): a one-line hint (my pick), or the
  newest item.
