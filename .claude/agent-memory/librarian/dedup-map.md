---
name: Duplicate pairs — RESOLVED + naming conventions applied
description: The Power Company dedup + naming cleanup executed 2026-07-01; records outcome and two open follow-ups
type: project
---

# Duplicate axis — RESOLVED (executed 2026-07-01, user-approved)

The four "Power Company: X" climbing-kind duplicates were MERGED into their referenced
reps_and_sets `*_drill` keepers and the climbing files DELETED:

- heavy_feet.yaml  → folded video+prose into heavy_feet_drill.yaml ("Heavy Feet"), deleted.
- hip_shapes.yaml  → folded into hip_shapes_drill.yaml ("Hip Shapes"), deleted.
- sloth_monkey.yaml→ folded into sloth_monkey_drill.yaml ("Sloth-Monkey"), deleted.
- one_touch.yaml   → folded into one_touch_drill.yaml ("One-Touch"), deleted.

For each: kept the drill's kind/sets/reps/timing/name/source/label; appended the climbing
version's `media:` (YouTube video + thumbnail) and its richer descriptive prose to the drill notes.
Verified nothing referenced the deleted "Power Company: X" names before deleting (they were orphans).

NOTE: "Perfect Repeats" (perfect_repeats.yaml, referenced by B) vs "Pumped Perfect Repeats"
(pumped_perfect_repeats.yaml, orphan) were left as DISTINCT — pumped is the endurance variant,
not a duplicate. Both remain.

# Naming conventions applied (2026-07-01)

1. Lock-offs renamed to Title-Case + degree symbol:
   - "1-arm lock-off 90 degrees"  → "One-Arm Lock-Off 90°"
   - "1-arm lock-off 120 degrees" → "One-Arm Lock-Off 120°"
   Refs updated in limit_bouldring_paradigm_climbing + strength_project_day.

2. Dropped the redundant "Power Company: " name prefix from all 19 remaining prefixed
   exercises (they carry source: "Power Company Climbing"). None were template-referenced,
   so no ref updates were needed. `source:` lines were left intact.
   Collision rule confirmed: the only de-prefix collisions ("Heavy Feet", "Hip Shapes") were
   exactly the pairs deleted in the merge, so they freed up — no live collisions remained.
   CONVENTION GOING FORWARD: exercise `name:` should NOT carry a "Power Company: " prefix;
   attribution lives in `source:` only.

# New reusable activity templates (2026-07-01)

Created in catalog/activity_templates/: Journal, Light Climbing Warmup,
Limit Boulder + Strength Block (source Paradigm), Antagonist & Prehab.
Sessions refactored to reference them where the inlined block matched exactly. See MEMORY.md.

# OPEN FOLLOW-UPS (flagged to user in the execution report)

- **Journal prompts consolidated**: A/B/C each had DIFFERENT session-specific reflection
  questions. A shared activity template can carry only one `notes` value, so the three
  bespoke prompt sets were replaced by one generic prompt. If the user wants per-session
  questions preserved, revert to inline Journal blocks (or make three named Journal templates).
- **A: Foundations warmup left INLINE**: the "Light Climbing Warmup" template = General Warmup
  + Silent Feet, which matched C exactly. A's warmup uses General Warmup + *Heavy Feet*, so it
  did NOT match and was left inline to avoid silently swapping its drill. Only C was refactored.
- **Antagonist & Prehab template created but NOT wired into any session**: upper_body_strength_day
  interleaves those 4 lifts with squats/press/deadlift (not a contiguous matching block), so per
  the "only refactor on exact match" rule it was left as-is. Template is available for future use.

Reminder: 40-ish exercises remain orphaned (browsable library entries) — orphan != cull.
