Review of plan-2-two-views.md. Checked against client/src, client/node_modules (svelte 5.57.1,
kit 2.70.3, tailwindcss 4.3.3), server/api, the docs, the mockup, git blame, and the kit and
Tailwind docs. Most severe first.

- 1 must fix. The player cannot be full screen at commit 17. Header sits in the root layout, and
  the (shell)/(full) split only comes at 27.
  Example: routes/+layout.svelte:14 renders `<Header />` for every route. Kit docs: "The root
  layout applies to every page of your app". `+page@.svelte` still inherits it.
  Fix: make the group split its own commit before 17.

- 2 must fix. Commit 27, "no change in behaviour", is false. Header also serves sign-in, and the
  root error page renders inside the root layout.
  Example: Header.svelte:14 `authPage` shows the theme switch on /login and /signup (lines
  57-62). After the move, sign-in loses the logo and theme switch until 56, and every error page
  loses the nav until 57.
  Fix: in the split commit, give login, signup and the error page a layout that keeps Header
  until they move.

- 3 must fix. Commit 48 breaks the template editor for five commits.
  Example: "48 drop `.lib-edit-*`", but `lib-edit-sticky-bar` is used at TemplateEditor.svelte:287
  (the Save bar) until 53-54. Related: TemplatePlan.svelte uses `.preview-flow-*` and is also drawn
  by TemplateEditor.svelte:280, so 52 is safe only if 51 restyles TemplatePlan itself. The plan
  does not say it does.
  Fix: 48 drops only the classes exercises/[id] used alone, and sticky-bar goes in 55. Say that
  51 restyles TemplatePlan.

- 4 must fix. The phone player has none of the four run edits. Part 3 calls them the main job.
  Example: V2_DESIGN Part 3: "swap an exercise ... add one ... reorder ... go back ... V1 only lets
  you skip. Closing that gap is the main job." The plan puts them only in the PC sheet ("Left, the
  sections ... where swap, add, reorder and go back happen"). Commits 17-26 have none. They also
  have no picker step (one of the five step kinds) and no resume.
  Fix: phone commits for swap, add, reorder, go back, the picker step and resume, before any PC
  work. Swapping happens in the gym.

- 5 must fix. The run page is picked by width, but the need follows the kind of run.
  Example: REQUIREMENTS 2.8: "record almost nothing in the gym ... and fill the rest in that
  evening or that weekend". It names no device. V2_DESIGN lists "a session written up later from
  memory, backdated" as one of three kinds of run. Under the plan:
  - A phone write-up gets the timer player.
  - A laptop by a home hangboard gets a sheet with the timer in a centre pane. Principle 3:
    "during a hang, the navigation steps back and the timer leads".
  Nothing asks for Tab-through-weight and Enter-ticks.
  Fix: a live run gets the player at every width. A write-up or finished run gets one edit screen
  at every width, if the owner wants one. The width then never changes the run page.

- 6 must fix. "Reads `wide` once" breaks in these cases:
  - 400 % zoom in a 1280 px window leaves 320 CSS px. The three-pane sheet stays, which fails
    WCAG 1.4.10 reflow and principle 10.
  - A window snapped to half of a 1920 px screen is 960 px. The sheet stays, and the plan has no
    sheet layout under 64rem.
  - A landscape tablet over 64rem (the plan's own risk, section 7) gets the keyboard sheet under
    chalky fingers. Rotating does not swap it. Going to Summary and back remounts the page, so it
    does swap then.
  - A run started on a phone and opened on a PC: the timer mirror is in the phone's localStorage
    (commit 10). V2_DESIGN also says "One device records a run, so last write wins today", so the
    phone's stale `PUT /runs/{id}` overwrites the PC's edits.
  Fix: the fix in 5 removes the width dependence. For the PC resume, keep the one-device rule or
  leave the sheet out.

- 7 must fix. A live swap at 64rem silently loses template edits.
  Example: TemplateEditor.svelte keeps `draft` (line 33) and takes the `clean` snapshot at mount
  (line 36). The guard compares against that snapshot (38-43). With 53-54 doubling the editor, a
  window snap or a zoom remounts the view. If the draft stays in the view, the edits are gone. If
  it is lifted, the new view's snapshot is the dirty draft, and the guard never fires again.
  Principle 6.
  Fix: keep the editor as one version. Its only PC difference is the aside at line 279, and CSS
  already handles it. If it is doubled anyway, first lift draft, snapshot and guard into a shared
  module, as their own commit.

- 8 must fix. Ten commits build PC screens that have no design. That is speculative.
  Example: section 7: "The PC commits wait on PC designs that do not exist yet". The d-*.png shots
  are phone frames side by side (checked build2/d-logbook-light-plan.png). Owner: "If adding it
  later is cheap, cut it now." Adding a PC view later is cheap: move the screen into
  lib/screens/phone, add the PC file and a three-line switch.
  Fix: cut 29, 34, 36, 38, 40, 42, 43, 45, 50, 54, and 31 (the two-view guide). Cut view.svelte.ts
  and lib/screens/pc. The smallest plan that keeps the option open:
  - one screen per route
  - shared parts
  - API and filter logic in lib modules, as today
  - wide widths by CSS, as today (exercises/+page.svelte:124 `hidden md:block`)
  - a PC view for a screen only once the owner approves its design

- 9 must fix. The plan contradicts itself on which screens get two versions.
  Example: the verdict says "two views pay only where the PC works differently (Plan, the run
  sheet, History), which is why the rule in section 1 keeps the rest single". Section 3 doubles 9
  screens anyway:
  - Cycle, a plain form (name, dates, block, day selects), while the exercise form stays "one
    version" (47).
  - Today and exercise history, with no PC reason given.
  - The template editor (53-54), while section 7 leaves out "a template editor redesign".
  Fix: one list, and each doubled screen gets a stated reason and a design.

