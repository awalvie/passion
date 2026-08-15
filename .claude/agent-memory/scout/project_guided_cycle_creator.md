---
name: Guided cycle-creator helper
description: Optional RP/Boostcamp-style front door that pre-fills the blank cycle editor — question set, single-smart-form flow, and answer→cycle mapping incl. climbing intensity spacing
type: project
---

Consult (2026-08): owner finds blank-slate cycle creation hard (today: name/start/weeks/weekday→template map only). Wants a guided helper that asks a few questions and PRODUCES an editable draft cycle. Advisory only.

## Reconciliation with prior stance (IMPORTANT)
My earlier `project_cycle_metadata_and_equipment` consult said "no creation wizard, keep new form minimal." This EXTENDS not reverses it — ONLY if the helper is a SECOND OPTIONAL door: blank form survives for power users; helper produces an editable DRAFT that lands in the existing detail editor; never a locked/mandatory plan. Entry = "Help me build one" button BESIDE the blank form on /training-cycles/new.

## App landscape verdict
- Full auto-questionnaire (Juggernaut AI, Sequence) = the agency-loss trap. Avoid.
- Backward-from-event (TrainingPeaks) = steal the date→weeks count-back mechanic only.
- Pick-program + map-to-your-week (RP, Boostcamp, Crimpd) = the model to copy; it's ALREADY Passion's weekday→template shape. RP mesocycle setup (goal, days/week, then edit) is the gold standard.
- Blank editor (Strong) = Passion today, the thing that's hard cold.
Insight: owner needs the RP/Boostcamp FRONT DOOR, not a smarter engine.

## Question set
REQUIRED (these three actually build the cycle):
- Q1 Emphasis — single-select, REUSE Focus enum (strength/endurance/technique/projects/general), default general → cycle.Focus + ranks Q4 templates + nudges default weeks.
- Q2 Timeframe — toggle: N-weeks stepper (default 4) OR target-date picker. Date→ceil(days/7) clamp[3,12]; offer CalendarEvent (competition/trip) on that date (reuse calendar, no new model).
- Q3 Available days — 7 Mon–Sun toggles, live count → the weekday slots (TrainingCycleWeekdayMapping).
- Q4 Which sessions — multiselect of user's SessionTemplates, pre-sorted by Focus/Label match, showing Needs chips, ≥1 → the pool distributed across Q3 days. CLIFF: zero templates ⇒ helper can't work; route to template creation / offer starters (biggest real gap).
OPTIONAL (behind "Add detail" disclosure, all skippable):
- Q5 Recent volume (light/mod/lot) → soft overload warning if days≫volume; maybe lighter opening week. Advisory, never blocks.
- Q6 Venue/equipment (existing ClimbingVenue/Board) → SORT templates by satisfiable Needs + BADGE (never hide) unmet gear. Prior stance: informational, no hard-filter (Fitbod trap).
- Q7 Injuries (text/chip) → seed cycle.Notes + optional injury CalendarEvent + bias spacing.
- Q8 Goal detail (1-line) → cycle.Goal / name.
CUT/FOLD: "weakness" folds into Q1 (it IS the emphasis); "current grade" low-value (grades on ticks) → at most Notes seed; "experience w/ structured training" controls helper verbosity not the cycle → cut or infer from prior cycles.

## Flow
Single smart form, ONE screen: Q1–Q4 stacked, optionals behind one <details> "Add detail" (mirrors cycle-details disclosure). NOT conversational (slow, hides decision set, fights notebook identity). 3-step wizard only as mobile fallback (Emphasis+Timeframe / Days / Sessions), each one decision, persistent "Skip — start blank" escape.
HAND-OFF (critical): Generate does what current POST does (create cycle + mappings + scheduled sessions) then REDIRECT to /training-cycles/{id} — that detail page IS already the editor (drag/move, add/remove, metadata, exercise overrides). Zero new edit surface. Land with a light "Here's a starting plan — drag/add/remove anything" draft banner.

## Output / generation mapping
Name = "{Weeks}-week {Focus} block" or from Goal. Weeks=Q2. Focus=Q1. Goal=Q8. Notes seeded from Q7. StartDate=next Monday (grid Monday-anchored) or today (owner call). CalendarEvent from Q2 date / Q7 injury. Lighter final week (Weeks≥4) = non-blocking rest/deload CalendarEvent over last week (deload = calendar event NOT cycle flag, per prior consult).

### Template-assignment algorithm (the crux)
D sorted chosen days, pool P:
- P==1 → same template all D days (beginner "climb 3x/wk").
- P==D → one/day, ordered by intensity spacing.
- P<D → round-robin, intensity-spaced.
- P>D → weekly-repeating map can't rotate across weeks; TRIM to D with a hint, don't silently drop.
Intensity spacing (climbing-specific, generic apps miss it): classify each template high/med/low via keyword heuristic on Name+Label+Needs+Focus (hang/board/power/max/campus/strength=high; endurance/ARC/aerobic/technique/mobility=low; else med). Rule: never two HIGH (finger-intensive) sessions <~48h apart (real weekday gaps). Project/quality session after a rest day.

## Complexity
Whole helper v1 (form + generate handler + naive round-robin) = MEDIUM, NO new model/migration if no intensity field; reuses create+calendar code. Intensity-aware spacing = v2, MEDIUM added logic, worth-the-cost (finger-load spacing is what makes a climbing plan safe vs generic) but ship second.

## A/B / owner decisions flagged
single-form vs 3-step (mobile); intensity: heuristic vs let-user-tap-hard-days (cheap, accurate, no schema) vs none — lean tap; target date auto-event vs ask; start next-Mon vs today; zero-templates cliff handling; cut experience Q; helper as opt-in button (recommend) not default; P>D trim vs abandon-map.
