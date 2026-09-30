# Research: mobile wrapping, UI screenshots, DB viewing, Docker deploy

Branch `rewrite`. No repo files were changed; the only commands run were reads, a
`WebFetch` against capacitorjs.com, a `firefox --headless` test against a throwaway
page in the scratchpad, and `psql`/`docker` probes against the already-running local
dev Postgres cluster and a public `alpine:3` image.

## 1. Wrapping the client in Capacitor

Verified in source: `client/src/routes/+layout.ts` already sets `ssr = false` and
`prerender = false` with the comment "later, wrapped by Capacitor" — this is planned,
not started. No `@capacitor/*` packages, `capacitor.config.*`, or CORS code exist yet
(`grep -i cors` over `server/` found nothing but doc-site JS). `client/src/lib/api.ts`
does `fetch(path, ...)` with a **relative** path like `/api/v1/tokens`, and
`client/src/lib/session.ts` already keeps the token in `localStorage` for this reason.

**What Capacitor needs:**
- `client/package.json` — add `@capacitor/core`, `@capacitor/cli`, and
  `@capacitor/ios`/`@capacitor/android` (new deps, not in the flake or lockfile today).
- `client/capacitor.config.ts` (new file) — `webDir` pointing at the existing build
  output (`../server/web/dist`, same directory Vite already writes to per
  `client/vite.config.ts`), plus an `appId`/`appName`.
- Verified via capacitorjs.com/docs/config: the WebView origin is **not** the real
  server — default schemes are `capacitor://localhost` on iOS and `https://localhost`
  on Android (`server.androidScheme` default `https`). `server.url` exists but the
  docs say it loads an external URL "for live-reload" and is "not intended for
  production." So the shipped app bundles the static build and runs from a fake
  origin, same as a self-hosted Jellyfin/Immich-style mobile client.

**How the app finds a self-hosted server:** there is no server URL to discover
automatically. The standard pattern (and the only one that fits "self-hosted, any
address"): add a first-run "Server URL" field, store it next to the token (same
`localStorage`/Capacitor Preferences), and change `request()` in `client/src/lib/api.ts`
to build an absolute URL (`${serverUrl}${path}`) instead of a relative one when running
under Capacitor.

**What breaks concretely:** the relative `fetch(path, ...)` in `api.ts` resolves
against `capacitor://localhost`/`https://localhost`, not the configured server, so
every API call 404s inside the wrapped app until that call is made absolute. That is
the one required code change on the client.

**CORS/auth on the Go side — two verified options:**
- *Real CORS*: `server/api/api.go`'s `Routes` uses `http.ServeMux` with exact
  method+path patterns (e.g. `"POST /api/v1/tokens"`); there is no `OPTIONS` handler,
  so a preflight request currently falls through to the catch-all `/api/` 404 handler.
  Adding CORS means middleware that answers `OPTIONS` and sets
  `Access-Control-Allow-Origin` (the Capacitor origins), `-Allow-Headers: Authorization,
  Content-Type`, `-Allow-Methods`. Auth itself needs no change — it's already a bearer
  token in a header, not a cookie, so no `credentials`/`SameSite` complications.
- *Native HTTP plugin*: verified via capacitorjs.com/docs/apis/http — enabling
  `plugins.CapacitorHttp.enabled: true` patches `fetch`/`XMLHttpRequest` to go through
  native HTTP, which "bypasses browser CORS policies" because it isn't a WebView
  request. This needs zero Go-side changes but is a global, invasive patch and doesn't
  help the web build.
Simplest: real CORS middleware — it's a small, standard addition and keeps web and
mobile on the same code path.

## 2. Screenshotting the phone UI without a phone

Verified by running it: `firefox --headless --no-remote -profile <dir> --screenshot=out.png --window-size=390,844 <url>` produced a real 390×844 PNG (`file` confirmed the exact dimensions) against a local `python3 -m http.server` page. `firefox --version` here is 155.0.1. Inline `<script>` in the page DOES execute during the screenshot (verified with a script that mutates the DOM before capture) — SPA rendering works.

