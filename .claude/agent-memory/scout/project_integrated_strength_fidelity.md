---
name: Integrated Strength — catalog diverges from Bechtel's published templates
description: The repo's integrated_strength_{a,b,c} is a lossy reconstruction; Bechtel's real templates are more varied and contain no full crimp and no core slot
type: project
---

Found 2026-08 while researching the boredom problem on the Strength & Stretching day.
**The catalog's Integrated Strength is not what Bechtel published.** The blandness is partly a
fidelity bug, not a limitation of his system.

Primary source (fetched, verified):
https://www.climbstrong.com/resource-posts/integrated-strength

## What Bechtel actually publishes — THREE templates, verbatim structure
**IS-1 (10-second hangs)** — 3 rounds each:
- C1: Open Hand Hang, 2 arms 10s → Deadlift 3 reps → Frog 60s
- C2: **Pinch Block 3"** 10s/side → One-Arm Kettlebell Press 6+6 → Kettlebell Arm Bar 30s/side
- C3: Half Crimp, 2 arms 10s → **Single Leg Squat** 3+3 → Tug of War Squat 60s

**IS-2 (Webb-Parsons)** — 3 rounds each. **All three circuits are HALF CRIMP; what varies is the
ARM ANGLE, not the grip:**
- C1: Half Crimp, 1 arm, **straight arm** 10s/side → Kettlebell Swing 10 → Frog 60s
- C2: Half Crimp, 1 arm, **bent arm** 10s/side → One-Arm KB Bench Press 6+6 → Lat/Rhomboid roll 60s
- C3: Half Crimp, 1 arm, **lock off** 10s/side → Rear Foot Elevated Split Squat 3+3 → Tug of War Squat

**IS-3 (bodyweight, 3-3-3 hangs)** — 3 rounds each:
- C1: Half Crimp, 2 arms, straight arm 3-3-3 → Single Leg Hip Thrust 10+10 → Prying Cobra 60s
- C2: **Pinch Block 3"** 3-3-3/side → **One-Arm Push-Up** 3+3 → Overhead Squat 60s
- C3: **Pocket Hang, second pair**, 2 arms, straight 3-3-3 → **Pistol Squat** 3+3 → Tug of War Squat

3-3-3 = "hang 3 seconds, let go for 2, hang for 3, let go for 2, hang for 3."
"Each circuit takes about five minutes… each group about fifteen. Including a good warm-up, the
session takes about an hour." 2-3 days/week in a strength phase; 1×/week or per 10 days in season.

## Four concrete divergences in the repo
1. **`hangboard_ladder_full_crimp.yaml` appears in NONE of the three Integrated Strength templates**
   — but **CORRECTION to my first read: it IS Bechtel's own prescription, from a different article.**
   His ladder piece ("How to Get Stronger Fingers For Climbing? Hangboard Ladders," *Climbing*, June
   2016 / online 21 Dec 2022, https://www.climbing.com/skills/hangboard-ladders-for-finger-strength/)
   explicitly prescribes **three positions in a fixed order: open hand → full crimp → half crimp**,
   with the rationale "**Full crimp is trained second to ensure you're warmed up for it. We finish
   with half crimp because it is the strongest position.**" So the repo is faithful to the LADDER
   article and unfaithful to the INTEGRATED STRENGTH templates — it has spliced two different Bechtel
   protocols. Do not claim full crimp is unsourced; claim it is **outsourced from the wrong article
   and contradicted by the evidence**: weakest grade predictor (Faggian r=0.47-0.58 vs slope
   0.67-0.72), Lattice's manual says "NO Full Crimp", Quarmby lists crimp use as an injury risk
   factor, Hooper's Beta says beginners should avoid it. Bechtel's stated reason for putting half
   crimp last — that it "is the strongest position" — is also **directly contradicted by Zihlmann
   et al. 2025 (n=85): open hand produced slightly higher force at every skill level.**
2. **Bechtel's third circuit is always a LEG exercise** (single leg squat / RFE split squat / pistol).
   The repo's group C is "Fingers, **midsection**, splits" with HSLR / body saws. Bechtel has no core
   slot in Integrated Strength at all.
3. **The lift menu was flattened.** Bechtel: kettlebell swing, one-arm KB press, one-arm KB bench
   press, single-leg hip thrust, single-leg squat, pistol squat, RFE split squat, one-arm push-up.
   Repo: "Deadlift or Front Squat / Push-Up or Overhead Press / HSLR or Body Saws." **Unilateral and
   kettlebell work — the interesting half — is missing.** This is most of the boredom.
4. **The hangboard protocol is a hybrid.** Repo uses 3-6-9s. Bechtel's IS templates use 10s holds or
   3-3-3. 3-6-9 ladders ARE a real Bechtel protocol but a DIFFERENT one; the Climb Strong pages
   documenting its load/rest are 404 or member-gated, so **the repo's dose (3 sets × 3 hangs, 60s
   rest, 65-75%) is UNVERIFIED against any published Bechtel number.**
   Also: IS-2 varies **arm angle** (straight / bent / lock-off) at one grip — the repo varies grip at
   one arm angle. Given isometric carryover is only ±10° (Lum & Barbosa 2019), arm-angle variation is
   a legitimately distinct and more interesting stimulus, and bent-arm hang is a top-tier grade
   predictor (r = 0.70-0.80, Faggian).

