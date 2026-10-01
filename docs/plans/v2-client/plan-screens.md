# Plan screens

The client cannot make a plan yet. Today shows only the days that already exist on the server. These screens let a person schedule sessions and build cycles from the phone.

## What the server gives

Checked in `server/api/cycle.go`, `server/api/scheduled_session.go` and `server/db/cycle.go`.

- `GET /api/v1/scheduled-sessions?from&to`: days in date order, with `template_name`, `cycle`, `run` and `status` (done, started, missed, planned).
- `POST /api/v1/scheduled-sessions` `{template, local_date}`: a one-off. A day holds each session once (422 on `local_date`).
- `PUT /api/v1/scheduled-sessions/{id}` `{local_date}`: move one day. Other days stay.
- `DELETE /api/v1/scheduled-sessions/{id}`: a run started from it stays.
- `GET /api/v1/cycles`, `GET /api/v1/cycles/{id}`.
- `PUT /api/v1/cycles/{id}` `{name, starts, ends, block_days, days: [{day, template}]}`: create or replace under an id the client picks. It places sessions again from today on, when the dates, the block or the days change. A removed or moved day then comes back. It answers `left_out`, the days that already held that session.
- `DELETE /api/v1/cycles/{id}`: removes the cycle and the days it placed. Runs and hand-placed days stay.

A block day can hold more than one session. A cycle is at most a year long. `block_days` must be no longer than the cycle. A cycle can name a retired template, which the template list leaves out.

## Screens

A fourth tab, Plan, between Today and Library. Icon: calendar.

`/plan`
- The days from a week ago to four weeks ahead, as an agenda. A heading per date ("Wed 7 Oct"). A row per session: the session name, the cycle name or "One-off", and the status (Done, Started, Missed).
- A date with no session shows nothing.
- A tap on a row opens it in place (`<details>`):
  - A date field and Move.
  - Remove, with a confirm. For a cycle's session, the confirm says that it comes back if the cycle's dates, block or days change.
- Start stays on Today.
- "Add a session" opens a form in place: a session `<select class="input">` and a date field, today by default.
- Cycles: one row per cycle with its name and dates. A tap opens the cycle. "New cycle" under the list.
- With no signal, the page says that the plan needs a signal.

`/plan/cycles/new` and `/plan/cycles/[id]`: one form for both.
- Name.
- Starts and Ends as date fields.
- Repeats every N days: 7 by default, at most 28, and at most the cycle's length.
- One row per block day: "Day 1 · Wed", where the weekday is the first time that day falls. Each row lists its sessions with a remove button, and has a `<select class="input">` to add one. A template missing from the list shows as "Retired session".
- Lowering N drops the sessions on days past it.
- Save sends the PUT. The form makes the cycle's id once, when it opens, so a retry does not make a second cycle.
- If `left_out` is empty, Save goes to `/plan`. Otherwise the form stays open and says "3 days already held that session, so the cycle left them as they were.", with a link back to the plan.
- The edit page has a Delete button (danger) with a `confirm()` that says the runs stay.
- New cycle defaults: today to four weeks later, block 7, no sessions.
- Each field shows the server's 422 message under it.

No goals, focus, labels, deload or rest periods. The V2 API has none of them.

## Code

- `lib/plan.ts`: the `Cycle` type, and `loadToday()`, which reads `/accounts/me` and returns `today`. Today's `+page.ts` and the Plan page both use it.
- `lib/dates.ts`: `addDays(date, n)` and `weekday(date)`. Both work in UTC on `YYYY-MM-DD` strings, so the device's time zone cannot change the day. They are apart from `plan.ts`, because the node tests cannot import `api.ts`.
- `lib/dates.test.ts`: tests for both.
- `routes/(app)/(tabs)/plan/+page.ts`: loads the days, the cycles and the templates in parallel.
- `routes/(app)/(tabs)/plan/+page.svelte`.
- `routes/(app)/(tabs)/plan/cycles/new/+page.ts` and `[id]/+page.ts`, and `lib/CycleForm.svelte`, which both pages use.
- `TabBar.svelte`: the Plan tab. `Icon.svelte`: a calendar icon.
- Each change calls the API, then `invalidateAll()`. No queue: the plan is made at home, with a signal.

## Commits

1. `plan: add date helpers` (`addDays`, `weekday`, `loadToday`, and the tests)
2. `plan: show the coming weeks in a plan tab` (rows without actions)
3. `plan: schedule a one-off session`
4. `plan: move a session to another day`
5. `plan: take a session off its day`
6. `plan: list cycles` (no links yet)
7. `plan: plan a new cycle`
8. `plan: edit a cycle`
9. `plan: delete a cycle`
10. `readme: describe the plan tab`

## Later

- The week strip and the cycle name on Today.
- A month grid.
- More than four weeks ahead.