- 10 should fix. The rule "more parts at once → PC version" doubles screens that one file handles
  today.
  Example: exercises/+page.svelte:104/124 and templates/+page.svelte:92/121 switch with
  `md:hidden` / `hidden md:block`. TemplateEditor.svelte:279 hides one aside. A CSS switch keeps
  focus, scroll and form state on resize. `{#if wide.current}` throws them away.
  Fix: a second view only for "another way of working", and only with a design.

- 11 should fix. The cost counts understate the plan's own cost.
  - "10 more screen files than one view" breaks its own rule, "A new screen: 4 files against 2".
    By that rule, 9 doubled screens are 18 more files, plus PcShell and CycleGridPc: 20, not 10.
    The 10 leaves out the 9 switch files.
  - exercises/+page.svelte lines 31-99 and 169 are shared by both layouts today: title, filter bar,
    empty text. The plan has no filter-bar part, so every one of those strings goes to 2 places.
  - It counts files, not lines. TemplateEditor has 297 lines (wc), the largest client file, and
    counts as "1".
  - Not counted: two views × two themes of shots and reviews per doubled screen, and PC-only
    behaviour (drag, Tab/Enter) with its own keyboard and a11y checks.
  Fix: count by its own 4-against-2 rule, and give the line counts of the doubled screens.

- 12 should fix. The drift controls are weaker than the plan says.
  - "a missing or unknown prop fails svelte-check": the switch spreads one Props object into both
    views, so every prop always arrives. A view that never reads a field passes. tsconfig.json
    has no `noUnusedLocals`, and a prop left out of the destructuring is never flagged anyway.
  - Screenshots miss the plan's own example, a missing sr-only label.
  - That example is not drift. git blame shows both row layouts (lines 104-145) came in one
    commit, 860bfe0, so review already missed a gap inside one file.
  - package.json has no JS test runner, so a parity test needs the owner's yes.
  - "Views hold no logic" fails for its own PC designs: drag-to-move and Tab/Enter ticking are
    logic that lives only in PC views.
  Fix: say plainly that drift control is review only. Keep the doubled surface small. Put heads,
  filters, empty states and field lists in shared parts, so a view holds only layout.

- 13 should fix. Commit 3 is blocked, and it has to change Header.
  - Header.svelte:10 reads `data-theme` at mount. Once app.html stops writing it when no choice is
    stored, a system-dark user sees the switch set to light.
  - The switch can store only light or dark. There is no way back to System.
  - The fix for both waits on the owner decision "Where the System, Light and Dark choice lives".
  - shot.mjs:36 always writes `passion-theme`, so no screenshot ever covers the no-choice path.
  Fix: do 3 in the shell commit that holds the choice, after that decision. Add a shot with no
  stored theme.

- 14 should fix. Some commits hold two or more changes:
  - 23 "summary and journal"
  - 24 "sound and a double buzz"
  - 33 "Today and its empty state", because 34 is Today's PC version
  - 43: sections pane, swap, add, reorder, go back, keyboard entry and a history pane
  - 36: grid, day pane, drag, "Move to…" and Remove
  Fix: split them.

- 15 should fix. The PC Plan works against principles 11 and 12.
  - Principle 12: "A larger screen shows more of the plan, not more controls". The PC adds drag
    and a day pane with Start, Move and Remove.
  - Principle 11: "a session ... look[s] the same in the player, in the plan and in history".
    The PC cell shows the session's name and the phone cell shows a mark. CycleGridPc is a part
    only one view uses.
  - Remove has no one-step undo. DELETE then POST makes "A one-off, with no cycle behind it"
    (server/api/scheduled_session.go:121).
  Fix: if a PC Plan gets designed, use the same cell part, larger. Offer no Remove without an
  undo that restores the row.

- 16 should fix. Width is the wrong signal for "another way of working (mouse, keyboard, panes)".
  - A tablet under a finger gets the PC view.
  - Half of a 1920 px screen (960 px) gives a mouse user the phone view and a bottom tab bar.
  - `64rem` in a media query uses the browser's default font size, not the root's. A raised
    default text size moves the switch, so the switch is not "by width only".
  Fix: say this in the owner decision about the 64rem switch.

