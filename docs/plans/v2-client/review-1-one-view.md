Review of plan 1: one adaptive view

Ranked, most severe first. Each finding has the problem, one example I checked, and the fix.

1. Must fix. The two-view cost is tilted toward one view.
   - Problem: it prices two views as copies of whole files, with every screen built twice. A two-view design shares its parts and forks only the layout.
   - Example: "7 files ... 1,075 lines". The sum is right, but the file choice is not. The root layout's only width rule is `md:mt-6` (+layout.svelte:16). ExerciseForm has one `sm:grid-cols-2` (line 16). exercises/[id] has one `sm:inline` (line 99). SectionEditor was left out for the same kind of rule.
   - Example: in TemplateEditor only lines 227–282 (56 of 297) change with width. The `needs` field (177–185) has no width rule, so "Two views: 3" files for a new field assumes the settings block is forked.
   - Example: "A new screen ... Two views ... 5" assumes every screen gets two views. By the plan's own count, 3 of the 10 widen.
   - Example: commits 62–80 rewrite the old screens anyway, so their lines are not a future cost.
   - Example: "have drifted" rests on one real case, the `sr-only` kind (exercises:111) against a bare `title` (exercises:143). The chip classes (templates 106–111 against 158–163) differ because one chip sits in a truncating line and the other in a table cell. "3 sections" against a number under a "Sections" header is on purpose.
   - Fix: measure today's forks as the 139 list lines, Header (121) and TemplateEditor 227–282. Count one layout for each screen that widens, not a whole screen. State one view's costs in the same units: container rules in each part, the MediaQuery escapes, and list-and-detail.

2. Must fix. The app cannot reach the player for 28 commits.
   - Example: "open a run in the player" is commit 15. "start a session from a template page" is commit 43, the last one in the player block. Only `POST /api/v1/runs` creates a run, and the client does not call it before 43. "resume a run" (about commit 37) has no way in until Today (48).
   - Fix: move "start a session from a template page" to right after 15. Give resume its way in (the template page or Today) in the same commit.

3. Must fix. The plan contradicts itself on which screens widen, and the player's PC column has no commit.
   - Example: section 4 says "Past 48rem the player adds a second column with SessionOutline". Section 5 says "The 3 player screens and Empty Today never widen". Section 8 says "Only Plan, History, Exercise and the Library widen". Section 4 also keeps TemplateEditor's side pane.
   - Example: part SessionOutline is commit 46, after the whole player block (11–43). No commit adds the column, or the view "behind Up next" on a phone.
   - Fix: decide. If the player keeps the column, move SessionOutline before the column and add a commit for it. Add the player and TemplateEditor to the list of screens that widen, and redo the section 5 counts.

4. Must fix. Four commits hold more than one change.
   - "sound and buzz when a phase ends": two browser APIs, and principle 5 asks "Can the person turn each cue off?"
   - "keep the run clock right after the phone sleeps (time from the clock, mirrored to localStorage)": the clock handles sleep, and the mirror handles a closed tab. Split them, or cut the mirror until a reload is shown to lose time.
   - "add a settings screen (theme, grade scales, sign out)": three changes. Grade scales is new (`PUT /api/v1/accounts/me/grades`). The other two move out of Header.
   - "log a sets step filled in from last time": logging (`PUT .../sets`) and the prefill (`GET /exercises/{id}/history`) are separate.
   - Fix: split each one. That adds about 5 commits.

5. Must fix. The Nav and Settings commits leave dead ends.
   - Example: commit 45 is "part Nav, in place of Header's links", a nav with "four links". Today lands at 48, Plan at 53 and History at 59.
   - Example: Settings (44) has no way in. The Nav has no settings link, and "Where settings live" is still open.
   - Example: from 44 to 79, Header's switch (Header.svelte:40–45) and the Settings chips both set the theme, and Header's switch cannot pick System.
   - Fix: start the Nav with Library only, and add each link in its screen's commit (section 5 already counts it that way). Get the settings decision before 44. Remove Header's theme switch and Log out in the commits that move them to Settings.

