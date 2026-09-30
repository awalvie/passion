# Plan 2: two views, one for the phone and one for the PC

Checked against the installed svelte 5.57.1, @sveltejs/kit 2.70.3 and tailwindcss 4.3.3
(client/node_modules/.pnpm). NM below means client/node_modules.

1. Approach

- One route tree. Each route keeps one `+page.ts` for its data. Its `+page.svelte` is a thin switch,
  `{#if wide.current}<PlanPc {...props} />{:else}<PlanPhone {...props} />{/if}`, over screens in
  `lib/screens/phone/` and `lib/screens/pc/`. The `(app)` layout picks a shell the same way: a tab bar
  on the phone, a sidebar on the PC. The player sits in an `(app)/(full)` group with no shell.
- `wide` is one `new MediaQuery('min-width: 64rem')` from `svelte/reactivity`. Its constructor calls
  `window.matchMedia`, and `current` reads `matches` and follows `change` (NM/svelte/src/reactivity/
  media-query.js). The server build is a stub, so the import is safe in the build (index-server.js).
- Not one route group per view: the same path in two groups stops the build with "routes conflict
  with each other" (NM/@sveltejs/kit/src/core/sync/create_manifest_data/conflict.js).
- Tablet: the width decides, under 64rem phone, from 64rem PC. I did not measure tablet widths.
  Resize: crossing 64rem swaps the screen live. State that must survive lives in `+page.svelte` or
  the URL (`urlFilters` already does this). The run page reads `wide` once, so turning a tablet
  mid-session swaps nothing. First load: `ssr = false` (routes/+layout.ts), so nothing renders
  before JavaScript, as today, and the first render already knows the width. No wrong view flashes.
- A screen gets a PC version only when the PC shows other parts, more parts at once, or another way
  of working (mouse, keyboard, panes). A wider copy of the phone layout stays one screen.

2. Foundations

- Tokens: a new `client/src/tokens.css` next to passion.css holds the look's raw values and the names
  parts read: `--ground --surface --surface-2 --line --edge --ink --ink-2 --ink-3 --cta --cta-ink
  --ok --rest --danger --tpl`, radii `--r-card --r-in --r-btn --r-chip`, and look traits such as
  `--block-rule --row-now-bg --title-stretch`. No such name is in passion.css (grep), so both load
  side by side during the move. A look is these values: screens use only token utilities and parts,
  so a new look touches no screen. Where a look adds or hides a part (Logbook's week pill, mockup
  lines 91, 252), the part reads a token for it. The mockup has 75 Logbook-only rules; I did not
  sort them into values and structure.
- Light and dark in three blocks, as the mockup does (passion-looks.html lines 9-24): light on
  `:root`, dark under `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) }`, dark
  on `:root[data-theme="dark"]`. Today app.html always writes `data-theme`, so an open app never
  follows a system change. The fix: write it only for a stored choice (System, Light or Dark).
- Type, from the Logbook CSS in the mockup, in rem so text size reaches it (app.html has
  `<meta name="text-scale">`): label 0.75, body 0.9375, input 1 (iOS zooms under 16 px), title 1.25,
  page 1.875, phase 2.5. The timer (9.375 on a hang, 6.25 at rest) is the one size capped with
  `min()`, so it cannot clip at 200 % (principle 10). `--target: 3rem` (48 px, plan.md A2). One scale
  for both views (principle 11).
- Tailwind is not connected today: app.css imports passion.css into `layer(components)` and no file
  has `@theme` (grep). The bridge is `@theme inline` in app.css (`--color-ink: var(--ink)` and so on),
  giving `bg-surface`, `text-ink-2`, `border-line`, `rounded-card`, `text-label`. `inline` is needed
  because the values flip per theme; `--color-*: initial` then drops Tailwind's palette. Both are
  parsed in NM/tailwindcss/dist/lib.js. No .svelte file uses a palette class (grep), so none breaks.
  No `dark:` utilities: Tailwind's `dark` is only the media query (lib.js) and ignores the override.
- Parts, shared by both views, flat in `lib/` as today:
  - Kept: Icon (the mockup draws 18 icons, 2 exist, 16 to add), Notes, FormError, DurationInput,
    ExercisePicker, StepEditor, ChoiceEditor, SectionEditor, TemplatePlan, ExerciseForm, RowActions
    (bigger targets, a confirm on remove, plan.md §6).
  - New, and what each replaces: Button (`.btn`, `.btn-primary`, `.btn-ghost`), Field and Select with
    a session dot (`.input` and every form's label pattern), Stepper (`.stepper-btn`), ChipGroup (the
    journal and tick pills), SessionMark (the dot in templates/[id]), SessionRow (the template list's
    phone card), DayMark, SetRow and SetTable, ClimbRow, StatTriple, TimerFace, HoldButton,
    BlockHead, PageTitle, UndoToast, EmptyState, Chart (inline SVG).
  - One per view: PhoneShell and PcShell (replace Header.svelte); CycleGrid and CycleGridPc, both
    fed by a shared `lib/calendar.ts`.