## His rationale — verified quotes, and which half holds up
- **Hormonal claim (does NOT hold up):** compound lifts are "efficient and effective at producing
  high levels of hormonal activity - **which is half the reason we do them**." Refuted by West et al.
  2009/2010, *J Appl Physiol* 108(1):60-67 — n=12, within-subject, 15 weeks, arm-curl-only vs
  identical curl + leg work to spike systemic hormones: elbow flexor CSA +12% vs +10% (p=0.25),
  strength ~+20% both (p=0.65). Verbatim: "**Transient resistance exercise-induced increases in
  endogenous purportedly anabolic hormones do not enhance muscle strength or hypertrophy following
  15 wk of resistance training.**" https://pmc.ncbi.nlm.nih.gov/articles/PMC2885075
  See also Morton et al. 2016 ("Neither load nor systemic hormones determine…") and Schoenfeld 2013.
- **Time-efficiency / mutual-rest claim (DOES hold up):** "the movements and load are different
  enough that each exercise serves as an effective 'rest' from the other two." Supported by Zhang,
  Weakley, Li, Li & García-Ramos 2025, *Sports Med* 55(4):953-975 — 19 studies, 313 participants:
  agonist-antagonist supersets give **~37% time reduction**, training efficiency SMD 1.74 (p=0.01),
  and **chronic max strength SMD 0.10 (p=0.36) — i.e. no compromise**; same volume load. Caveat:
  **RPE is higher (SMD 0.77, p=0.02)**, and one low-intensity (~55% 1RM) superset study showed no
  max-strength gain — so keep the load meaningful.
  https://pmc.ncbi.nlm.nih.gov/articles/PMC12011898/
  => **Keep the circuit format. Drop the hormonal justification.**
- **No-pulling rationale (verified, half-defensible):** "Why no pulling? Well, because you are
  probably overloading the pattern already. More importantly, though, **the Hip Hinge is going to be
  a big pull, and the finger strength sets also represent a pull…enough!**"
  The "hip hinge is a big pull" part is weak — a deadlift loads neither elbow flexion nor scapular
  retraction. BUT the evidence partly vindicates the conclusion: **1RM weighted pull-up shows no
  significant correlation with climbing ability** (Faggian 2024). If pulling goes in, it should be
  **isometric/lock-off or reps**, not 1RM. Catalog already has Paradigm's One-Arm Lock-Off 90°/120°.
- **Boredom (the irony worth remembering):** Bechtel designed the circuit partly AS a boredom fix —
  "**If you rest a lot between efforts, you get bored and start the next set too soon. The solution?
  Find something productive to do between sets.**" The owner finds the result boring anyway.
- **On variety, he sanctions BETWEEN-cycle rotation:** "the best program is probably to change
  programs every couple of months! **Pick three positions, and train them exclusively through a 4 to
  6 week phase.**" So three grip slots is intentional and cycle-locked — but he never says which
  three, and full crimp is in none of his published examples.
- **Against adding volume:** "Strength comes slowly" — experiment with REDUCED frequency if gains
  continue, rather than adding volume. Directly counsels against the "make it spicier by adding
  more" instinct.

## Catalog gaps confirmed absent (would need authoring)
No pinch block, no hangboard lock-off / bent-arm hang, no kettlebell swing, no one-arm KB press,
no single-leg hip thrust, no pistol squat, no front lever, no Copenhagen, no scapular pull-ups,
no dips, no standalone toe-hook isometric, no standalone wrist work, no ab wheel, no Pallof,
no system board, no weighted bouldering, no lock-off ladder.
Present but easy to miss: **`max_lifts_power.yaml` (Tension Climbing) IS an edge-lift / no-hang
protocol** — 20-30mm block, 80% 1RM, lifts off the ground, 3min rest, with a full 9-week wave.
Nelson's three **wall_crawl** files are already the on-wall finger-strength answer.

## Design implication for Passion
Fixed slots, rotating fills, curated choice — this is already expressible with
`kind: exercise_catalog` + `children:`, which is how groups A/B/C work today. **No new model
needed**, so every recommendation here is template + exercise-YAML work (low/medium), never a
migration. Evidence for the pattern: Sylvester 2016 RCT (adherence 64% vs 51%, volume-matched) plus
Kassiano 2022 (systematic variation fine, random churn costs a little). Whatever the progress metric
is must stay FIXED.
