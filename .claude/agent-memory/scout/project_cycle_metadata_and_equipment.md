---
name: Cycle metadata + equipment/venue availability
description: Which mesocycle fields to add (reuse Label/Focus vocab), and informational-first equipment "needs" over a Fitbod matrix
type: project
---

Consult (2026-08): owner wants to enrich TrainingCycle (today just name/start/weeks/weekday→template map + per-cycle/week exercise overrides) with notes/focus/tag "+ suggest more", and a way to declare available equipment/gyms and reference availability when building sessions.

## Key grounding — reuse existing vocabularies, don't invent
- `Label` (comma-separated chips) ALREADY exists on SessionTemplate/ActivityTemplate/LibraryExercise. The owner's "tag" = add `Label` to TrainingCycle verbatim. Zero new concept.
- `SessionJournal.Focus` is an existing ENUM: strength|endurance|technique|projects|general. The cycle "focus" should REUSE this vocab (single-select, optional) → creates a plan-vs-reality loop (cycle focus vs logged journal focus). Climbing-native already.
- `Source` (program/coach) exists on templates — optional add to cycle only if owner follows named external programs.
- CalendarEvent already models trip/injury/rest-or-deload/competition/other with Blocks flag + date range, rendered inline on the cycle grid. So target EVENT/DATE and DELOAD WEEK are ALREADY covered — do NOT duplicate them as cycle fields.
- ClimbingVenue (commercial|outdoor+location) and ClimbingBoard (kilter/moon/tension/spray/custom) already exist per-user; referenced by SessionJournal + ClimbingExerciseMeta. The "what I own" registry EXISTS.

## Cycle fields — verdict
ADD: (1) Notes markdown auto-grow textarea — the catch-all that absorbs ambiguous stuff, "program as little as you want". (2) Focus single-select reusing Journal enum, optional. (3) Tag = Label chips (existing pattern). (4) Goal one-line text (the aspiration, distinct from dated calendar events).
CONSIDER (progressive/later): Phase/block-type select (base/build/peak/deload) — overlaps Focus, coach-facing; only if owner truly periodizes in named blocks. Source — only for external programs.
DON'T: deload-week flag (per-week, not per-cycle; calendar rest/deload event + week overrides cover it); target event/date (already CalendarEvent competition kind); structured target-grade (grades live on ticks → put in Goal text); intensity/volume emphasis categorical (clutter; Notes covers); weekly load targets TSS/hours/sets-per-group (wrong altitude — exercise targets already express volume); separate Description field (one Notes suffices); Color (no surface); any required field or creation wizard.
PLACEMENT: metadata lives in a collapsed `<details>` "Cycle details" on the DETAIL page, autosave-on-change (mirror override-save). Keep NEW-cycle form minimal (name/start/weeks/weekday map). Enrich later, never at creation. Complexity MEDIUM: 3-4 additive columns + 1 autosave handler + 1 details block; reuses Label/Focus/autosave patterns.

## FULL CANDIDATE CATALOG (2026-08 follow-up — owner wanted the complete menu, not the shortlist)
Landscape scanned: Lattice/Crimpd/Steep/TrainingBeta (climbing); RP/JAI/Boostcamp/Strong/Hevy (strength); TrainingPeaks/TrainerRoad/Intervals.icu (endurance); Notion/Linear (generic).
TIER 1 ADD (cheap, ethos-fit, reuse existing vocab): Notes (markdown, catch-all); Focus (reuse SessionJournal.Focus enum → plan-vs-reality loop); Tag (=Label chips); Goal (1-line aspiration, distinct from dated CalendarEvent).
TIER 2 OPTIONAL/progressive: Phase/block-type (overlaps Focus; top periodizer field); Review/retro (reuse Journal WentWell/NextFocus shape — closes plan→do→review, the highest-value post-Tier-1 add); Outcome/result+1-5 rating (pairs w/ Goal); Status/visibility (Archived declutters list; "active" derivable from dates); Priority A/B/C (only if overlapping cycles); Week-by-week per-week label/note (partially EXISTS via CycleExerciseWeekOverride + "varies by week"); Progression scheme (only if it drives auto-gen — no engine yet); Deload policy cadence; Training-days/rest policy (weekday map already implies); Source (exists on templates); Success criteria (fold into Goal); cycle-level equipment/venue needs (better per-session); Save-as-template (clone action not field); Target RPE/readiness (pairs w/ Journal RPE/Energy/Sleep, comparison baseline only); Color/icon.
TIER 3 SKIP + why: target event/date (=CalendarEvent competition); deload-week flag (per-week; calendar+overrides); structured target-grade (ticks; use Goal text); intensity/volume emphasis categorical (Notes); weekly load targets TSS/CTL/hours/sets-per-muscle (wrong altitude; exercise targets); finger-load/TUT ceiling (no infra); MEV/MAV/MRV landmarks (whole subsystem); HRV/sleep/readiness auto-targets (no device integration); 1RM/e1RM %-loading (weights are absolute kg, no e1RM); periodization-model selector (no engine to consume); diet/nutrition phase + bodyweight target (no bodyweight tracking); coach/athlete assign (solo tool); explicit end date (derived); separate short Description (Notes first line).
Headline held: Tier-1 four remain the right first build; if adding one more, Review+Outcome pairing (reuses journal reflection) beats everything; Phase is the top periodization add but overlaps Focus.

## Equipment/venue availability — verdict
Owner conflates two jobs: (A) registry of what I have (exists: venues+boards), (B) requirements tagging + filter/match (the tagging-chore risk).
App landscape: Strong/Hevy = no equipment model (exercise has equipment attr, filter only). Fitbod = gym-profiles + big equipment checkbox matrix + auto-filter (rich but chore, over-serves multi-venue). Crimpd/Lattice = NAME required gear informationally ("you'll need 20mm edge + board"), don't filter. Board apps = board IS the venue.
DO NOW (LOW): optional freeform "Needs/equipment" chip line on SessionTemplate (reuse Label pattern), shown on dashboard card + run header. Answers "what does this session assume / can I do it here" at a glance, zero matching infra, no chore (type once per template). This is the Crimpd/Lattice pattern.
Also: "filter exercises by what's available" is approximable TODAY by filtering the picker on LibraryExercise.Label chips — no new field.
CONSIDER LATER (MEDIUM-HIGH, only if informational chips prove insufficient): small climbing-first capability vocab (~8-10: hangboard, campus, system/spray board, kilter/moon/tension, lead wall, auto-belay, weights, rings/TRX) attached to ClimbingVenue (tag your 1-3 venues ONCE, not per session) + optional session "needs" from same vocab + subset-match to BADGE "doable here". Tag the few venues, not every exercise.
DON'T: Fitbod equipment matrix; mandatory per-exercise equipment tags (rot); auto-HIDE non-doable exercises (kills agency — matches owner's "doing over logging" + the Fitbod "automated away agency" trap) — badge/dim instead; a third parallel Equipment registry separate from Venue/Board (fold capability onto Venue you already have; reference existing ClimbingBoard vocab for boards).
