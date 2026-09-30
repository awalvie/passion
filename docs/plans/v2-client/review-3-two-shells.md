Review of plan 3, "shared parts inside two shells". Most severe first.

1. Must fix. The desk has no design, yet desk machinery shapes the first 28 commits.
   - Problem: Shell's desk half, Panes, the MediaQuery switch and hidden panes are built for a
     layout nobody has drawn. Panes lands in the player at commit 21, before the plan's own gate:
     "The side nav and panes need a mockup before commit 30." Nothing in the app can trigger a
     second pane today, and adding one later is cheap.
   - Example: "(21) the run outline, where Panes lands: an aside on desk". The mockup at 1280 px
     shows phones side by side (build2/d-logbook-light-plan.png), and no desk pane.
   - Fix: the smallest step that keeps the option open. Put each list in its route's
     `+layout.svelte` and render it only at the index route, with the detail as a child route.
     That fixes the URLs and the code's place now. Bodies use no `sm:`/`md:`/`lg:`. A phone
     shell only, and on a wide screen the one pane sits centred. SideNav, Panes, MediaQuery,
     hidden panes and the 1023/1024 px resize shots wait for an agreed desk mockup. Svelte's own
     doc comment on MediaQuery says "If you can use the media query in CSS to achieve the same
     effect, do that" (svelte/src/reactivity/media-query.js:23).

2. Must fix. The pane URL rule breaks in three places.
   - Filters: `urlFilters` resets the filters from `location.search` after every navigation
     (lib/filters.svelte.ts:20-23). On desk the list stays mounted, so a row click to
     `/exercises/5` (no query) clears the filters under the cursor. If the person then types in
     the search, line 15 calls `replaceState('/exercises?q=…')`. The address bar now says
     `/exercises` while pane 2 still shows exercise 5, and a reload loses the detail.
   - Resize: on a landscape tablet, tap exercise A, then B, then C. Rotate to portrait and the
     phone shell shows C. Back shows B, not the list, which breaks "Back goes to the list on a
     phone".
   - One URL, two meanings: `/plan` shows "the day, today by default" on desk and no day on a
     phone. `/run/{id}/steps` has no meaning on desk, where the outline is already an aside.
   - Fix: when a detail is already open, a row link replaces the history entry instead of
     pushing one. The rule needs no width check, because a phone hides the list whenever a
     detail is open. `urlFilters` writes the current path, and row links carry the query.
     `/plan` shows the same one-line hint as `/history`. The run outline is its own route at
     every width (see 6).

3. Must fix. A live region inside a hidden pane is silent.
   - Problem: `hidden` takes the pane out of the accessibility tree. On a phone,
     `/run/{id}/steps` hides the player body, so "Rest over" is not read while the outline is
     open. This fails principle 10.
   - Example: "(16) announce a phase change once, in a polite live region", and "Panes hides a
     pane with `hidden` instead of unmounting it".
   - Fix: put the live region in the run layout, outside every pane. Say so in commit 16.

4. Must fix. The order contradicts itself, and three commits leave dead links.
   - Section 1 says "The player's `(app)/run/` layout uses Shell with no nav", but Shell first
     lands at "(29) the phone shell". Panes at 21 needs the phone/desk switch that Shell owns.
   - Commit 10 adds Start on a template's page, but the route it opens first exists at 11.
   - Commit 29 brings a tab bar with Plan and History. Those screens land at 34 and 40, and
     Library still opens the old header.
   - Fix: the player needs no Shell, so drop it from the run layout. Put 10 after 11. Each tab
     joins `nav.ts` in the commit that builds its screen.

5. Must fix. Several commits hold two changes.
   - 11 "focus layout, elapsed pill, Skip, HoldButton": Hold to end finishes a run, which is its
     own change.
   - 13 "a sound and a double buzz". 17 "last time, Stepper, Done turns into the rest
     countdown": the prefill from history, the set log and rest are three changes.
   - 21 "the run outline, where Panes lands". 26 "the summary, with one best". 29 "the phone
     shell, holding Today". 48 "on Field, with its preview in the Panes aside".
   - By the literal "and" rule, also 32, 34 and 52.
   - Fix: split each one. The count goes up, but each commit stays one change.

6. Should fix. The desk player works against principle 3 and against the plan's own claim that
   state survives a resize.
   - Problem: at 1024 px or more, a tablet on the floor gets the timer plus an outline pane.
     Principle 3 says "during a hang, the navigation steps back and the timer leads". The
     outline also mounts in two places: the layout's aside on desk and the child route on a
     phone. At `/run/{id}/steps` on desk, both render, which duplicates ids and the swap, add
     and reorder controls. On a resize the outline moves between the two mounts and loses an
     open reorder.
   - Example: "Log sets at 1280 px: … The run Outline sits in a second pane, where swap, add and
     reorder happen."
   - Fix: the player is one pane at every width. The outline is `/run/{id}/steps` everywhere,
     and closing it calls `history.back()`.

