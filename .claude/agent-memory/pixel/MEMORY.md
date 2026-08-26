# Memory Index

- [Disabled button convention](disabled-button-convention.md) — disabled controls use CSS `:disabled{opacity;cursor:not-allowed}` or a Tailwind class toggle, never inline JS opacity writes
- [gc-chip toggle pattern](gc-chip-toggle-pattern.md) — new_cycle_guided.html's hidden-input/label-chip component, now joined by a second unpromoted component (.gc-tags tag input); watch for a second instance/page to justify promoting either into passion.css
- [cycle-actions kebab pattern](cycle-actions-kebab-pattern.md) — training_cycle_detail.html's page-header overflow menu is a deliberate parallel to site-nav-dropdown (mobile-inline rules would break it), not a missed reuse — open question on whether to formalize or fold in
- [event-delete confirm gap](event-delete-confirm-gap.md) — plan item 5's "also fix the sibling event-delete onclick confirm" sub-task was missed; check every sub-item of a FINAL DECISIONS block, not just the headline change
- [Unbounded per-item stacking in fixed cells](unbounded-per-item-stacking-in-fixed-cells.md) — one-bar-per-session in a calendar day cell distorted the whole grid row at 7 sessions; always cap + "+N" for per-item renders inside grid cells
- [Dispatcher facts can go stale mid-session](dispatcher-facts-can-go-stale-midsession.md) — briefed "hard facts" about a file under active iteration can already be wrong by review time; re-derive from the live file/render, don't trust the brief verbatim
