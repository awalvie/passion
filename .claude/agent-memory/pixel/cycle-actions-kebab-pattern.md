---
name: cycle-actions-kebab-pattern
description: training_cycle_detail.html's page-header overflow menu is a deliberate parallel to site-nav-dropdown, not a missed reuse
type: project
---

`templates/training_cycle_detail.html` introduced `<details class="cycle-actions">` + `.cycle-actions-menu` (passion.css ~line 936) as a page-level "page-header overflow (kebab ⋯) menu" instead of reusing `site-nav-dropdown`. The plan (docs/plans/ui-ux-fixes.md item 5) originally said to reuse `site-nav-dropdown`, but the implementation deviated with a documented rationale in the CSS comment: `site-nav-dropdown-menu` has `@media (max-width:767px)` rules that force it to render inline (no floating menu) for the mobile nav-collapse pattern, which would break a page-header action menu that needs to float on mobile too.

This looks like a legitimate, justified deviation rather than an oversight — verified via screenshot (`v2-cycle-detail-kebab.png`), renders cleanly, destructive red text present, `hx-confirm` naming the cycle.

Open question raised with the user (not yet answered as of 2026-08-15): keep `.cycle-actions` as its own small documented component (candidate DESIGN.md entry: "page-header overflow menu"), or fold it into `site-nav-dropdown` via a new modifier class. Check whether this was decided before assuming either pattern is canonical on the next review that touches a similar header-kebab need.
