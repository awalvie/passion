---
name: Non-uniform set schemes (ladders, pyramids, drop sets)
description: Where real apps store a prescription that varies within a set, and why climbing apps look different from strength apps
type: reference
---

Verified 2026-09-08. Question: does a 3/6/9 hangboard ladder belong to the exercise or to the workout's use of it?

**get-a-grip (plumbmybumb/get-a-grip, MPL-2.0, Swift/SwiftData — real code, weight highest)**
- No exercise library at all. `GripSpec.swift`: "A grip is a VALUE, not a record. There is no grip library, no folder, no picker of saved grips... A grip lives inline on the set row that uses it, and two sets built months apart with the same three fields are automatically the same trend series because they compute the same `GripSpec.key`."
- The named library row is the ROUTINE (`SessionTemplate`, e.g. "Daily no-hangs"). Set list stored as a `[SetPlan]` JSON blob, not a relation, because "set order IS the routine" (CloudKit to-many is unordered).
- Non-uniformity = ordered set rows + routine-level "rhythm" defaults (`holdSeconds`, `restSeconds`, `setBreakSeconds`, `leadInSeconds`, target % band) with per-set `nil`-means-inherit overrides. Comment: nil prefill is "what makes 'change every rest to 25 s' a single edit".
- Within-SET variation is not representable: `PlanMath.hold(set, in: plan)` resolves once per set, outside the rep loop. Starter routine expresses non-uniformity as six rows (repsPerSide 6,6,2,2,1,1, different grips). Set rows carry their own UUID so byte-identical rows repeat at different positions.
- Per-rep numbers exist only as a DERIVED runtime expansion: `PlanMath.sequence()` -> `[RepSlot]` (setIndex, repIndex, side, grip, holdSeconds, leadInBefore, restAfter, target band). Never stored; the runner freezes it into the log.

**Strength apps agree, no disagreement found:** library row = identity + which parameters are tracked; every varying number lives in per-set rows of the routine, plus a set-TYPE enum on the row (Hevy: warm-up/normal/failure/drop, unique per-set load/rep/duration targets). TrainHeroic library exercise stores "Parameters" (reps, weight, weight %) and video, not a scheme. RP app: myo-reps is a technique you SELECT at use time on a plain exercise.

**Climbing/hangboard apps invert it because their library unit is a protocol, not a movement:** Crimpd's 75+ free / 200+ paid "workouts" are named protocols with configurable hold type and intensity; Beastmaker ships named graded (5A-7C) workouts specifying holds, grip, sets, reps, hang time. So "Hangboard Ladder: Half Crimp" is a normal library item for a protocol library. Nobody puts a ladder in a movement library.

**Third pattern (single sighting):** TrainHeroic lets a coach save an exercise prescription (e.g. 8 sets with per-set targets) for reuse on other exercises/workouts — a scheme object separate from both. Found in a third-party review only, not TrainHeroic docs.

**Not verified:** Crimpd's internal model and whether a protocol's per-hang sequence is user-editable; Lattice; whether RP stores technique on exercise-in-mesocycle; Juggernaut AI's model.
