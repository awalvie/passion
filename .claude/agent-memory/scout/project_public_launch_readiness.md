---
name: Public launch readiness review (beta → strangers)
description: Ranked launch blockers, catalog boundary pushback, licensed-content exposure, and a four-ship launch sequence
type: project
---

# Consultation 2026-09-03 (RECOMMENDED — owner has not decided)

Owner wants to take Passion out of beta and host publicly. Today `/signup` is open
and every new account imports the *whole* configured catalog, including
`/opt/passion/catalog-private/` (Paradigm / Power Company / Kettle). Live leak.

## 1. Catalog boundary — argued AGAINST copy-on-edit

`docs/SHARED_CATALOG.md` settled on "shared rows at owner 0, copy-on-edit, store the
parent id on the copy". I pushed back. The industry did NOT converge on copy-on-edit.

What Strong / Hevy / JEFIT / Crimpd / Lattice / MacroFactor actually do:

1. The shared tier is **immutable**. You cannot edit a built-in.
2. **Duplicate** is an explicit user action, never a side effect of pressing Edit.
3. Small per-user annotations attach to the shared row *without copying*
   (note, rest timer, favourite, hidden).
4. Real numbers live **downstream** — on the log, the programme instance, the
   calendar entry. Never on the library definition.

That is reference-with-overrides + explicit fork.

### The overlap argument (the decisive one)

`CycleExerciseOverride` / `CycleExerciseWeekOverride` already carry
Sets/Reps/WeightKg/RepSeconds per cycle and per week. So *every numeric field* a
user would edit already has a per-user home. What copy-on-edit uniquely adds is
name, notes and out-of-cycle defaults — all cheaper as one `UserExercisePref`
table (owner_id, library_exercise_id, note, sets, reps, weight_kg, rep_seconds,
hidden), slotting into the existing chain as
week → cycle → **user default** → catalog default.

### The failure copy-on-edit has that annotations do not

A user who holds a copy is **frozen out of catalog improvements forever**. Fix a
wrong dose upstream and every user who once tweaked a note keeps the bad dose
silently. Same trap as the `CatalogEditedAt` bug ("an edit was undone on the next
restart"), but permanent. MyFitnessPal's food database rotted exactly this way.
Also: "prefer the copy, hide the original" is a join on *every* list query, forever.

Steps 1 and 2 of the SHARED_CATALOG plan stand unchanged. Only 3 and 4 shrink.

### Hoisted prerequisite

**Filename-as-row-id** (todo.md has it as a separate job) becomes a hard blocker.
`CycleExerciseOverride.LibraryExerciseID` is an unenforced `*uint`, so a YAML
rename under a shared catalog silently dangles *other users'* overrides rather
than erroring. 215 files already have unique basenames — free now, expensive later.

## 2. Launch blockers, ranked

Tier 0 (before any stranger): licensed-content leak; **backups with one tested
restore**; hard-fail on default JWT secret AND `DemoOwnerID == 0` (together they
mean login-as-the-shared-catalog-owner); explicit HTTP methods on every route
(audit P0 #3 — GET-mutating handlers + CSRF middleware allowing missing Origin);
login rate limiting; password reset; LICENSE + catalog LICENSE + privacy + ToS.

Tier 1: account deletion (as a real transactional cascade), data export, email
verification, JWT revocation. All deferrable **only because invite codes bound the
population** — a manual SSH procedure is a legitimate GDPR answer for 20 users,
not for 2000.

No LICENSE at all today = all rights reserved = self-hosting is not legal, which
contradicts the stated goal. `passion.example.yaml` ships `DevAuthBypass: true`
plus a published JWT secret — a supply chain of insecure self-hosts.

## 3. Licensed content

The exposure is **expression, not doses**. Methods and numbers are not
copyrightable; the coach's prose, cueing, naming, and the *selection and
arrangement* of a programme are. Residual after the private-repo move: git history
(clean `filter-repo` while forks are zero — last cheap moment), the 13 house-style
notes the owner did not write, and any session template still wearing a
programme's shape in new words.

`Final Exam I/II` is the sharpest edge: *invented* names attributed to a named
coach. Misattribution, arguably passing off. Two-line rename.

Industry norm: Crimpd licenses and credits deliberately (Eva López, Lattice);
Lattice owns outright; Strong/Hevy dodge it with generic movements nobody owns.
The public catalog's current `source:` fields (Gresham, Ondra, Tyler Nelson) are
right in form — a name plus your own words about a publicly known method.

Product point that matters more than the legal one: **nobody picks a training app
for its exercise list.** Passion's value is the run player and the cycle builder.
Borrowed content is a strategic weakness because a single email can take it away.
The self-authored `Warm Up` rewrite already beat the programme's version.

## 4. Recommended gate: INVITE CODES

Not a waitlist (same engineering work, but collects strangers' emails *before* you
have a privacy policy, and implies a schedule you don't want). Not open signup.
~40 lines: a code table, a signup field, a mint command. Bounds the population, so
every Tier 1 manual procedure becomes legitimate — and it stops the content leak
tonight while step 2 lands properly.

Four ships: **0** stop the bleeding (invite gate, backups, config hard-fails,
licences, filter-repo) → **1** shared-catalog reads + import split + filename ids,
Duplicate instead of copy-on-edit → **2** account layer (SMTP, reset, rate limit,
method routing, policies, deletion) then hand out codes freely to 20–50 people →
**3** verification, export, token revocation, then drop the invite requirement.

Deliberately waiting: admin UI (sqlite3 until it hurts), Postgres (SQLite +
Litestream is fine to hundreds of users on this write pattern), History metrics,
light-mode ground, and everything on the "Explicitly NOT building" list — public
users will ask for a social feed and the answer stays no.

**Fix the week-override bug (audit §2.4, HIGH CONFIRMED, silently does nothing)
before Ship 2.** It is an advertised cycle-builder feature that no-ops.