- 17 should fix. Commit 7 alone can harm the old screens.
  Example: `viewport-fit=cover` with `.passion-container` padding only 1rem (passion.css:245) and
  no safe-area-inset-left/right. A notched phone in landscape can put content under the notch.
  Fix: fold 7 into the first commit that pads a safe area.

- 18 should fix. The readme and DEVELOPMENT.md updates are the wrong ones.
  - Commit 32 adds lib/screens to a structure that lists only top-level folders. CLAUDE.md asks
    for "New top-level directory".
  - readme.md:21 ("The client has sign-in, the exercise library and the session template screens
    so far") goes stale at the first player commit.
  - DEVELOPMENT.md:73 ("Put a page that needs sign-in under client/src/routes/(app)/") goes stale
    at the group split.
  Fix: cut 32. Update line 21 as each screen lands, and line 73 in the split commit.

- 19 should fix. The owner decisions leave some out and put one in the wrong order.
  - Missing, and it must come first: whether the PC gets its own screens at all. The plan's own
    words: "If the owner wants the PC to be mostly a wider phone, this plan costs more than it
    gives."
  - Missing: exercise history "waits for the owner's review" (V2_DESIGN Part 5, step 1).
  - Missing: whether a write-up gets its own screen (finding 5).
  - Missing: what to build while the run tables wait. The player is first but blocked. The library
    screens wait on nothing, because their tables are agreed.
  - Missing: where the cue-off or mute switch lives (principle 5: "Can the person turn each cue
    off?").
  - Partly the plan's own homework: "The switch at 64rem". Measure the widths first ("I did not
    measure tablet widths"), then ask.

- 20 cut. Look-trait tokens that add or hide parts serve looks that will not ship.
  Example: "a new look touches no screen. Where a look adds or hides a part (Logbook's week pill),
  the part reads a token for it." Principle 11: "There is one look on every platform."
  Fix: tokens only for values that change with the theme, plus the session colour. The look's
  structure goes in the parts.

- 21 cut. Commit 6 changes no output.
  Example: the Tailwind docs say "By default only used CSS variables will be generated". grep
  finds no palette class in any .svelte file.
  Fix: merge it into 5 as "replace Tailwind's palette with the look's colours", or cut it.

- 22 cut. Commits 11-16 add six parts that nothing uses yet.
  Example: Stepper and SetRow arrive at 13-14 and have no user until 20.
  Fix: add each part in the commit of its first screen. That is 6 fewer commits.

- 23 should fix. Small facts that are wrong:
  - Mockup line 177 is `30px minmax(0,1fr) 60px 40px 48px`, not `30px 1fr ...`. With `1fr`, a long
    exercise name can widen the grid.
  - "`inline` is needed because the values flip per theme": the flip happens on `:root`, where
    plain `@theme` also resolves. `inline` is needed for values set on one element, such as
    `style="--tpl:#EF4444"` (mockup line 723). Tailwind docs: a variable is "resolved where
    --font-sans is defined".
  - "The `(app)` layout picks a shell" would wrap `(app)/(full)` too. The shell has to sit in
    `(app)/(shell)`, as commit 27 says.
  - The mockup's light and dark blocks are lines 9-26, not 9-24.
  - "2 exist" (icons) is true by name only. The mockup draws timer and layers differently from
    Icon.svelte (lucide 0.525.0), and the plan does not say where the 16 new icons come from.
  - "They drifted": both layouts were written in one commit (finding 12).

- Checked and correct:
  - Counts: 9 pages, 3 layout and error files, 10 load files, 13 components and 6 modules in
    lib, 24 inputs in 8 files, 75 Logbook-only rules, 18 mockup icons, 10 mockup screens as 7
    routes, 58 commits.
  - File and line references: passion.css:390, exercises/+page.svelte:111/143/169,
    TemplateEditor.svelte:177, tailwindcss/index.css:1, mockup 91/252/362-371, runSummaryBody (no
    set count).
  - Behaviour: kit conflict.js strips groups and throws. MediaQuery reads `matches` and listens
    to `change`, and the server build is a stub. `ssr = false` means no wrong view flashes. The
    `dark:` variant is the media query only. The run, cycle and heatmap blocks in passion.css are
    unused.

- Commit count after the cuts and splits above: 58, minus 11 PC and guide commits, minus 32, 6, 7
  and 11-16 (9), plus 3 splits = 41. The missing phone run edits, picker and resume (finding 4)
  come on top.

Verdict: the file and line facts hold, but the plan builds ten PC screens that have no design. It
breaks the app at 17, 27 and 48. It leaves the phone player without the run edits that Part 3
calls the main job. It picks the run page by width when the need follows the kind of run. Cut it
to one view per route, and add a PC view for a screen once that screen's design is approved.
