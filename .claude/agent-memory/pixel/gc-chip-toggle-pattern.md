---
name: gc-chip-toggle-pattern
description: First appearance of a hidden-input/label-chip toggle component, introduced in new_cycle_guided.html
type: project
---

`templates/new_cycle_guided.html` introduced a new component not yet documented in DESIGN.md: a checkbox/radio "chip" toggle — native input hidden via `position:absolute;opacity:0`, label wraps a `<span>` that carries the visible chip styling (border/bg swap on `:checked`, outline on `:focus-visible`). Scoped under `.gc-*` classes in a page-local `<style>` block.

Verified via Playwright screenshot (light + dark) on 2026-08-15: renders correctly, focus ring visible and distinct from checked state, touch target (`min-height:2.75rem` on the span) looks adequate.

If more pages adopt this same chip-toggle pattern, it should graduate from a page-scoped `<style>` block into `passion.css` as a named, documented component (e.g. `.chip-toggle`) rather than being reinvented per page. Worth raising with the user if a second instance shows up elsewhere.

**Update (2026-08-15, same page, next review pass):** the same template added a *second* novel page-scoped component before the first was ever promoted: `.gc-tags`/`.gc-tag-chip`/`.gc-tag-field` (a free-text tag input with an inline "×" remove button per chip, JS-driven add/remove, `<datalist>` autocomplete). Still scoped to `new_cycle_guided.html`'s local `<style>` block, still undocumented in DESIGN.md. No sibling page uses either `.gc-chip` or `.gc-tags` yet. Raised with the user again whether to promote now (two components on one page) or keep waiting for a second *page* to adopt one of them — answer not yet recorded as of this note.

**Update (2026-08-16): promoted.** All `.gc-*` styles (chip toggle, tags, goal rows, day rows, energy chips) now live in `static/passion.css` (~line 4014 onward), no more page-local `<style>` block. `.gc-chip` is used across focus/time-mode/days/energy selectors; `.gc-goal-row` (before→after goal editor) now spans two pages — `new_cycle_guided.html` and `training_cycle_detail.html`'s "Cycle details" disclosure — confirming it graduated to a real shared component rather than a one-off. Superseded concern: touch target on the new `.gc-goal-remove` button is 2rem (32px), under the 44px minimum other `.gc-*` controls respect — flagged in the 2026-08-16 review, not yet fixed as of this note.

Also worth noting: this template is the first place a "you need a prerequisite before this page works" gating message appears (`{{ if not .Templates }}`) — the existing empty-state convention (`card card-pad text-center py-12` + icon at `opacity:0.3` + heading + subtext, see templates.html's own "No session templates yet" block) applies to this case too, since it's literally the same underlying condition. Check for this whenever a new page has a similar prerequisite gate.
