# Research: modern app-like mobile web with SvelteKit — vs. Passion client today

Scope: `/home/awalvie/code/lamp/passion/client` (SvelteKit 5, runes, Tailwind 4,
`@sveltejs/adapter-static`, served by the Go binary). No files were edited, no
servers started — this is read-only research plus web lookups. All web claims
are cited to the fetched/searched source; anything I could not verify from a
primary source is flagged "not verified" rather than stated as fact.

## 0. Client baseline (read directly, verified)

- `client/package.json`: `svelte ^5.56.1`, `@sveltejs/kit ^2.63.0`,
  `@sveltejs/adapter-static ^3.0.10`, `tailwindcss ^4.3.3`, `vite ^8.0.16`. No
  `@capacitor/*`, no `vite-plugin-pwa`, no `workbox-*` packages.
- `client/src/routes/+layout.ts`: `export const ssr = false; export const
  prerender = false;` with comment "The client is a single-page app served as
  static files by the Go binary and, later, wrapped by Capacitor." So **every**
  route is client-rendered only, nothing is prerendered.
- `client/vite.config.ts`: `adapter-static` configured with
  `fallback: 'index.html'`. Per the adapter's own docs (fetched, see §1), a
  fallback page is what makes this SPA mode; the docs advise avoiding
  `index.html` as the fallback name "to avoid conflicting with a prerendered
  homepage" — moot here since nothing is prerendered, so no actual conflict.
- Routes today (verified via `find`): `login/`, `signup/`, and, behind
  `(app)` (which gates on a token in `(app)/+layout.ts`), `exercises/` and
  `templates/` only. No dashboard, run/session, history, or cycles routes
  exist in the Svelte client yet — the readme and the passion.css comment
  ("carried over from v1 unchanged") confirm the large `.run-*`, calendar and
  heatmap CSS blocks are legacy styling waiting for those screens to be
  rebuilt, not evidence the screens exist today.
- `client/src/app.html`: sets `viewport` to `width=device-width,
  initial-scale=1` only (no `viewport-fit=cover`), a theme-flash-prevention
  inline script, and nothing else PWA-related — no `<link rel="manifest">`,
  no `apple-touch-icon`, no `theme-color` meta.
- `client/static/`: only `robots.txt`. No `manifest.json`/`.webmanifest`, no
  icons beyond `src/lib/assets/favicon.svg`, no `service-worker.js`.
- `client/src/lib/Header.svelte` + root `client/src/routes/+layout.svelte`:
  a single sticky top header (logo + collapsible nav), no bottom tab bar
  anywhere in the codebase (`grep -n "bottom-tab\|tabbar\|tab-bar"
  src/passion.css` → no hits).
- Scratchpad note `scratchpad/research-mobile-deploy.md` (from an earlier
  session, read for context): Capacitor is a *planned* later step, not
  started — no `capacitor.config.*`, no CORS middleware on the Go server yet.
  Relevant here because several PWA/mobile choices below (manifest, service
  worker, icons) are exactly what Capacitor will also need, so doing them now
  is not wasted work.

---

## 1. PWA: manifest, service worker, install, theme-color, apple-touch-icon, standalone

