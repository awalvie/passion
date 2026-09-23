---
name: Person / account model for the v2 rewrite
description: How comparable training apps model the person, and what that means for Passion v2
type: project
---

# How other apps model the person (2026-09-18)

The owner started V2 from an empty database and asked how other apps model "the person". I
was told not to read V1, so none of its design leaked in. The owner designs the schema. I only
gave constraints.

## What I checked

- **Crimpd.** The profile is gender, age, weight and height. Climbing settings are max sport
  grade, max boulder grade and grading system. You can "view and edit your historical weight,
  height, and grade data", so these are dated entries, not columns.
  (crimpd.com/docs/getting-started)
- **Hevy.** A separate log of dated measurements: weight, body fat, circumferences. The free
  tier gets weight and waist only. Units are a global setting plus a per-exercise KG/LBS
  override, added because one global toggle was wrong for real users.
- **MacroFactor.** Stores raw scale weight by date and works out a trend weight: a rolling
  weighted average with the gaps filled in. Everything downstream uses the trend, never the
  raw point.
- **Lattice.** Finger strength is % of bodyweight on a 20 mm edge. A hang result means nothing
  without the bodyweight on the day.
- **Kilter.** One global grade scale, Font or V, default Font. One is enough because Kilter is
  boulders only.
- **Strava.** Keeps a timezone on the account and falls back to it for activities with no GPS,
  so every indoor session. Its API field `start_date_local` holds local time but is labelled
  `Z`, a long-known trap. Support says the only fix for a wrong date is to delete and
  re-upload: a schema choice turned into a permanent scar.
- **Strong and Hevy.** Both log fully offline and sync later. Hevy lets you stay signed in on
  two devices.

## Constraints I gave

1. Split by how often things change, not by topic. Login, slow profile facts, preferences and
   measurements live four different lives. Measurements and bests are never columns on the
   person.
2. Store date of birth, not age. Store span and height, and work out the ape index.
3. Never store a best (max hang, max pull-ups, max grade) as a profile column. It's a query
   over a test log, and it depends on edge, grip, arm, angle and bodyweight. The owner's own
   Tindeq protocol (peak kg and %BW, half crimp and drag apart, weeks 1 and 4) can't fit in a
   column.
4. Snapshot anything a past number was worked out from, on the past row. Bodyweight for %BW
   in climbing. FTP on TrainingPeaks and Garmin, where users complain that changing it today
   rewrites last year.
5. Store one unit (kg), named in the column. Convert only on the way in and out. A unit column
   per row breaks every total and sort.
6. Hangboard load is signed. Band-assisted hangs are negative added weight, and that's where
   the owner starts.
7. Grades take two settings: boulder scale and route scale. Font boulders with French routes
   is normal in Europe. Store the grade as logged, plus a number for sorting and pyramids.
   Converting loses detail, so never overwrite. A board grade needs the angle beside it.
8. Timezone is an IANA name on the person, never an offset. On every event store the UTC time
   and the zone, and copy the local date onto the row. The local date is part of the record.
   Otherwise a session logged abroad moves a day when you get home, and a streak breaks by
   itself.
9. Several devices: ids the client can make, `created_at` and `updated_at` on everything
   including the profile, soft delete, and a table of sign-ins rather than one token column on
   the user. One token column means the laptop logs the phone out.
10. The profile is almost all optional. You must be able to log a session with nothing but an
    account. Don't gate logging behind onboarding.

## Mistakes to avoid

- Age instead of date of birth. Gender required, and quietly feeding an algorithm.
- A self-reported "max grade" that nothing updates from the logs. Work it out from ticks.
- The profile as a dump for inputs that change over time.
- No way to fix a past entry short of deleting the session.
