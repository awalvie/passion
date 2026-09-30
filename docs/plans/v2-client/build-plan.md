# Client build plan

The plan to take the V2 client from sign-in to a Home Screen app on iPhone, with the API wired. The look is restyled later from `passion-design/final/brief/direction.md`. The new markup uses tokens, so that restyle only rewrites `tokens.css`.

## Facts that shape the plan

Checked against the server code:

- `PUT /runs/{id}` replaces the run and clears any field left out (`server/api/run.go:506`). It needs `started_at` and `local_date`. The client keeps the whole run as the server sent it, including journal, place, notes and `started_at` as the raw string, and sends all of it back.
- The server marks a step `done` when its first set or climb is written (`server/db/run_set.go:100`, `server/db/climb.go:234`). So `status` cannot say "the person moved on".
- A step written as `skipped` loses its sets, and a skipped step takes no sets (`server/db/run_set.go:75,121`).
- `POST /runs` is not idempotent. A retried start makes a second run.
- `PUT .../sets` and `PUT .../climbs/{id}` are idempotent. A `DELETE` of a climb the server never got answers 404.
- Sets have no side field.
- The server sets `local_date` on start from the account's time zone. The client does not send it.
- Go has no MIME type for `.webmanifest`, so the manifest is `manifest.json`. The SPA fallback serves deep links (`server/web/web.go:36`).
- iOS Home Screen apps have their own `localStorage`, so the person signs in again inside the installed app.

## Architecture

### Routes

All routes except login and signup sit under `(app)`, which has the sign-in guard. Tabbed screens sit in `(app)/(tabs)`, which changes no URL.

| URL | Screen | Chrome |
|---|---|---|
| `/login`, `/signup` | as today | none |
| `/` | Today | tabs |
| `/templates`, `/templates/[id]`, `/templates/new`, `/templates/[id]/edit` | Library, sessions; `[id]` gets Start | tabs |
| `/exercises`, `/exercises/[id]`, `/exercises/new` | Library, exercises | tabs |
| `/history`, `/history/[id]` | History list and one finished run | tabs |
| `/settings` | Theme, sound, sign out | tabs |
| `/run/[id]` | Session: overview, progress, Continue, ••• menu (finish, discard) | nav bar |
| `/run/[id]/step/[step]` | Player; the step's kind picks Sets, Sides, Timer, Open or Climbs | nav bar "< Session" |
| `/run/[id]/finish` | Journal, then finish | nav bar |

- Tab bar: Today, Library, History. Library is active on `/templates*` and `/exercises*`. Settings opens from Today's nav bar.
- The live bar shows above the tab bar while a run is unfinished.
- The phone column is at most 430 px wide and centred on a desktop.

### Styles

- `client/src/tokens.css` holds new tokens: `--ground`, `--surface`, `--line`, `--ink`, `--ink-2`, `--tint`, `--live`, `--prep`, `--hang`, `--rest`, `--bad`. None of these names exists in `passion.css`. At first each one points at a V1 variable (for example `--ground: var(--bg)`), so both themes work.
- `app.css` maps them with `@theme inline { --color-ground: var(--ground); ... }`. No token uses the `--spacing` or `--radius-*` names, which Tailwind reads.
- New markup uses only these token utilities, written as literal class names, never `.card`, `.btn` or `.input`.
- Text inputs are at least 16 px, so iOS does not zoom.

### Lib modules

- `lib/id.ts`: a uuid v4 from `crypto.getRandomValues`, so it also works over plain http on a LAN.
- `lib/run.ts`: run types and pure helpers.
- `lib/plan.ts`: `ScheduledDay` and `localToday` in the account's time zone.
- `lib/sync.ts`: the write queue.
- `lib/runState.svelte.ts`: the open run as `$state`. Each method changes it locally, then queues the write. It uses `$state.snapshot` before it queues or stores anything.
- `lib/timeline.ts`: the timer engine, a pure function with a `node --test` test.
- `lib/timerStore.ts`, `lib/audio.ts`, `lib/wakeLock.ts`, `lib/grades.ts`.

### Where the person is

- A step is finished when its `elapsed_seconds` is set or its status is `skipped`. The client writes `elapsed_seconds` in a body `PUT` when the person moves on: the last set is logged, they tap Done, or they skip.
- "Where I am" is the first step that is not finished, unless the URL names a step.
- The session clock is `now - started_at`.

### What each kind logs

| Kind | Set row |
|---|---|
| `reps_and_sets` | `{reps, weight_kg}` |
| `timed_reps` | `{reps: reps done, seconds: rep_seconds, weight_kg}` |
| `open` | `{seconds}` |
| per side | two rows per set: odd numbers left, even numbers right; progress counts rows / 2 |
| `climbing` | climbs, not sets |

