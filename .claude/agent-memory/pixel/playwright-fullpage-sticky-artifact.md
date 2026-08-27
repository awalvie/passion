---
name: Playwright fullPage screenshot sticky-element artifact
description: fullPage:true screenshots misplace position:sticky elements (nav bar, sticky action bars) mid-page — verify with a viewport-only screenshot before reporting a layout bug
type: feedback
---

Passion's nav bar and several action bars (`.lib-edit-sticky-bar`, cycle sidebar, etc.) use
`position: sticky`. Playwright's `page.screenshot({ fullPage: true })` on a page taller than the
viewport can render sticky elements at their scroll-triggered position instead of stitching them
correctly, making the nav (or a sticky bar) appear to duplicate/float in the middle of the page.

Seen on /training-log/new and /exercise-library/{id}/edit during the 2026-08-27 mobile audit —
looked exactly like a broken stacking-context bug at first glance. Re-shooting the same page as a
viewport-only (non-fullPage) screenshot at the relevant scroll offset showed correct rendering.

**How to apply**: before reporting any "duplicated nav" / "element floating mid-page" finding from
a fullPage screenshot, re-verify with a non-fullPage screenshot (or grep the CSS for
`position: sticky` on the suspect element). Don't spend review time treating it as a real bug.
