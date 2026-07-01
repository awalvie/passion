---
name: Known duplicate / near-duplicate exercise pairs
description: The Power Company climbing-kind vs plain reps_and_sets drill overlaps, pending user decision
type: project
---

# Duplicate axis (identified 2026-07, PENDING user approval — do NOT act unprompted)

Two parallel families exist for several Power Company drills:
1. "Power Company: X" — kind `climbing`, has video media, ORPHANED (no template references them).
2. Plain "X" (often `*_drill.yaml` file) — kind `reps_and_sets` with sets/reps/timing,
   REFERENCED by the A/B/C session templates.

Confirmed pairs:
- Power Company: Heavy Feet (heavy_feet.yaml, climbing, orphan) vs Heavy Feet (heavy_feet_drill.yaml, reps, referenced by A: Foundations)
- Power Company: Hip Shapes (hip_shapes.yaml, climbing, orphan) vs Hip Shapes (hip_shapes_drill.yaml, reps, referenced by B)
- Power Company: Sloth Monkey (sloth_monkey.yaml, climbing, orphan) vs Sloth-Monkey (sloth_monkey_drill.yaml, reps, referenced by B)
- Power Company: One Touch (one_touch.yaml, climbing, orphan) vs One-Touch (one_touch_drill.yaml, reps, referenced by A)
- Perfect Repeats (perfect_repeats.yaml, reps, referenced by B) vs Power Company: Pumped Perfect Repeats (pumped_perfect_repeats.yaml, climbing, orphan) — related but distinct (pumped = endurance variant)

The plain reps_and_sets versions are the "keepers" (they're wired into templates and carry
concrete set/rep/timing). The "Power Company: X" climbing-kind versions are richer in prose +
video but unreferenced. Recommendation leaned toward: keep ONE per concept, preferably merging
the video/notes from the climbing version into the referenced reps version, or converting
templates to reference the climbing version. Awaiting user direction.

40 exercises are currently orphaned (not referenced by any template) — most are the full
Power Company: X climbing library + several standalone drills (PE Flow, Sub Limit, Limit
Bouldering Traditional, Tension Activation, etc.). Orphan != cull: many are intentionally
browsable library entries. Flagged for review, not deletion.