- Data layer, shared: api.ts, session.ts, filters.svelte.ts, exercise.ts, template.ts, and new run.ts,
  cycle.ts, calendar.ts, timer.svelte.ts, theme.svelte.ts, view.svelte.ts. A view never calls the
  API, filters a list or builds a request body; it gets data and callbacks from `+page.svelte`.
- What new screens wait on: their API exists (readme), but the player, Summary, Today, Plan and
  Cycle wait on the owner agreeing the run and cycle tables (V2_DESIGN Parts 3, 4). History's
  "15 sets" has no field in the run list (server/api/run.go, runSummaryBody). "Not synced yet" waits
  on offline sync.

3. Order of work, one change per commit, 58 commits

- Docs: 1 describe the new look in DESIGN.md. 2 replace "The frontend follows V1" in V2_DESIGN.md.
- Foundations: 3 follow the system theme until one is picked. 4 add the look's tokens. 5 map them
  into Tailwind. 6 drop Tailwind's colours. 7 set `viewport-fit=cover`, so safe-area paddings work.
  8 add Archivo, only on the owner's yes.
- Player first, phone view: 9 run types and requests. 10 a timer worked out from the clock and
  mirrored to localStorage (V1's lesson). 11-16 Button, HoldButton, Stepper, SetRow, ChipGroup,
  TimerFace. 17 open a run full screen. 18 start a run from a template page. 19 play a timed step.
  20 log a sets step. 21 log a climb. 22 end with a hold. 23 summary and journal. 24 sound and a
  double buzz when rest ends. 25 keep the screen on. 26 tell a screen reader when the phase changes.
  Until 43, a PC shows the phone player in a centred column.
- Shells: 27 move the app pages into `(app)/(shell)`, no change in behaviour. 28 phone shell. 29 PC
  shell, from 64rem. 30 drop the old header and its styles. 31 DEVELOPMENT.md: a screen in two views.
  32 readme: `lib/screens` in the structure.
- Home screens, phone then PC: 33-34 Today and its empty state. 35-36 Plan. 37-38 Cycle.
  39-40 History. 41-42 exercise history. 43 the PC view of a run, a run sheet.
- Library, one screen at a time: 44-45 exercise list. 46 drop `.lib-row`. 47 exercise form, one
  version. 48 drop `.lib-edit-*`. 49-50 template list. 51 template page, one version. 52 drop
  `.preview-flow-*`. 53-54 template editor. 55 drop `.passion-disclosure` and `.template-color-*`.
  56 sign-in pages. 57 error page. 58 remove passion.css.
- passion.css is never edited, only cut. Its run, cycle, heatmap and guided-builder blocks are used by
  no .svelte file today (grep of every class attribute); they go with the file in 58.

4. How the phone and the PC differ

- Plan at 390 px (m-*-plan.png): title, the six weeks as 42 px cells with a coloured mark per session
  (mockup lines 362-371), a legend, "This week" rows, "Add a session", Undo, the tab bar.
- Plan at 1280 px, PC view. No mockup exists: the d-*.png shots are phone frames side by side. A
  sidebar with the four places; the cycle as a wide grid with each session's name in its cell; a day
  pane with Start, Move and Remove. Drag to move, or a "Move to…" button (principle 4). "This week"
  goes, because the grid names the week. Same SessionMark, DayMark and UndoToast. A move is one
  `PUT /api/v1/scheduled-sessions/{id}`, and Undo is a second.
- Log sets at 390 px (m-*-log.png): a set table on a `30px 1fr 60px 40px 48px` grid with 48 px rows
  (lines 177-179), two steppers, Done that turns into "Rest 1:59", Up next, Hold to end.
- Log sets at 1280 px, PC view: a run sheet for the home posture, such as a write-up of a past day
  (readme). Left, the sections and their status, where swap, add, reorder and go back happen
  (V2_DESIGN Part 3). Centre, every step of the section with its SetTable; Tab moves through weight
  and reps, Enter ticks. Right, the exercise's last sessions. A timed step shows TimerFace in the
  centre. Same SetRow part.

5. The cost of two views, measured

- Today (find): 9 page components, 3 layout and error components, 10 `+page.ts`/`+layout.ts`, and in
  `lib/` 13 components and 6 modules. A second version today: the exercise list, the template list,
  TemplateEditor, and Header as two shells: 4. Strictly doubled: all 7 app pages plus the shell.
- The 10 mockup screens are 7 routes (three are step kinds of one run; empty Today is a state).
  A second version: 6, all but Summary.
- End state: 16 page routes (9 today, 7 new), 9 screens in two versions, two shells. That is 10 more
  screen files than one view, and 9 PC layouts to design that the mockup does not draw.
- A new form field: 2 places either way, if fields sit in shared form parts; 3 against 2 if a view
  holds the field. Example: `needs`, in TemplateEditor.svelte line 177 and template.ts.
- A new screen: 4 files against 2 (`+page.ts`, the switch, two screens), and two layouts to design.
  Example: exercise history; its route exists (`GET /api/v1/exercises/{id}/history`).
- A list bug: markup 2 places against 1; logic 1 against 1 if the filter sits in `lib/`. Example:
  exercises/+page.svelte already draws each row twice. The phone card gives a screen reader the type
  as text (line 111); the desktop row has it only as a `title` on the cell (line 143). They drifted.
- A copy change: 2 places against 1 on a doubled screen. Example: "No exercises match your search."
  (exercises/+page.svelte line 169) is shared today only because it sits outside both layouts.
- A token change: 1 against 1. Example: the iOS fix, `.input` from 0.875rem to 1rem (passion.css line
  390). Today that is not 1 place: 24 inputs in 8 files add `text-sm` or `text-xs`, and utilities beat
  the components layer (NM/tailwindcss/index.css line 1). Each such override doubles with its screen.
- Drift it invites: a fix, a field or a label in one view only; an unsaved-changes guard in one view
  only (TemplateEditor's `beforeNavigate`); state lost at 64rem; a new screen missing one view.
- What stops it: both views take one Props type from `lib/`, so a missing or unknown prop fails
  svelte-check; views hold no logic; records look the same through shared parts; a doubled screen's
  commit screenshots both views; reviewers ask "did the other view get it?". Only the Props type is
  mechanical. The rest is discipline.
- Verdict: two views pay only where the PC works differently (Plan, the run sheet, History), which
  is why the rule in section 1 keeps the rest single. If the owner wants the PC to be mostly a wider
  phone, this plan costs more than it gives.

6. How each commit is checked

- `pnpm --dir client check` (types and the compiler's a11y warnings), then `pnpm --dir client build`.
- Screenshots with the scratchpad's shot.mjs (headless Firefox, theme set in localStorage) at 390
  and 1280 px, light and dark: the phone screen at 390, the PC screen at 1280. A shell or switch
  commit: also 1023 and 1024. Player commits: also rest and paused, and 390 at 200 % text.
- Keyboard: every control reachable by Tab, focus visible, Hold to end has a keyboard path, every
  drag has a button.
- Screen reader, on player commits: run a hangboard step and log a climb. The digits sit in
  `role="timer"`, silent by default; one polite, atomic region says "Rest over" once
  (research-principles-svelte.md §3).
- Contrast: 4.5:1 text, 3:1 control edges, on the look's own surfaces in both themes (contrast.py).
- Doubled screens: grep both views for each label and field. Agents per CLAUDE.md: pixel, copy,
  simplify.

7. Risks, and what the plan leaves out

- Drift (section 5) is the main risk, and it never goes away. The PC commits wait on PC designs
  that do not exist yet.
- A landscape tablet gets the PC view under a finger, so the PC view keeps 48 px targets and drops
  hover-only controls (DESIGN.md's `group-hover` pattern).
- The phone view serves every width up to 1023 px, so an upright tablet stretches it.
- Each route ships both views in its chunk. I did not measure the size.
- During the move, old screens sit in new shells with passion.css still loaded.
- The wake lock needs HTTPS in production (research-svelte-mobile.md §6).
- Left out: server work, offline sync, the lock-screen timer, a manifest and service worker,
  Capacitor, streaks and weekly bars, a template editor redesign (the mockup draws no editor).

8. Decisions for the owner

- The look, or the blended fourth. tokens.css waits for it; commits 9 and 10 do not.
- Archivo, a new font file for the client. Check its licence first.
- The switch at 64rem, by width only.
- A PC design round for the 9 doubled screens, before their PC commits.
- The four places (Today, Plan, History, Library), a proposal in plan.md.
- Which screens get a PC version: the rule in section 1, or all of them.
- The PC run page as a run sheet, not the player.
- Agreeing the run and cycle tables, which the player and Plan build on.
- Where the System, Light and Dark choice lives.
