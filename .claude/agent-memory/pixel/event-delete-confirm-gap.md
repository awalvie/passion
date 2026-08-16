---
name: event-delete-confirm-gap
description: Sibling event-delete button still uses onclick confirm() instead of hx-confirm, missed in the ui-ux-fixes.md batch
type: feedback
---

`docs/plans/ui-ux-fixes.md` item 5 explicitly called for fixing "the sibling event-delete `onclick confirm` → `hx-confirm`" as part of the same batch that added the cycle-delete kebab menu. As of the 2026-08-15 review, `templates/training_cycle_detail.html`'s per-event delete button (inside the `edit-event-dialog-{{ .ID }}` dialog, the form posting to `/calendar-events/{{ .ID }}/delete`) still uses `onclick="return confirm('Delete this event?')"` rather than `hx-confirm` — the cycle-delete action itself was correctly converted (uses `hx-confirm` naming the cycle), but this sibling was not touched.

When reviewing a plan's FINAL DECISIONS block, check every item is actually done, not just the primary one — "also fix X" sub-items are easy to drop.
