<h1 align="center">Passion</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/PostgreSQL-18-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL 18">
  <img src="https://img.shields.io/badge/Svelte-5-FF3E00?style=flat-square&logo=svelte&logoColor=white" alt="Svelte 5">
  <img src="https://img.shields.io/badge/licence-PolyForm_Noncommercial-555?style=flat-square" alt="PolyForm Noncommercial">
</p>

<p align="center">Passion is a training log for climbers.</p>

<p align="center"><img src="docs/screenshots/laptop-today.png" alt="Today on a laptop: last week, this week and next beside today's session" width="900"></p>

## Plan

Plan your training in cycles.

<table>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-cycle.png" alt="A six-week cycle on a calendar" width="360"><br>A cycle</td>
    <td align="center"><img src="docs/screenshots/phone-today.png" alt="Today with this week and the next session" width="360"><br>Today</td>
  </tr>
</table>

## Train

Run a session: hangs, sets and reps, climbs.

<table>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-session.png" alt="A session with its sections" width="360"><br>The session</td>
    <td align="center"><img src="docs/screenshots/phone-sets.png" alt="Sets with last time's reps and weight" width="360"><br>Sets</td>
  </tr>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-timer.png" alt="A hang timer counting down" width="360"><br>Hang timer</td>
    <td align="center"><img src="docs/screenshots/phone-climbs.png" alt="Logging a climb" width="360"><br>Climbs</td>
  </tr>
</table>

## Look back

See your hours, climbs and grades over time.

<table>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-history.png" alt="History with time, climbs and hardest send" width="360"><br>History</td>
    <td align="center"><img src="docs/screenshots/phone-past.png" alt="A past session with its session journal and sets" width="360"><br>A past session</td>
  </tr>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-progress.png" alt="Progress for climbs" width="360"><br>Progress</td>
    <td align="center"><img src="docs/screenshots/phone-exercise.png" alt="The history of weighted pull-ups" width="360"><br>One exercise</td>
  </tr>
</table>

## Journal

Keep a training journal.

<table>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-journal.png" alt="The Journal with two pinned notes" width="360"><br>Journal</td>
    <td align="center"><img src="docs/screenshots/phone-pinned.png" alt="Pinned notes at the start of a session" width="360"><br>Pinned notes</td>
  </tr>
</table>

## Make your own

Use the exercises and sessions that come with the app, or build your own.

<table>
  <tr>
    <td align="center"><img src="docs/screenshots/phone-library.png" alt="The library of sessions" width="360"><br>Library</td>
    <td align="center"><img src="docs/screenshots/phone-template.png" alt="The sections of a session" width="360"><br>A session's sections</td>
  </tr>
</table>

## Run it

You need Docker and Postgres 18.

```sh
make image
docker run -e DATABASE_URL=postgres://... -p 8080:8080 passion:dev
```

Open <http://localhost:8080> and sign up. On an iPhone, add it to the Home Screen from Safari.

From source: install [nix](https://nixos.org), then run `nix develop` and `make run`.

## Docs

- [Deploy](docs/DEPLOY.md)
- [Develop](docs/DEVELOPMENT.md)
- [Catalog format](docs/CATALOG_FORMAT.md)
- API docs at `/api/docs/` on a running server

## Licence

[PolyForm Noncommercial 1.0.0](LICENSE).