6. Should fix. "The phone column, centred" is a step back on a PC and misses principle 12.
   - Example: DESIGN.md already has a "Two-column content layout (most detail pages)" (docs/DESIGN.md:174–180). Today TemplateEditor (lines 227 and 279) and exercises/[id] (`lib-edit-grid`, passion.css:3627–3649) show two columns on a PC.
   - Example: REQUIREMENTS §4 says today's app "is not built for a phone screen first", so the PC is where people use it now.
   - Example: principle 12 asks "Can you see the shape of the whole cycle, with today marked?" The default leaves Cycle as a phone column.
   - Fix: propose DESIGN.md's two columns as the PC default for home screens (Today, Plan, Cycle, History, Exercise, Library and the editors), and the phone column only for gym screens. Each layout still goes to the owner, but with a better default.

7. Should fix. The two losses the plan gives up are not forced.
   - Example: "A list beside its detail does not fit CSS well, so it is left out." A SvelteKit nested layout does it with no JS switch. history/+layout.svelte renders the list and `{@render children()}` in one CSS grid. On a phone the list hides while a child route is open, which takes one route check and no media query. This is "more of the plan, not more controls", which principle 12 asks for.
   - Example: "The library lists ... lose `<table>` semantics." One `<table>` can hide its Labels and Source columns below a container width and keep its semantics.
   - Fix: keep `<table>`, and take that item out of the owner decisions. Cost list-and-detail for History as an option instead of leaving it out.

8. Should fix. The theme fix adds JS that the CSS does not need, and it breaks Header in the meantime.
   - The claim is right. app.html:10–13 always sets `data-theme`, so after load passion.css's system branch (`@media (prefers-color-scheme: dark) { :root {...} }`, lines 62–113) never applies. That one script is the whole cause.
   - Problem: the plan keeps setting `data-theme` and adds a matchMedia listener. Header reads the theme only once (Header.svelte:10). After an OS change its switch shows the wrong state, and the first click seems to do nothing, from commit 1 to commit 44.
   - Problem: shot.mjs always writes `passion-theme`, so the check "the page follows an OS change without a reload" has no way to run.
   - Fix: in app.html, set `data-theme` only when a theme is pinned. Write tokens.css in three blocks (light, `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) }` and `[data-theme="dark"]`), as the mockup already does (passion-looks.html:9–26). Then no listener is needed. Say how commit 1 gets tested.

9. Should fix. Parts mix scoped styles and Tailwind utilities, and the cascade will surprise.
   - Example: "Each part keeps its own rules in a scoped `<style>` block", and parts also use `@md:` and `text-ink` from `@theme`. Utilities sit in `@layer utilities`, and Svelte's component CSS sits in no layer. A rule outside any layer beats a layered rule whatever its specificity. So a scoped `padding` silently beats `p-4` on the same element.
   - Fix: use one way per part. If parts use scoped styles that read `var(--token)`, the `@theme` mapping in commit 3 has no user yet and can wait.

10. Cut. Tokens that switch between looks.
   - Example: "With these, Logbook is 'no card, a top rule' and Chalk is 'a card'". Also nav-inset and nav-radius ("Chalk's floating bar, Logbook's flush bar"), title-case and title-stretch. plan.md says the owner picks one look "before any client code changes", and nothing in the app switches looks.
   - Example: "The blended fourth look is a new tokens.css" is not true of Topo. plan-look4.md lists "Seven Topo-only rules", among them a phase box whose order against the Paused rule matters.
   - Fix: make tokens only for the picked look's values: colour, type, radius and motion. Put structure in the parts. Drop "tokens.css is the only file it changes".

11. Cut. The MediaQuery switch in TemplateEditor.
   - Example: "`hidden md:block` still renders it on every keystroke". Nothing was measured. Svelte 5 updates only the nodes whose state changed, and TemplatePlan is 58 lines.
   - Fix: keep the CSS hide until a measurement shows a cost.

12. Cut. The purge at the start and the last redundant drops.
   - Example: commit 8, "drop the rules no screen uses", deletes 2,819 of the 3,710 lines in rule blocks (css_usage.py) before any screen moves. The rules it deletes match no element, so the page does not change. plan.md already chose to "delete the old styles a screen no longer uses".
   - Example: "drop btn, btn-ghost, card and card-pad, if it is their last user" (after the error page, the last old screen) and "drop its rules" (Header) come just before "delete passion.css".
   - Fix: cut all three. "delete passion.css" removes whatever is left. That is 3 fewer commits and one less owner decision.