7. Should fix. The two-view counts lean toward this plan.
   - The count of 18 includes the player, which the plan itself calls "the same player body at
     the same sizes" at 1280 px. It also includes sign-in and the wrappers around shared forms,
     and `/run/{id}/steps`, a route that only this plan's pane rule needs.
   - "a new field on a form: 1 / 1 / 2 per form": the forms are already shared components
     (ExerciseForm.svelte:89, StepEditor.svelte:48), so two views edit them once too.
   - "two views about doubles the screen markup" was not measured.
   - This plan's own column leaves out the Panes layout, the index hint page, the focus, scroll
     and back-arrow code, and the resize shots. By its own table it never beats one view. It
     wins only on side-by-side desk panes, which have no design.
   - Fix: count each stance with shared parts, and state plainly what the shells buy.

8. Should fix. "Width-aware code sits in the 4 components of lib/shell/" is not true.
   - The back arrow is phone-only (exercises/[id]/+page.svelte:72-78). Beside the list on desk
     it has no job, so something in the body must hide it.
   - Other width-only content outside lib/shell: `/plan`'s default day (desk only), the template
     preview "in the Panes aside" (desk only, as TemplateEditor.svelte:279 is today), and the
     run outline aside.
   - "A desk-only table would break the rule this plan rests on" is a false choice. A wide pane
     can give Row its Labels and Source columns through a container query, the plan's own tool,
     with no viewport code.
   - Fix: list every width-dependent piece. Move the arrow into Panes, and offer the library
     table as a container-query Row.

9. Should fix. The new base styles lose to passion.css until commit 55.
   - Tailwind declares `@layer theme, base, components, utilities` (tailwindcss/index.css:1).
     passion.css is imported into `components` (app.css:7). So a new `base.css` in
     `layer(base)` loses to passion.css:215-224 on `html` background and colour and on `body`
     font.
   - This shows in the iOS overscroll area, and after commit 7 in the notch strip. Neither
     shows up in a screenshot.
   - Fix: import base.css and the look file into a layer declared after `components`, or
     outside any layer.

10. Should fix. `hidden` loses to any `display` rule.
    - The repo has already been bitten by this: passion.css:3-10 adds `display: none
      !important` so that `[hidden]` beats display utilities on five wrappers.
    - Fix: Panes sets no `display` on a pane, or adds the same `!important` rule.

11. Should fix. On a phone, a detail waits for a list it does not show.
    - SvelteKit awaits every layout and page load before it renders (kit
      src/runtime/client/client.js:1324). A phone deep link to `/history/exercises/5` waits for
      GET /runs. With no signal, a failed list load turns the whole detail into the error page.
    - Fix: state the cost and accept it, or load the list in the index page.

12. Should fix. Focus after a pane change.
    - On desk, "focus goes to the new pane's heading" on every row click. A keyboard user
      loses their place in the list.
    - On a phone, Back puts focus on the list heading instead of the row that was tapped.
    - SvelteKit focuses the first `[autofocus]` in the whole document (client.js:3233). That
      element can sit in a hidden pane, where it cannot take focus.
    - Fix: on desk, keep focus on the row. On a phone, Back returns focus to the row. No
      `autofocus` inside a pane.

13. Should fix. The token names can collide in Tailwind.
    - The colour list has `rest`, and the type scale has a "rest size". I compiled
      `@theme inline { --color-rest: …; --text-rest: … }` with tailwindcss 4.3.3.
      `text-rest` came out as `color: var(--rest)` only, and the size was dropped without a
      warning.
    - Fix: give the type sizes names that no colour uses.

14. Should fix. Parts are styled two ways.
    - Parts use scoped `<style>` blocks, and screens use `theme.css` utilities. The built
      scoped CSS sits outside every layer
      (server/web/dist/_app/immutable/assets/ExerciseForm.DLZDSSC1.css), so it beats every
      Tailwind utility. A caller's `class="w-full"` on Button loses to Button's own width.
    - Fix: pick one way per part. Say that parts take no layout classes from their callers.

