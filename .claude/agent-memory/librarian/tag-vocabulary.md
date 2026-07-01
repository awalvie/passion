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