It **cannot** pre-seed `localStorage` before that render: `firefox --help` has no flag
for injecting a script, a cookie, or storage ahead of navigation. Since the app's own
routes only run their JS after load, there is no way to hand it a token first without
either (a) the app itself reading a token from somewhere reachable pre-render (it
doesn't), or (b) real automation (WebDriver/CDP) that can call `localStorage.setItem`
before navigating.

**Right now this is moot**: the readme confirms "the client has only the sign-in
screen so far," which needs no auth. So plain `firefox --headless --screenshot` at a
phone size, with zero new dependencies, is sufficient today.

**When authenticated screens exist**, the simplest upgrade is `playwright` — verified
present in the same nixpkgs channel the flake already pins
(`nix eval --raw nixpkgs#playwright-driver.version` → `1.63.0` against
`nixpkgs-unstable`, same input as `flake.nix`). Playwright's
`context.addInitScript()`/`storageState` can set `localStorage` before the first
navigation, and it ships real device-size presets. This is a real new devShell
package, not zero-cost, but it doesn't add a new flake input — same channel.
`nixpkgs#pgweb` and `#adminer` are also present in that channel (used in §3).

Recommendation: keep using bare `firefox --headless --screenshot` for now; add
`playwright` to `flake.nix` only once there's a logged-in screen worth capturing.

## 3. Viewing the dev database

Verified live against the already-running cluster (`pg_ctl -D "$PGDATA" status` showed
it already up — nothing started or stopped here): with direnv active, `PGHOST`,
`PGDATABASE=passion`, and `DATABASE_URL` are already exported by `flake.nix`'s
`shellHook`. Running bare `psql` connected immediately over the Unix socket in
`.pgdata`, `\conninfo` showing `Database: passion`, `Socket Directory: .pgdata`,
trust/peer auth, no password.

`psql` is already in the shell (`pkgs.postgresql_18`) and needs no config — simplest
option, zero new dependency. Recommended target, mirroring the style of `db-up`/`db-down`:

```make
db-shell: db-up
	psql
```

If a GUI is ever wanted, `pgweb` is confirmed available in the same nixpkgs channel
(`nix eval --raw nixpkgs#pgweb.pname` → `pgweb`) and could be added to
`flake.nix`'s `packages` and run as `pgweb --url "$DATABASE_URL"` — but that's a new
devShell dependency for something `psql` already does, so not recommended unless
someone specifically wants browsing over `psql`. Not attempted live (unverified
whether pgweb's driver accepts the `host=<socket-dir>` DSN form `DATABASE_URL` uses,
or needs a TCP host instead). `pgAdmin`/`adminer` both need a running web server
process and, for adminer, PHP — more moving parts than this project uses anywhere
else; not recommended.

## 4. Docker deployment

Read `Dockerfile` in full (52 lines, 3 stages: `node:22-alpine` client build →
`golang:1.26-alpine` server build → `alpine:3` runtime).

**Confirmed by reading `catalog/catalog.go`**: the shipped `catalog/` tree is
`//go:embed`'d into the Go binary at build time (`COPY catalog ./catalog` happens in
the `build` stage, before `go build`). So it does **not** need to exist in the final
image or be mounted — it's already inside the binary. That rules out one plausible
"gotcha" (nothing needs the shipped catalog/ path at runtime).

**What does need mounting**: `cfg.Catalog.Private` (`passion.example.yaml`'s
`catalog.private[].location`) and the config file itself (`PASSION_CONFIG`) are read
from the real filesystem at startup (`server/cmd/passion/main.go` calls
`config.Load(os.Getenv("PASSION_CONFIG"))` and `catalog.LoadAll(...,
cfg.Catalog.Private)`), not embedded. Both must be bind-mounted read-only.

**Confirmed gotchas:**
- No `HEALTHCHECK` in the Dockerfile (grepped for it — absent) despite `/healthz`
  existing (`server/api/api.go`). Alpine's base image ships busybox `wget` by default
  (verified: `docker run --rm alpine:3 which wget` → `/usr/bin/wget`, no `curl`), so a
  healthcheck needs no extra package: `wget -qO- http://localhost:8080/healthz`.
- Runs as non-root already (`adduser -D -u 10001 passion`, `USER passion`) — so any
  bind-mounted config file or private catalog directory must be **readable by uid
  10001** on the host, or the container fails to start.
- No `VOLUME` declared for anything — fine, since Postgres is a separate container in
  this deployment and the app itself is stateless.
- `ENTRYPOINT` is fixed (`["/usr/local/bin/passion"]`), no `CMD`/args — config is
  entirely env vars (`DATABASE_URL`, `PASSION_ADDR`) or a mounted file
  (`PASSION_CONFIG`), matching the readme's Configuration table.

**Suggested `docker-compose.yml` shape** (not written to the repo, per instructions —
described here):
```yaml
services:
  db:
    image: postgres:18
    environment:
      POSTGRES_DB: passion
      POSTGRES_USER: passion
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password   # or an env file
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U passion"]

  app:
    build: .                 # or a pinned image tag from `make image`
    depends_on:
      db:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://passion:...@db/passion
      PASSION_CONFIG: /etc/passion/passion.yaml
    volumes:
      - ./passion.yaml:/etc/passion/passion.yaml:ro
      - ./private-catalogs:/etc/passion/catalogs:ro   # matches catalog.private[].location
    ports:
      - "8080:8080"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]

volumes:
  db-data:
```
(`postgres:18` tag matches the readme's "PostgreSQL 18" badge; not independently
verified against Docker Hub from this sandbox — no network pull of it was attempted,
unlike `alpine:3` which was pulled and checked.)

**Backups**: a `pg_dump` sidecar or host cron calling
`docker compose exec db pg_dump -U passion passion | gzip > backup-$(date +%F).sql.gz`
on a schedule, writing outside the `db-data` volume. The app has no dump/restore
tooling of its own (`server/cmd/` has only `passion` and `catalogid` — verified by
reading the readme's Project structure table and `server/cmd/`).

**Upgrades**: readme confirms "It migrates its own database on the way up, so an
upgrade never needs a second command" (`db.Migrate` runs before `db.Open` in
`main.go`) — so an upgrade is: pull/build the new image, `docker compose up -d app`.
No separate migrate step, no downtime tooling beyond normal container restart.