15. Should fix. Facts that are wrong or overstated.
    - "8 files hold viewport-width code": passion.css adds 14 width `@media` blocks
      (lines 603 to 4368), so it is 9 files. The 52 `sm:`/`md:`/`lg:` classes in 8 Svelte files
      are right.
    - "3 screens already draw a second markup per width": TemplateEditor.svelte:279 is a
      desk-only preview in a component, not a second markup of a screen. The true count is two
      screens.
    - "passion.css's 4 `env()` insets start to work": only `.site-header` (passion.css:596) is
      on a class the client renders. `.run-shell` (1119), `.run-playlist-panel` (1156) and
      `.run-sticky-controls` (2145) are V1 player classes that no markup uses.
    - "The rest go with the screen that used to need them": about 380 of the 436 classes have no
      client screen at all. With css_usage.py, only 891 of 3,710 rule-block lines sit in rules
      that the client uses.
    - "Changing it later means one token file and one import line": this is false for Topo.
      Topo changes structure, not only tokens: panels instead of lines, and a different radius
      and shadow (plan-look4.md, "Do not share").
    - Archivo "not measured": it can be measured. Its licence is OFL (google/fonts
      ofl/archivo/METADATA.pb). The latin woff2 with both the width and weight axes is
      90,104 bytes (fonts.gstatic.com, v25).
    - Checked and right: app.html:7-15, ExercisePicker.svelte:48, exercises/+page.svelte:104,
      :115, :124, :152, :169, templates/+page.svelte:92, :121, ExerciseForm.svelte:89,
      StepEditor.svelte:48, passion.css:16/69/121/171, :215-235, :581, :2876, api.go:53 and
      59-68, run.go:411-419, kit routing.js:118-124, client.js:3232, utils.js:26, tailwind
      theme.css:329. Also right: 9 pages, 2 layouts, 1 error page, 10 load files, 13
      components, 6 modules (lib/index.ts is an empty placeholder), 16 inline styles in 9
      files, 23 button lines, 7 `.input` components, 436 classes, 77 Logbook lines.

16. Should fix. Commit 7 on its own makes the old screens worse.
    - passion.css has no `safe-area-inset-left` or `-right`. With `viewport-fit=cover`, the
      old screens run under the notch in landscape. The one inset that works is on the header,
      which is being moved away.
    - Fix: fold 7 into 11, the first full-screen page.

17. Should fix. Commit 3 has no way back to following the system.
    - The old switch always pins a theme (Header.svelte:23-27), and it reads the theme only
      once (:10). After one toggle, nothing unpins it, and a change in the system setting
      leaves the switch showing the wrong state.
    - Fix: add a "System" choice to 3, or say that 3 only adds the listener.

18. Should fix. The review agents will fight the new parts.
    - pixel.md:39-41 and CLAUDE.md's Dropdowns rule demand `class="input text-sm"` on every
      `<select>`. pixel.md:16 also reads `static/passion.css`, which is the wrong path.
    - "The pixel agent on each template or CSS commit" will therefore flag every Field.
    - Fix: ask the owner to update pixel.md and that CLAUDE.md rule before commit 45.

19. Should fix. The double buzz cannot work on an iPhone.
    - MDN's compatibility data (api/Navigator.json) gives `vibrate` for Safari as
      `version_added: false`, and Safari on iOS mirrors it. The plan tests on "the owner's
      iPhone".
    - Fix: commit 13 is sound only on iOS. Ask the owner whether that is enough, or whether a
      native wrapper comes later.

20. Should fix. The mockup rule is applied unevenly.
    - `/history/runs/{id}` is left out because it "has no mockup". The plan still builds other
      screens that have no mockup: the day, the outline, swap, add, reorder, choice, the one-off
      session, the library, templates and sign-in. Panes lands on two of them (the day and the
      outline).
    - Fix: apply one rule to all of them, and let the owner choose which rule.

21. Should fix. The decision list is missing some decisions and has one that is not the
    owner's.
    - Missing: desk panes now or later (the core one). The player on a landscape tablet. History
      rows without counts, although the mockup shows "23 sets", or a server field first.
      Editing a cycle with no undo, against principle 12, or server undo first. Sound only on
      iPhone.
    - Plan's default day is a UX choice the plan made without listing it, and it contradicts the
      hint the plan picked for `/history`.
    - Not the owner's: the breakpoint. It follows from the desk mockup.

22. Should fix. Commits 43 to 46 split the library across two layouts.
    - The list moves into the shell at 43, while the detail and the new-exercise form stay in
      the old layout until 45 and 46. Panes cannot show an old page as its second pane, so on
      desk a row click leaves the shell.
    - Fix: say that the library's panes arrive at 46.

23. Should fix. Where Summary sits is not stated.
    - If `/run/{id}/summary` sits under the run layout, the finished player stays mounted behind
      it. If Hold to end pushes the summary, Back returns to a finished player.
    - Fix: put Summary outside the player layout, and replace the history entry on finish.

24. Cut. Structure tokens for looks that will not ship.
    - "a token holds the keyword (`--sections-dir: column`)" and "Another look is another file".
    - The owner decided on one look everywhere. Build the picked look's tokens after the pick,
      and nothing for the others.

25. Cut. The `(old)` group moves 19 route files twice.
    - They move once into `(old)` and once more into `(shell)`.
    - Header already branches on the path (Header.svelte:14). A branch on the run route id in
      the root layout keeps the player bare in one line, and commit 53 deletes it.

26. Cut. Docs and tests written ahead of the code.
    - Commit 1 documents "shells and the pane rule" before they exist. The repo writes docs as
      built ("docs: describe exercise history as built"), so each part's docs land with it.
    - The 1023/1024 px resize shots with a draft typed in: take them once, when Panes lands.

Verdict: the facts are mostly right, but the plan builds a desk that has not been designed,
and the desk bends the phone's routes and the player. Cut the desk half until there is a desk
mockup, fix the URL, filter and live-region breaks, and split the commits that hold two
changes.