13. Should fix. Two of the listed owner decisions are not the owner's, and some are missing.
   - Not the owner's: "Library lists as rows ... giving up `<table>`" is a false choice (see 7). "drop now the 380 passion.css class names" was already settled by plan.md (see 12).
   - Missing: exercise history "waits for the owner's review" (V2_DESIGN.md Part 5, step 1). The player's prefill and the Exercise screen build on it.
   - Missing: whether History ships without the mockup's "23 sets", or waits for server counts.
   - Missing: the rule for "≈ 75 min". The plan lists it as waiting, but never asks for it.
   - Missing: where the sound and buzz off switch lives.
   - Missing: which commits wait on the review of the run and cycle tables. Commits 15–58 all build on them.
   - Order: V2_DESIGN Part 5 says "Build exercise history, then session history". The plan builds History (59) before an exercise's history (61). Reorder, or ask.

14. Should fix. Some commits are missing.
   - A switch that turns each cue off (principle 5).
   - The get-ready countdown and the 3-2-1 beeps (plan.md A2, "for the real player"), or a line under "Left out" that says they are not built.
   - A dialog part. The confirm for Hold to end, swap and add each need one, and no part is listed.
   - A DEVELOPMENT.md update after commit 9. It says "Put a page that needs sign-in under `client/src/routes/(app)/`", and CLAUDE.md asks for an update when a workflow changes.

15. Should fix. The docs come too early.
   - Example: commit 6, "docs: describe the new look in DESIGN.md", lands before any screen uses the look. For about 70 commits DESIGN.md then describes none of the old screens. DEVELOPMENT.md step 4 still says to follow it.
   - Fix: land DESIGN.md with the first screen that uses the look, the player, and add to it as each part lands.

16. Should fix. `viewport-fit=cover` also changes the old screens.
   - Example: "which makes passion.css's 4 `env()` uses live". They are 4 lines (596, 1120, 1164 and 2155) with 5 calls, and only `.site-header` (596) is in a rule a screen uses. The old container has no side or bottom inset. In landscape the old screens run under the notch, and at the bottom under the home bar. Headless Firefox has no safe areas, so the screenshots cannot show this.
   - Fix: in the same commit, pad the shell layout for the side and bottom insets. Check one old screen on the owner's phone in landscape.

17. Should fix. Small numbers and claims that are wrong.
   - "56 of its 436 class names": `org` and `w3` come from a data URI (passion.css:4104), so there are 434 names. `text-sm` is a Tailwind class, and its only passion.css rule is `.run-playlist-head .text-sm` (1192). So 55 are used and 379 are not, not 380.
   - "19 lines in 10 files" of markup: ExerciseForm.svelte:149 is inside a scoped `<style>`, so the markup has 18 lines in 9 files.
   - "later than Tailwind 4's floor (Safari 16.4, March 2023)": that floor also lists Firefox 128, from July 2024 (tailwindcss.com/docs/compatibility). The argument holds for Safari 16.4 to 17.4 only.
   - "`pnpm --dir client check` ... is clean": the command exits 0 when there are warnings (svelte-check's `--fail-on-warnings` is off by default). So each check must read the warning count.
   - css_usage.py works for this repo, which builds no class names at run time. But it matches any word, not only class attributes, so `<input` counts as `.input` and `var(--card-muted)` as `.card-muted`. That is safe for drops, but label its "used" count as an upper bound.

18. Should fix. Principle 11 is quoted as the reason for one view.
   - Example: the plan quotes "Don't: a history card with a layout of its own." The principle asks that a part looks the same everywhere. Two views that share parts meet it too.
   - Fix: quote it for shared parts, not for one component tree.

Size: the 80 adds up (8+2+33+6+9+3+19). Giving each part its own commit before its first user follows the repo's db-then-api pattern, so it is not padding. With the fixes above (about +5 splits, +4 missing, −3 cuts), the total goes up, not down.

What checked out: the 139 forked lines, 2,127 .svelte lines in 12 route files and 13 components, 6 modules with 405 lines, `--muted` at passion.css 16/69/121/171, the MediaQuery quote, Tailwind 4.3.3's `@theme` flags, `--color-*: initial`, the `text-*` sub-keys and container sizes (md 28rem, 4xl 56rem), no palette colours in the markup, Svelte's Atrule handler and `css_unused_selector`, no counts in runSummaryBody, no colour in the day list, every API route the plan names, the mockup's 48 and 74 look lines, and Orca at /usr/bin/orca.

Verdict: the stance is sound and most facts about the repo check out. But the two-view comparison the owner asked for is tilted, the player cannot be reached in the app until its last commit, the plan contradicts itself on what widens, and it builds tokens for looks that will not ship. Fix findings 1–5 before this plan goes to the owner.
