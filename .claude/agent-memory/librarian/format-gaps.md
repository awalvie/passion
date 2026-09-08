---
name: Where the catalog format makes the author write prose
description: Measured 2026-09-07 — the fields that are dead, and the structured facts that ended up inside notes
type: project
---

Counted across both trees (184 library entries, 24 blocks, 17 sessions).

# Fields that are effectively dead
- `weight_kg` — 1 entry of 184 (weighted_pull_ups: 10). Every other load sits in prose.
- `session_duration_seconds` — 1 of the 56 `session`-kind entries (pulse_raiser: 180).
- `rung_seconds` — 3 entries, all `"3,6,9"`, all the Logical Progression hangboard ladders.
- `type:` on a session activity — the importer forbids `ref` + `type` together, so every
  block instance (about 40 of them) has an empty type. Only inline activities carry one.
- `needs:` — 5 of 17 sessions, free text, inconsistent casing. Blocks and exercises cannot
  declare equipment at all, which is the layer where it is actually known.

# Structured facts living in notes
- **Per side** — 33 entries on a wider match ("per side/leg/arm/hand", "each side", "both
  sides", "one-arm", "single-leg"); 29 on the narrow one.
  `reps: 6` plus "6 reps per side" means the runner counts half the work.
- **Load** — 12 entries say "Use NN% of 1RM"; 21 mention 1RM or RPE.
- **Tempo** — 5 entries ("Tempo: 3-1-1-1", "2, 3, 1 Pacing", "Tempo: 2-3-2-0").
- **Menu arity** — CORRECTED 2026-09-08. 21 of the 24 `exercise_catalog` menus state a rule
  in prose; 20 say pick one and 1 says "pick one or more" (`pcc_go_hard` Climbing Method).
  THREE state nothing: `endurance.yaml` "Easy campusing (optional)", `lead.yaml` "Lead Focus",
  `pcc_do_more.yaml` "Endurance Method" — those need a human decision, not a sed.
  The earlier note claimed a "choose a couple" menu. Wrong: that phrase is in
  `paradigm/muscular_activation.yaml`, which is `kind: session`, a plain movement, not a menu.
  There is no pick-2 menu in either tree. The error reached `docs/V2_PLAN_REVIEW.md` §3.1.
- **A menu written as a movement** — `power_company/high_pressure_sends.yaml` is
  `kind: climbing` and its notes say "Choose one of these formats", then describe two
  protocols that each exist as their own file (`three_strike_repeat`, `ten_minute_takedown`).
  So the true menu count is 24 by `kind:` and at least 25 in intent.
- **Optional** — encoded as a name suffix in 4 menus: "Wall Crawls (optional)".
- **Prerequisite / gating** — max_hangs "Who this is for", campus_punks "Prerequisites",
  increase_pace "Gated: high F6s onsight".
- **Cycle-scoped progression** — max_lifts_power carries a 9-week sets x reps table in
  prose; the hangboard ladders say "same hold and load for the whole 4-week cycle".

# Dead YAML in the tree
`passion-private-catalog/session_templates/a_foundations.yaml` lines 9-13 give the
`heavy_feet` ref four numeric overrides. The importer copies every field from the library
entry when `ref` is set and never reads them. Validation only rejects `ref` + `name`.

# Repetition that points at a missing abstraction
- Menus cannot be library entries (only block/session exercise items carry `children`).
  11 of 24 blocks are a wrapper around exactly one menu.
- `movement_practice_board` / `_endurance` / `_lead` differ by one prose line and a
  partly overlapping child list. Five `kettle_*` blocks share one notes skeleton.
- `finger_warmup_pulls` is a block wrapping one ref — a pure alias.

# Retirement has no marker
`catalog/session_templates/strength_base_session.yaml` was deleted in e527218 (2026-09-03)
and exists nowhere. The reason lives only in the commit message. The importer keeps the row
alive when history points at it, so deletion silently produces an unmanaged zombie.
