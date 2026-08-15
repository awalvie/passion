---
name: gc-chip-toggle-pattern
description: First appearance of a hidden-input/label-chip toggle component, introduced in new_cycle_guided.html
type: project
---

`templates/new_cycle_guided.html` introduced a new component not yet documented in DESIGN.md: a checkbox/radio "chip" toggle — native input hidden via `position:absolute;opacity:0`, label wraps a `<span>` that carries the visible chip styling (border/bg swap on `:checked`, outline on `:focus-visible`). Scoped under `.gc-*` classes in a page-local `<style>` block.

Verified via Playwright screenshot (light + dark) on 2026-08-15: renders correctly, focus ring visible and distinct from checked state, touch target (`min-height:2.75rem` on the span) looks adequate.

If more pages adopt this same chip-toggle pattern, it should graduate from a page-scoped `<style>` block into `passion.css` as a named, documented component (e.g. `.chip-toggle`) rather than being reinvented per page. Worth raising with the user if a second instance shows up elsewhere.

Also worth noting: this template is the first place a "you need a prerequisite before this page works" gating message appears (`{{ if not .Templates }}`) — the existing empty-state convention (`card card-pad text-center py-12` + icon at `opacity:0.3` + heading + subtext, see templates.html's own "No session templates yet" block) applies to this case too, since it's literally the same underlying condition. Check for this whenever a new page has a similar prerequisite gate.
