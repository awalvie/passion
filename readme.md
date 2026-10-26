<h1 align="center">Passion</h1>

<p align="center">A training log and planner for climbers that you host yourself.</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/PostgreSQL-18-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL 18">
  <img src="https://img.shields.io/badge/Svelte-5-FF3E00?style=flat-square&logo=svelte&logoColor=white" alt="Svelte 5">
  <img src="https://img.shields.io/badge/licence-PolyForm_Noncommercial-555?style=flat-square" alt="PolyForm Noncommercial">
</p>

<p align="center"><img src="docs/screenshots/laptop-today.png" alt="Today on a laptop: last week, this week and next beside today's session" width="900"></p>

## Plan

Cycles of sessions that repeat each week. A missed day stays on the calendar.

<p align="center">
  <img src="docs/screenshots/phone-today.png" alt="Today on a phone" width="240">
  <img src="docs/screenshots/phone-plan.png" alt="The schedule for last week and this week" width="240">
  <img src="docs/screenshots/phone-cycle.png" alt="A six-week cycle on a calendar" width="240">
</p>

## Train

One step at a time, with last time's numbers beside each set. Hang timers that sound on a
silenced iPhone. Every go on a climb, with sends and flashes worked out. Logs without signal.

<p align="center">
  <img src="docs/screenshots/phone-session.png" alt="A session in progress" width="240">
  <img src="docs/screenshots/phone-timer.png" alt="A hang timer counting down" width="240">
  <img src="docs/screenshots/phone-climbs.png" alt="Logging a climb" width="240">
</p>

## Look back

Hours, climbs and a grade pyramid for 10 weeks, one cycle or any dates. A history for each
exercise.

<p align="center">
  <img src="docs/screenshots/phone-history.png" alt="History with time, climbs and hardest send" width="240">
  <img src="docs/screenshots/phone-progress.png" alt="Progress for climbs" width="240">
</p>
<p align="center">
  <img src="docs/screenshots/phone-past.png" alt="A past session with its session journal and sets" width="240">
  <img src="docs/screenshots/phone-exercise.png" alt="The history of weighted pull-ups" width="240">
</p>

## Journal

Notes on your climbing. A pinned note shows on Today and at the start of each session.

<p align="center">
  <img src="docs/screenshots/phone-journal.png" alt="The Journal with two pinned notes" width="240">
</p>

## Make your own

Build exercises and sessions in the app, or load them from files as a private catalog.
Boulders in Font or V, routes in French or YDS.

## Run it

You need Docker and Postgres 18.

```sh
make image
docker run -e DATABASE_URL=postgres://... -p 8080:8080 passion:dev
```

Open <http://localhost:8080> and sign up. On an iPhone, add it to the Home Screen from
Safari's Share menu, then sign in again there.

From source: install [nix](https://nixos.org), then run `nix develop` and `make run`.

## Docs

- [Deploy](docs/DEPLOY.md): configuration, deploys and backups.
- [Develop](docs/DEVELOPMENT.md): setup, where code goes, and styling.
- [Catalog format](docs/CATALOG_FORMAT.md): exercises and sessions as files.
- API docs: `/api/docs/` on a running server.

## Licence

[PolyForm Noncommercial 1.0.0](LICENSE).
