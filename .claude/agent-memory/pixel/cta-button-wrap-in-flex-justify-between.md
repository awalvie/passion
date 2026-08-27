---
name: cta-button-wrap-in-flex-justify-between
description: A .btn-primary anchor placed as a flex sibling of a wrapping filter-controls group (sm:flex-row sm:justify-between) wraps its own text to 2 lines at in-between widths unless given shrink-0/whitespace-nowrap
type: feedback
---

**Rule**: When a page header is `flex flex-col gap-N sm:flex-row sm:items-center sm:justify-between` with a filter/search control group as one child and a `.btn-primary` "New X" anchor as the other, the anchor needs `shrink-0 whitespace-nowrap` (or equivalent). Without it, flex's default `min-width:auto` lets the anchor shrink below its content's min-content width once the filter group's own internal wrapping no longer leaves enough row space — the button text wraps to 2 lines and the button balloons from `.btn`'s normal 36px height to ~56px.

**Why**: Verified live (2026-08-27, narrow-desktop audit) on `/templates` and `/activity-templates` at exactly 768px viewport — "New Session Template" / "New Activity Template" wrapped to 2 lines (56px tall) at 768px but rendered fine at 1024px and below 640px (stacks full-width instead). `/exercise-library`'s "New saved exercise" button never had the problem because its layout puts the button in a header row separate from the filter form, so it's never a flex sibling of the wrapping controls.

**How to apply**: When reviewing any page-header CTA button that sits beside a filter/search control group in a shared flex row, check the button's rendered height at the exact breakpoint where the filter group starts wrapping (usually `sm:`/640px or `md:`/768px) — not just at the extremes.

**Resolved 2026-08-27 by moving the CTA, not by `shrink-0`.** `shrink-0 whitespace-nowrap` stops the wrap but leaves the button in a row whose length depends on the user's data — those filter controls are behind `{{ if .DistinctSources }}` / `{{ if .DistinctTags }}`, so the button keeps *moving* as the catalog grows. Both `/templates` and `/activity-templates` now use `exercise_library.html`'s structure: the CTA shares the header row with the H1 and nothing else, so its position depends only on container width. Prefer that structure for new list pages; `shrink-0` is the fallback when the CTA genuinely has to stay in a shared row.