**Manifest** — MDN, "Web application manifest":
https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest.
A manifest is a JSON file (conventionally linked via `<link rel="manifest"
href="/manifest.json">`) whose members include `name`, `icons`, `start_url`,
`display`, `theme_color`, `background_color`. web.dev's PWA course
(https://web.dev/learn/pwa/web-app-manifest) covers the same fields.

**Install criteria** — MDN "Making PWAs installable"
(https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_installable)
and web.dev (https://web.dev/learn/pwa/installation-prompt): the baseline
cross-browser requirement is a valid manifest + a registered service worker +
HTTPS + (for a custom install UI) listening for the `beforeinstallprompt`
event, saving the event, and calling `.prompt()` later. web.dev's own
articles note `beforeinstallprompt`/`appinstalled` moved out of the manifest
spec into their own incubator, but Chrome has "no plans to remove or
deprecate support" (https://web.dev/promote-install/index.html /
https://web.dev/articles/customize-install). iOS Safari does **not** fire
`beforeinstallprompt` — there install is manual ("Add to Home Screen"), which
is why the legacy Apple meta tags in the next paragraph still matter.

**Service worker in SvelteKit** — fetched
https://svelte.dev/docs/kit/service-workers directly: a file at
`src/service-worker.js` (or `src/service-worker/index.js`) "will be bundled
and registered automatically." Inside it you get the `$service-worker`
module: `build` (Vite-built asset URLs, empty array in dev), `files` (static
dir contents), `version` (a string for cache-busting), and `prerendered`
(empty array in Passion's case, since nothing is prerendered). The docs'
own caution: "Be careful when caching! In some cases, stale data might be
worse than data that's unavailable while offline." Default registration
(if you don't do it by hand) is
`navigator.serviceWorker.register('./path/to/service-worker.js', { type: dev
? 'module' : 'classic' })` — confirmed by direct fetch of the doc page.
Because Passion's client is 100% client-rendered with `fallback:
'index.html'` and no prerendering, there's no interaction with the
adapter-static "ssr must not be false for prerendering" constraint (fetched
from https://svelte.dev/docs/kit/adapter-static: "ensure SvelteKit's ssr
option isn't set to false" — that requirement is about *prerendering*
producing real HTML, and Passion prerenders nothing, so it doesn't apply).

**theme-color** — MDN manifest docs (above) and general HTML meta docs: a
`<meta name="theme-color" content="...">` tag (and/or the manifest's
`theme_color`) tints the browser UI (status bar / task-switcher chrome) to
match the app. Not present in `app.html` today.

**apple-touch-icon / iOS-specific standalone meta** — fetched Apple's own
"Configuring Web Applications" doc
(https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html):
- `<meta name="apple-mobile-web-app-capable" content="yes">` — "Enables
  standalone mode ... no URL bar or button bar—only a status bar appears."
- `<meta name="apple-mobile-web-app-status-bar-style" content="black">` —
  status bar styling, requires the line above.
- `<link rel="apple-touch-icon" href="...">`, with `sizes="180x180"` etc. for
  the home-screen icon; the manifest's `icons` array is not fully honored by
  iOS home-screen installs historically, so this link tag is the reliable
  path there. Apple's doc does **not** mention `theme-color` at all — that's
  a separate (Chromium/Android-originated) mechanism.

**display: standalone** — MDN manifest docs: `"display": "standalone"` makes
the installed app open without browser chrome (address bar, back/forward
buttons), the core "feels native" signal.

**A maintained shortcut that needs no manual manifest/SW plumbing**:
`@vite-pwa/sveltekit` (fetched https://github.com/vite-pwa/sveltekit) — a
zero-config Vite plugin, `npm i @vite-pwa/sveltekit -D` then
`SvelteKitPWA()` in the plugins array, generates the service worker via
Workbox and wires up the manifest and update prompts. It's a real, working
option, but it is a new dependency where SvelteKit's own built-in
`src/service-worker.js` + a hand-written `static/manifest.json` do the same
job with zero new packages — given the project's "prefer no new dependency"
rule, the built-in route is the one to reach for first.

**What the client does today**: none of the above. No manifest, no service
worker, no theme-color, no apple-touch-icon, no install path. (Verified:
`find client/static` → only `robots.txt`; `grep` of `app.html` for
`viewport-fit|apple-touch-icon|theme-color|manifest` → no hits.)

---

## 2. Viewport and safe areas

**viewport-fit=cover** — searched (WebmasterWorld/MDN summaries) and cross-
checked against MDN's `env()` page
(https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/env):
`viewport-fit=cover` in the viewport meta tag lets the page extend under
notches/rounded corners/home-indicator area; without it, `env(safe-area-inset-*)`
values are effectively 0 because the browser never lets content under those
regions in the first place.

**env(safe-area-inset-*)** — MDN `env()`: "the safe distance from the top,
right, bottom, or left inset edge of the viewport... where it is safe to
place content into without risking it being cut off by the shape of a
non-rectangular display." Originated on iOS Safari, now broadly supported.

**Client today**: `client/src/app.html:5` has
`<meta name="viewport" content="width=device-width, initial-scale=1" />` —
**no `viewport-fit=cover`**. Yet `client/src/passion.css` already uses
`env(safe-area-inset-top, 0px)` (line 596, `.site-header` padding),
`env(safe-area-inset-bottom, 0px)` (lines 1120 `.run-shell`, 1164
`.run-playlist-panel.run-playlist-open`, 2155 a bottom bar). **This is a live
inconsistency**: those `env()` calls are no-ops without `viewport-fit=cover`
in the viewport meta, so today they're falling back to their `0px` default
on real notched devices — the code is already shaped for safe-area handling,
it's one meta-tag change away from actually doing anything.

**100dvh** — fetched https://web.dev/blog/viewport-units directly: `100vh`
"bleed[s] out of the viewport" or leaves a gap as mobile browser toolbars
show/hide; `svh` = viewport with dynamic UI *expanded* (smallest), `lvh` =
UI *retracted* (largest), `dvh` = "adapts" live as the toolbar animates.
Chrome shipped it in Chrome 108, alongside existing Safari/Firefox support.
**Client today**: no `dvh`/`svh`/`lvh` anywhere (`grep -rn "dvh\|100vh" src`
→ no hits) — not yet a problem only because there's no full-height layout
today (no run/session screen exists yet in the Svelte client), but it is
exactly the kind of thing to get right before building the run-timer screen,
where a fixed `100vh` would visibly jump as the URL bar hides.

**overscroll-behavior** — already used, twice, correctly: `passion.css:234`
`overscroll-behavior-x: none` on `html, body` (with a comment explaining it
kills horizontal rubber-banding while preserving `position: sticky`), and
`overscroll-behavior: contain` on the full-screen playlist overlay
(`passion.css:1163`). MDN backs the standard behavior (searched, standard
usage confirmed): `contain` stops scroll chaining to the parent without
killing the overlay's own bounce; `none` kills both scroll chaining and the
native bounce/glow effect.

**iOS 16px input-zoom rule** — cross-checked via CSS-Tricks' well-known
writeup (https://css-tricks.com/16px-or-larger-text-prevents-ios-form-zoom/)
and community sources: iOS Safari auto-zooms the viewport on focus for any
input whose **computed, rendered** font-size is under 16px; setting
`font-size: 16px` (or `1rem` with a 16px root) on the focused control is the
fix, no viewport-meta hacks needed (and `user-scalable=no` is explicitly the
wrong fix — it breaks pinch-zoom accessibility). **Client status: mixed.**
`client/src/passion.css:381-391`, the shared `.input` class (used by every
`<input>`/`<select>`/`<textarea>` styled per docs/DESIGN.md's "All inputs
and selects: `class=\"input text-sm\"`") sets `font-size: 0.875rem` — **14px,
under the threshold**, so any of those fields will trigger iOS zoom-on-focus
today. One field already has the fix applied with the exact right reasoning
in a comment: `passion.css:1513-1527`, `.run-notes-textarea` sets
`font-size: 1rem` with the comment "16px prevents iOS Safari from zooming in
on focus." So the team already knows the rule; it just isn't applied to the
general `.input` class yet.

---

## 3. Navigation: bottom tab bar vs. header, large title, back behaviour

**Material Design 3** — fetched via search, https://m3.material.io/components/navigation-bar/guidelines:
the component (renamed from "Bottom navigation" to "Navigation bar") is for
"3 to 5 destinations," "positioned at the bottom of windows for convenient
access," and is scoped to "compact window widths smaller than 600dp" (i.e.
phone portrait) — on larger/medium widths Material recommends a navigation
rail instead.

**Apple HIG** — fetched via search,
https://developer.apple.com/design/human-interface-guidelines/tab-bars and
.../navigation-bars: tab bars are for navigating "top-level sections," "use
three to five tabs in iOS." Large titles are "an opt-in version [of the nav
bar] that transitions from the standard nav bar when you start scrolling" —
best "at the top level of an app with tabs." The navigation bar's back
button "changes to reflect the title of the screen you just came from,"
i.e. back behaviour is tied 1:1 to the navigation stack, not a generic
"go back."

**Client today**: one persistent, sticky **top** header
(`client/src/lib/Header.svelte` + `.site-header` in `passion.css:589-601`,
`position: sticky; top: 0`), no bottom tab bar anywhere. Navigation is a
hamburger-style collapsible menu on mobile (`site-header-menu-btn`,
`peer-checked:flex` pattern in `Header.svelte:64-84`), not a large-title /
tab-bar pattern. "Back" is handled by the browser's own back button / swipe
edge-gesture (no custom back button in the header, no `history.back()` calls
found — `grep -rn "goto("` shows only forward navigations after
create/save/delete actions). With only 4 real destinations today
(dashboard/home, exercises, templates, plus account actions) the app is
already within Material's/Apple's 3-5-destination sweet spot for a bottom
tab bar, if the owner wants that pattern later — but that's a design
decision to make once the dashboard/run/history routes exist, not a partial
retrofit onto today's 2-item nav.

---

## 4. Motion: View Transitions, Svelte transitions/animate:flip, spring/tweened, prefers-reduced-motion

**View Transitions API** — fetched via search, MDN
https://developer.mozilla.org/en-US/docs/Web/API/View_Transition_API:
`document.startViewTransition(callback)` snapshots the current DOM, runs your
callback (which does the actual state/DOM change), then cross-fades/animates
to the new state. For SPA (same-document) transitions this is the relevant
entry point.

**SvelteKit's `onNavigate`** — fetched via search (svelte.dev blog "Unlocking
view transitions in SvelteKit 1.24" + `$app/navigation` docs): `onNavigate`
is a lifecycle function, called during component init, that "runs the
supplied callback immediately before we navigate to a new URL" (client-side
navigations only, not full-page loads). If the callback returns a `Promise`,
SvelteKit waits for it before completing the navigation — which is exactly
the hook point for `document.startViewTransition`. The canonical pattern
(from the SvelteKit blog, matches multiple independent write-ups found):
```js
import { onNavigate } from '$app/navigation';
onNavigate((navigation) => {
  if (!document.startViewTransition) return;
  return new Promise((resolve) => {
    document.startViewTransition(async () => {
      resolve();
      await navigation.complete;
    });
  });
});
```

**Svelte's own transition/animate primitives** — fetched via search,
https://svelte.dev/docs/svelte/svelte-animate and the motion docs: `animate:flip`
(from `svelte/animate`) computes each list item's before/after position and
animates the delta ("First, Last, Invert, Play") — the standard tool for
reordering a list (e.g. Passion's drag-and-drop exercise/activity lists).
`transition:` directives (`fade`, `fly`, `slide`, etc. from `svelte/transition`)
animate an element's own enter/exit.

**svelte/motion (`spring`, `tweened`)** — fetched via search,
https://svelte.dev/docs/svelte/svelte-motion: `tweened` animates over a fixed
duration/easing; `spring` animates with physics (`stiffness`/`damping`), so it
keeps "existing velocity" rather than restarting — the natural fit for
something like an animated progress ring or a big countdown timer that
should feel springy rather than mechanically eased.

**prefers-reduced-motion** — MDN
(https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion):
a media feature reflecting the OS-level "reduce motion" accessibility
setting; supported in every major browser.

**Client today**: none of View Transitions / `onNavigate` / `svelte/animate`
/ `svelte/motion` are used anywhere (`grep -rn "onNavigate\|startViewTransition\|svelte/motion\|svelte/animate\|svelte/transition"` across
`src` → zero hits). `prefers-reduced-motion` **is** used, once, correctly:
`passion.css:1912-1916` disables the `.run-phase-text` colour transition
under `@media (prefers-reduced-motion: reduce)`. That's a good, narrow
precedent to extend once more transitions/animations are added, rather than
a gap to close from scratch.

---

## 5. Touch: 44px targets, :active, tap-highlight-color, touch-action, swipe, bottom sheets, vibrate

**44px / 44pt targets** — searched: Apple HIG's "default target size of
44x44 pt," matching WCAG 2.5.5 (AAA, same 44×44 CSS px). The *mandatory*
WCAG 2.2 (AA) bar is lower: Success Criterion 2.5.8 Target Size (Minimum)
requires **24×24 CSS px** with several exceptions (inline text targets,
sufficient spacing between small adjacent targets, an equivalent control
elsewhere, essential presentation) — confirmed via multiple accessibility-
guide summaries of the W3C criterion. So 44px is the "feels great on a
phone" bar (Apple/WCAG AAA), 24px is the legal-minimum floor (WCAG AA).

**Client today**: docs/DESIGN.md already states the house rule precisely —
"any button that is the primary action in a mid-session flow gets a minimum
44px height... Add `.btn` to opt into that size; it is the only class
carrying `min-height: 2.75rem`" (2.75rem = 44px) — and that's implemented:
`passion.css:281` `.btn { min-height: 2.75rem; ... }`, plus explicit 44px
targets on `.site-header-menu-btn` (`min-width/height: 2.75rem`,
`passion.css:645-646`) and `.run-nav-arrow` (`width/height: 2.75rem`,
`passion.css:1408-1409`). This is a deliberate, already-correct two-tier
system (44px for primary/mid-flow actions, smaller for dense table rows and
pills) — matches the guidance, not a gap.

**:active feedback** — used in several places already: `.btn:active {
transform: translateY(1px); }` (`passion.css:294`), `.session-card:active`
(477), `.dnd-handle:active` (510), `.stepper-btn:active` (3555),
`.mobile-agenda-session:active { opacity: 0.65; }` (4166). Reasonably
covered for the buttons/cards that have it; not universal (e.g. plain
`.site-header-link` has `:hover` only, no `:active`), which matters more on
touch since there's no hover state to fall back on.

**-webkit-tap-highlight-color** — MDN
(https://developer.mozilla.org/en-US/docs/Web/CSS/-webkit-tap-highlight-color):
non-standard WebKit-only property controlling "the color of the highlight
that appears over a link while it's being tapped"; commonly set to
`transparent` when a design already has its own `:active` treatment (to
avoid a double effect — the grey flash *and* the custom `:active` state
firing together). **Client today**: not set anywhere (`grep` → no hits), so
on Chrome/Android any tappable element without it may show the default grey
overlay on top of the custom `:active` styles above.

**touch-action** — MDN: controls which touch gestures the browser handles
natively vs. leaves to the page (`manipulation` = allow panning/pinch-zoom
but drop the legacy double-tap-to-zoom delay). **Client today**: set
globally, correctly — `passion.css:218`, `html { touch-action: manipulation;
}` — this already removes the ~300ms tap-delay app-wide.

**Swipe gestures** — no dedicated pointer/touch event handling anywhere in
the client (`grep -rn "touchstart\|touchmove\|touchend\|pointerdown\|pointermove\|Hammer\|swipe"` across `src` → zero hits). Not a gap yet because
there's no carousel/list-with-swipe-actions built in the Svelte client today
(the `.run-hero-carousel` CSS exists in passion.css but that screen isn't
built here yet) — flagging it as a "when you build that screen" concern, not
a present-day bug.

**Bottom sheets via `<dialog>`** — fetched MDN directly
(https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/dialog):
`showModal()` opens a modal `<dialog>` with focus-trapping and inert
background ("Modal dialog boxes block interaction with other UI elements,
making the rest of the page inert"); `::backdrop` styles the scrim; Esc
closes it by default. MDN's own page does **not** describe a bottom-sheet
pattern explicitly — that's a CSS layout choice on top of `<dialog>` (anchor
it to the bottom of the viewport, animate `transform: translateY()`), not
something MDN documents as a named variant; I could not find a primary-
source confirmation of "bottom sheet" as an official `<dialog>` use case,
only third-party blog patterns, so I'm not stating that as settled doc
guidance, just as a buildable pattern on top of a documented primitive.
**Client today**: no `<dialog>` element anywhere in `client/src` (only
`<details>`, used for the exercise/activity editors' expand-collapse UI in
`SectionEditor.svelte`, `ChoiceEditor.svelte`, `TemplateEditor.svelte`,
`StepEditor.svelte`, and the account/library dropdown menus in
`Header.svelte`/`passion.css`). `<details>` gives no focus trap, no Esc-to-
close, no backdrop — fine for an inline disclosure widget, not a substitute
for a modal/sheet if one is ever needed (e.g. a "confirm delete" or a
mobile filter sheet).

**navigator.vibrate()** — fetched MDN's Vibration API page directly
(https://developer.mozilla.org/en-US/docs/Web/API/Vibration_API): flagged
"Limited availability... not Baseline because it does not work in some of
the most widely-used browsers." Search results (not the MDN page itself)
report the underlying reason: WebKit removed vibration support from iOS
Safari in 2017 over "concerns of user annoyance and API misuse," and it
remains unsupported on iOS Safari today — **I could not get MDN's own
compat-table text via fetch (it renders client-side), so this iOS-unsupported
claim rests on secondary sources (a W3C implementation-report reference and
multiple 2024-2026 community threads), not a directly quoted MDN sentence;
flagging that gap rather than overstating certainty.** Net effect either
way: never gate feedback on vibration alone — pair it with a visual/`:active`
cue (which the client already has, see above), and treat `navigator.vibrate`
as a bonus for Android, not something to rely on for iOS. **Client today**:
not used anywhere (`grep -rn "vibrate"` → no hits).

---

## 6. Screen Wake Lock (for a future timer/run screen)

Fetched via search, MDN's Screen Wake Lock API summary
(https://developer.mozilla.org/en-US/docs/Web/API/Screen_Wake_Lock_API):
`navigator.wakeLock.request('screen')` returns a `WakeLockSentinel`
Promise; "This feature is available only in secure contexts (HTTPS)... The
API throws on HTTP and on `file://` pages. Chrome, Firefox, and Safari gate
`navigator.wakeLock` to HTTPS and to `http://localhost`." A request can be
rejected for OS-level reasons (low battery, power-saving mode) or if the
document isn't visible/active, so a wake lock has to be re-requested on
`visibilitychange` if the sentinel gets released while backgrounded — this
is documented behavior, not an edge case to skip. **Relevant directly**:
Passion's dev flow already runs over HTTP on `localhost` (per
`vite.config.ts`'s dev proxy setup) so this is a non-issue in dev; in
production the Go server would need to actually be served over HTTPS for
Wake Lock to work at all — worth confirming as part of whatever reverse
proxy/TLS setup fronts the production deployment. **Client today**: no
run/session/timer screen exists yet in the Svelte client, so Wake Lock isn't
applicable yet — this is a "build it in from day one of the run screen"
item, not a retrofit.

---

## 7. Component libraries for Svelte 5 — what each gives, and the recommendation

- **bits-ui** (https://bits-ui.com, https://github.com/huntabyte/bits-ui) —
  headless (no shipped visual styling), 40+ accessible primitives, built
  natively for Svelte 5 runes (confirmed via search of its own docs/README
  language: "Svelte 5 native, built specifically for Svelte 5 using runes").
  Cost: a real new dependency, MIT-licensed, you still write 100% of the
  visual layer.
- **shadcn-svelte** (https://shadcn-svelte.com) — not a library you `npm
  install` and import from; a CLI (`npx shadcn-svelte@latest add <component>`)
  copies component *source* into your repo, which you then own and edit
  directly. It's built on top of bits-ui for behavior + Tailwind for
  styling. Confirmed ported to Svelte 5 runes
  (https://shadcn-svelte.com/docs/migration/svelte-5). Cost: pulls in
  bits-ui transitively for every component you add, plus whatever Tailwind
  conventions (a `cn()` util, CSS variables) its `init` step wants to lay
  down — those may collide with Passion's existing hand-rolled token system
  in `passion.css`/`docs/DESIGN.md` rather than compose with it.
- **Melt UI** (https://melt-ui.com, https://github.com/melt-ui/melt-ui) —
  headless "builders" (attach-to-any-element pattern), described in its own
  README as "under active development... worked on by volunteers in their
  free time" — I could not confirm from the README content actually fetched
  whether it has Svelte 5 support or has been effectively superseded by
  bits-ui (same author base); treat that as an open question, not a
  confirmed fact, if this one is ever reconsidered.
- **Skeleton** (https://skeleton.dev) — a fuller "toolkit" (components +
  Tailwind design-token layer + its own theming system) rather than headless
  primitives; latest major is `skeleton@5.0.0`. I could not confirm Tailwind
  CSS 4 compatibility specifically from what I fetched — flagging as
  unverified rather than assuming yes given Passion is already on Tailwind 4.
  Cost: it brings its own opinionated design-token/theming layer, which
  would directly compete with Passion's existing `--text`/`--panel`/`--border`
  etc. token system in `passion.css` — redundant, not additive.
- **Flowbite-Svelte** — the *stable* published package is Svelte-4-only; the
  Svelte-5 rewrite is a separate, still-early project
  (`flowbite-svelte-next` / `next.flowbite-svelte.com`), self-described as
  "early development... APIs and packages are likely to change quite often."
  Not a safe pick today for a Svelte 5 codebase.

**Recommendation**: none of these earn their place right now. Passion's
design system (`docs/DESIGN.md`) is already a small, specific, hand-rolled
CSS vocabulary (`.card`, `.btn-*`, `.input`, the tick/badge token families) —
adding shadcn-svelte or Skeleton means running two competing design-token
systems side by side, and Passion's actual current UI needs (dropdowns,
`<details>` disclosures, a sticky header) are already served by plain HTML
elements plus the existing CSS. The one plausible future trigger: if a
screen needs a real interaction pattern that's expensive to hand-roll
correctly — an accessible combobox/autocomplete, or a drag-to-dismiss sheet
with focus-trapping — **bits-ui** is the one to reach for, precisely
because it's headless (no visual opinions to fight) and is the only one of
the five confirmed to be built for Svelte 5 runes from the ground up. Until
that need exists, stay dependency-free.

---

## 8. Open-source SvelteKit apps to learn from

- **Immich** — https://github.com/immich-app/immich (web app source:
  `github.com/immich-app/immich/tree/main/web`). Verified directly, two
  ways: (1) its own README states "This project uses the SvelteKit web
  framework"; (2) a direct GitHub API listing of `web/static/` (fetched via
  `curl` to `api.github.com/repos/immich-app/immich/contents/web/static`)
  shows exactly the PWA assets this research covers in practice:
  `manifest.json`, `manifest-icon-192.maskable.png`,
  `manifest-icon-512.maskable.png`, and `apple-icon-180.png`. Immich is one
  of the most widely deployed self-hosted apps in this exact category
  (Go/TS backend + web frontend, run on a home server, used from a phone
  browser) and is broadly well-regarded for its UI — worth reading its
  `web/static` and `web/src/app.html`-equivalent directly as a working
  reference for manifest + icon setup. I did not independently verify
  community "praise" claims beyond its general popularity (huge self-hosted
  user base, frequently recommended) — treat "well-praised" as reputation-by-
  popularity, not a cited review.
- Everything else I found in search (svelte-hackernews, various "svelte-pwa"
  starter templates, `supasveltic`) were toy/demo projects, not real apps
  with a track record — I'm deliberately not listing those as "well-praised"
  examples since I have no evidence for that claim.

---

## Flat list: convention → client today → effort

- **Web app manifest** (`manifest.json` + `<link rel="manifest">`) — not
  present (`client/static/` has only `robots.txt`; `app.html` has no
  manifest link). **S** — one static JSON file + one `<link>` tag in
  `app.html`.
- **Service worker** (`src/service-worker.js`, auto-registered per
  kit.svelte.dev docs) — not present. **S–M** — trivial for a "cache the
  built assets" version; **M** if real offline/update-prompt UX is wanted.
- **`theme-color` meta / manifest `theme_color`** — not present
  (`app.html` has no such meta). **S** — one line, reuse `--bg`/`--panel`
  token values already in `passion.css:11-13`.
- **`apple-touch-icon` + `apple-mobile-web-app-capable` meta** — not present.
  **S** — needs a 180×180 PNG (currently only `favicon.svg` exists) plus two
  `<link>`/`<meta>` lines in `app.html`.
- **`display: standalone`** (manifest field) — not present (no manifest at
  all yet). **S** — one manifest field, bundled with the manifest task above.
- **`viewport-fit=cover`** — not present in `app.html:5`'s viewport meta.
  **S** — one attribute, but must ship together with an
  audit of the safe-area paddings below it currently silently no-ops.
- **`env(safe-area-inset-*)`** — used in 4 places in `passion.css` (596,
  1120, 1163, 2155) but currently inert without `viewport-fit=cover`. **S** —
  fixed by the viewport change above; no new CSS needed, just verify each of
  those 4 spots visually on a notched-device simulator afterward.
- **`100dvh` / dynamic viewport units** — not used anywhere yet; no full-
  height layout exists yet either. **S** when the run/session screen is
  built (use `dvh` from day one there) — retrofitting later would be **M**
  if a `100vh` layout ships first and has to be found and replaced.
- **`overscroll-behavior`** — already used correctly (`passion.css:234`,
  `:1163`). **Done**, no change.
- **iOS 16px input-zoom rule** — violated by the shared `.input` class
  (`passion.css:381-391`, `font-size: 0.875rem` = 14px), while
  `.run-notes-textarea` already does it right (`passion.css:1513-1527`,
  explicit comment). **S** — bump `.input`'s `font-size` to `1rem`; check
  knock-on layout (the class is used everywhere per DESIGN.md, so this is a
  single-class, high-blast-radius but low-complexity change — worth a visual
  pass after).
- **Bottom tab bar vs. header** — client has only a sticky top header +
  hamburger menu (`Header.svelte`, `passion.css:589-601`), no bottom tab bar.
  **L** — a real navigation-pattern decision (which the project's own rules
  say to ask about before building, since it's an ambiguous UX call), plus
  new component work once there are enough top-level destinations (today
  there are effectively 2: exercises, templates) to justify 3-5 tabs.
- **Large title pattern** — not present; header is a fixed small title, no
  scroll-collapsing large-title behavior. **M** — a scroll-linked CSS/JS
  behavior on top of the existing sticky header.
- **Back behaviour** — relies entirely on the native browser back
  gesture/button today (no custom back button, confirmed via `grep` for
  `history.back`/back UI in `Header.svelte` and routes). **Done for now** —
  matches Apple's own guidance that back should track the navigation stack,
  which the browser already does; only becomes a gap if a bottom-tab pattern
  is adopted later (tabs need their own back-stack-per-tab handling).
- **View Transitions API + `onNavigate`** — not used anywhere (`grep` → zero
  hits). **S** — the documented `onNavigate` + `startViewTransition` snippet
  is ~10 lines in the root `+layout.svelte`, guarded by a
  `document.startViewTransition` feature check (auto-degrades on
  unsupported browsers).
- **Svelte `transition:`/`animate:flip`** — not used anywhere yet. **S per
  usage** — apply directly where lists reorder (the existing drag-and-drop
  exercise/activity lists in `SectionEditor.svelte`/`TemplateEditor.svelte`
  are the natural first candidates) or where cards enter/exit.
- **`svelte/motion` (`spring`/`tweened`)** — not used anywhere yet. **S per
  usage**, relevant once there's a run-timer or progress-ring UI to animate.
- **`prefers-reduced-motion`** — used once already, correctly
  (`passion.css:1912-1916`, `.run-phase-text`). **S per new
  transition/animation added** — just remember to wrap each new one, same
  pattern.
- **44px touch targets** — already correct and documented as house policy
  (`docs/DESIGN.md`'s Responsive section; `.btn` in `passion.css:281`,
  `.site-header-menu-btn` at 645-646, `.run-nav-arrow` at 1408-1409).
  **Done**, no change.
- **`:active` feedback** — present on `.btn`, `.session-card`,
  `.dnd-handle`, `.stepper-btn`, `.mobile-agenda-session`, but not
  universal (e.g. `.site-header-link` only has `:hover`). **S** — add
  `:active` alongside existing `:hover` blocks where a tap has no other
  affordance.
- **`-webkit-tap-highlight-color`** — never set (`grep` → zero hits).
  **S** — one global rule (e.g. `-webkit-tap-highlight-color: transparent;`
  on `html` or per-component), paired with making sure `:active` styles
  exist first (previous item) so taps don't go silent.
- **`touch-action`** — set globally and correctly
  (`passion.css:218`, `html { touch-action: manipulation; }`). **Done**.
- **Swipe gestures** — none implemented; no screen needs them yet (the
  carousel CSS in `passion.css` has no built Svelte component behind it
  today). **M–L** whenever that screen is built — real swipe handling
  (pointer events + a small physics/threshold model, or a small library) is
  nontrivial to get right, not a quick add.
- **Bottom sheets via `<dialog>`** — `<dialog>` is unused; only `<details>`
  (no focus trap/Esc/backdrop) is used for disclosure UI today. **M** per
  sheet — `showModal()` + `::backdrop` + a bottom-anchored transform is a
  well-defined but real small component to build, reusable once built.
- **`navigator.vibrate()`** — unused. **S**, but low value alone since it's
  unsupported on iOS Safari (per secondary sources — MDN's own compat table
  text wasn't directly quotable here) — only worth adding paired with an
  existing visual/`:active` cue, never as the sole feedback signal.
- **Screen Wake Lock** — unused; no timer screen exists yet to need it.
  **S** once the run/timer screen exists (a `navigator.wakeLock.request`
  call + a `visibilitychange` re-request handler is short), gated on
  production actually serving over HTTPS.
- **Component library adoption** (bits-ui/shadcn-svelte/Melt UI/Skeleton/
  Flowbite-Svelte) — none adopted; recommend continuing that way. **N/A** —
  explicitly not recommended today; revisit only if a specific hard
  interaction pattern (accessible combobox, drag-dismiss sheet) is needed,
  and reach for bits-ui specifically then (see §7).

## Uncertainties / things I could not verify from a primary source

- Whether MDN's own compatibility table lists iOS Safari as unsupported for
  `navigator.vibrate()` — the MDN page fetched didn't render the compat
  table text; this rests on secondary sources (search snippets, a W3C
  implementation-report reference).
- Whether Melt UI has Svelte 5 support or has been effectively superseded by
  bits-ui — its README (fetched) didn't state either way.
- Whether Skeleton (skeleton.dev) supports Tailwind CSS 4 — not confirmed
  from what was fetched/searched.
- Whether "bottom sheet" is an MDN-documented named pattern for `<dialog>` —
  it is not; it's a CSS layout built on top of a documented primitive, and I
  found only third-party blog descriptions of the specific pattern.
