---
name: Catalog tag & source vocabulary
description: The controlled label vocabulary and normalized source names for catalog YAML
type: project
---

# Label vocabulary (controlled, reuse — do NOT invent near-synonyms)

Every exercise/template `label:` is 2-4 comma-separated lowercase tags drawn from this set.
Established during the 2026-07 catalog-wide classification pass.

**Movement / quality**
- `technique` — general movement skill / precision drills
- `footwork` — foot-specific (silent feet, heel hooks, hovers, switches)
- `balance` — single-leg, flagging, body-positioning stability
- `power` — max recruitment, limit bouldering, campus
- `dynamic` — deadpoints, momentum, sloth-monkey fast side
- `endurance` — pump management, PE intervals, ARC, repeats
- `fingers` — finger-strength / hang-specific
- `strength` — general strength (pulls, presses, lifts)
- `mobility` — stretching, splits, hip openers, joint prep
- `tension` — body-tension / core-tension drills
- `antagonist` — push/shoulder/prehab balance work
- `tactics` — reading, projecting, pressure, flash/onsight practice
- `core` — trunk-specific (body saws)

**Context**
- `warmup`, `cooldown`
- `boulder`, `route`, `board` (systems board), `hangboard`

Rules: additive-only when re-classifying. MERGE with existing label (union), never clobber.
Keep to 2-4 tags. Prefer reusing an existing tag over minting a synonym.

# Normalized source names

Infer `source:` from notes ("Source: …" lines) or a "Power Company: …" name prefix.
Normalized values in use (count as of 2026-07):
- "Power Company Climbing" (40) — all "Power Company: X" exercises + PCC session templates + the plain `*_drill` reps_and_sets variants that cite Kris Hampton
- "Self-Coached Climber" (2) — Hover Hands, Plan-Climb-Review
- "Neil Gresham" (2) — Silent Feet, Straight Arms + Relaxed Grip
- "Tension Climbing" (1) — Max Lifts (power)
- "Paradigm Climbing" (1) — Limit Bouldering session template
- "Emil Abrahamsson" (1) — Emil's Sub-max Daily Fingerboard
- "Climbing Doctor" (1) — Grip Tipping Point
- "Catalyst Climbing" (1) — Deadpoint / Clap Drill

If a note lists multiple sources (e.g. "Self-Coached Climber, Ondra method, Climbing.com"),
pick the primary coaching program. If truly unknown, OMIT source (do not write empty string).

# Files where source was deliberately left ABSENT
Custom/multi-source items: the A/B/C technique curriculum session templates, the strength_*
session templates, the generic warmup activity templates (Warm Up, Fascia, Synovial, Drills),
and generic strength/mobility exercises (bench press, rows, splits, etc.).

# 2026-07-01 cleanup deltas
- "Power Company Climbing" count dropped by 4: the four "Power Company: X" climbing duplicates
  (Heavy Feet, Hip Shapes, Sloth Monkey, One Touch) were deleted after merging into their drills.
  The 19 remaining Power-Company exercises kept source but dropped the name prefix.
- New activity templates + their source/label choices:
  - "Journal"                         → label "cooldown, tactics", no source (custom).
  - "Light Climbing Warmup"           → label "warmup, footwork", no source (generic warmup).
  - "Limit Boulder + Strength Block"  → source "Paradigm Climbing", label "boulder, power, strength".
  - "Antagonist & Prehab"             → label "antagonist, strength", no source (generic prehab).
  "Paradigm Climbing" is now used as an activity-template source too (was session-template only).

# 2026-09-07 recount (both trees, 184 library entries)

Source counts now: John Kettle (41), Power Company Climbing (36), Paradigm Climbing (28),
Logical Progression (6), Neil Gresham (5), Self-Coached Climber (2), and one each for
Tyler Nelson, Tension Climbing, Emil Abrahamsson, Climbing Doctor, Catalyst Climbing,
Adam Ondra. 12 distinct values — small enough to treat as a controlled list.

"John Kettle" and "Logical Progression" are new since the 2026-07 pass. Note the folder
names are coaches and `source:` is the programme: `bechtel/` holds source
"Logical Progression", `nelson/` holds source "Tyler Nelson".

Label coverage is 100%. Nineteen tags carry almost everything; then a tail of
near-singletons that prove free text drifts: `shoulder`(2) AND `shoulders`(1),
`prehab`(2) beside `antagonist`(12), `lead`(1) beside `route`(16), `stretching`(1)
beside `mobility`(48), plus one-offs `squat`, `hinge`, `pressing`, `mental`, `rest`.
Do not mint any more of these. Body-part tags total 8 entries in the whole catalog —
climbers do not browse by muscle, so do not build that axis out.

The tags also mix three axes in one list: quality trained (power, endurance, fingers),
body part (core, shoulder, hinge), and session context (warmup 36, cooldown 12,
boulder 53, route 16, board 11, hangboard 15). The context tags are the ones that behave
like a real facet.