Prefill comes from the newest entry of `GET /exercises/{id}/history`.

### Timers

Timer state lives in localStorage under `passion-timer:<runId>`: `{ step, anchor, pausedAt|null, offsetMs, rest: {endsAt}|null }`.

`timeline(step)` lists the phases: prep, then for each set and rep a hang and a rep rest (none after the last rep), a set rest between sets, then done. The current phase comes from the clock with pauses taken out, so sleep, lock and reload lose nothing. "End rep" and "Skip rest" move the offset to the end of the phase. "End set" moves it to the next set. "End exercise" logs what is done and finishes the step. A 250 ms interval only redraws and plays tones. It runs inside the component.

### Writes

- Log set sends the step's whole set list with `PUT /runs/{id}/steps/{step}/sets`. A climb is `PUT /runs/{id}/climbs/{uuid}`. A body change is `PUT /runs/{id}` with the whole run.
- `sync.ts` keeps pending writes keyed by method and URL, and a newer write replaces an older one. It mirrors them to `passion-pending:<runId>`.
- It sends one request at a time, the body write first, each with a 10 s timeout.
- It retries on `online`, on `visibilitychange`, after the next success, and on Retry.
- A 404 on a DELETE counts as done. Another 404 or a 422 drops the write and shows its message. A 401 keeps the queue.
- A "Not saved. Retry" strip shows while writes wait. Finish waits for the queue.
- The start is never queued. Its button is disabled while it runs.
- Sign-out clears `passion-timer:*` and `passion-pending:*`.

### Audio and wake lock

- One `AudioContext`, unlocked by the Start tap and resumed on the next `pointerdown` and on `visibilitychange`. It plays a tick in each of the last 3 seconds of a phase, one tone when a hang starts, and two tones when a rest ends. It sets `navigator.audioSession.type = 'playback'` where that exists.
- The wake lock is requested while a player screen is mounted, and again on `visibilitychange`. Failures are ignored.

## Commits

Each commit passes `pnpm --dir client check` with no new warnings, `pnpm --dir client build`, and a check in the browser against a local server (`make db-up`, `go run ./server/cmd/passion`).

1. `client: add design tokens mapped onto v1`
2. `client: add home screen web app meta` (`viewport-fit=cover`, status bar, safe areas)
3. `client: add manifest and app icons` (PNGs made with `rsvg-convert` from `lib/assets/logo.svg`)
4. `client: add run and schedule types`
5. `client: add screen, nav bar and button parts`
6. `settings: add theme and sign out screen`
7. `client: add a tab bar in place of the header` (moves the tabbed routes into `(tabs)`, drops Header and the wide container)
8. `session: show a run's sections and progress` (`runState` load, "where I am", Continue)
9. `today: show today's scheduled sessions` (deletes the redirect; one card per session; Start, or Resume when `status` is started, or Done)
10. `today: start or resume a scheduled session`
11. `library: start a session from a template`
12. `today: start an open session`
13. `client: show a live session above the tab bar` (`GET /runs`, the newest unfinished run)
14. `session: discard a session`
15. `player: log sets for a step` (adds `sync.ts` and the step route)
16. `player: rest between sets`
17. `player: finish a step and move on` (Done, skip the sets left, skip exercise, what comes next)
18. `player: add a set and a note`
19. `player: prefill sets from last time`
20. `player: log per-side sets`
21. `player: run timed reps with prep, hang and rest` (with `timeline.test.ts`)
22. `player: play a tone at phase changes`
23. `player: keep the screen awake`
24. `player: run an open timed block`
25. `player: log climbs`
26. `player: edit and remove a logged climb`
27. `session: add an exercise` (needed for an open session)
28. `session: finish with the journal`
29. `history: list finished sessions`
30. `history: show one session`
31. `docs: describe the tabs group and tokens`
32. `readme: add iphone install steps`

### Later, only once 1–32 work

In this order: choose from a choice, plan screens (list, one-off, cycles), remove, move and swap exercises, reopen a done step, hang settings from the timer, week strip, cycle and length on Today, stats, exercise history, grade scales.

An unpicked choice is harmless for now: nothing logs against it, and finish ignores it. The session screen shows it as "Choice, not picked".

## Risks

- Per-side sets use odd rows for left and even rows for right. This needs the owner's yes, or a side column on the server.
- History cannot show "written up later" without a server flag.
- Web Audio needs a tap first, and the silent switch mutes it unless `audioSession` is set. There is no vibration on iOS. The wake lock in a Home Screen app is unreliable on older iOS (unchecked which version fixes it).
- Two devices on one run: the last write wins, and a stale `PUT` can undo the other device's changes.
- A missing asset gets the HTML shell with a 200 from the server, so a wrong icon path fails quietly.
