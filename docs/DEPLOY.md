# Deploy

Passion is one program with a Postgres 18 database. It migrates the database each time it
starts.

- [Docker](#docker)
- [Binary](#binary)
- [From source](#from-source)

## Docker

```sh
curl -LO https://github.com/awalvie/passion/raw/master/docker-compose.yml
echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)" > .env
docker compose up -d
```

Open <http://localhost:8080> and sign up. `/healthz` gives 503 when the database is not
available.

To upgrade:

```sh
docker compose pull
docker compose up -d
```

The image is `ghcr.io/awalvie/passion`, for amd64 and arm64. Each release has its own tag,
for example `v3.0.0`. `latest` is the newest release.

## Binary

Download the archive for your system from
[Releases](https://github.com/awalvie/passion/releases) and unpack it. Make an empty
Postgres 18 database, then start Passion with its address:

```sh
DATABASE_URL=postgres://passion:password@localhost/passion ./passion
```

To upgrade, replace the binary and start it again.

### With systemd

1. Copy `passion` to `/usr/local/bin/`.
2. Make `/etc/passion/env` with the line `DATABASE_URL=postgres://...`.
3. Copy `deploy/passion.service` to `/etc/systemd/system/`.
4. Run `systemctl enable --now passion`.

## From source

Install [nix](https://nixos.org), then run:

```sh
nix develop
make client
go build -o passion ./server/cmd/passion
```

## Before other people use it

- Serve it over HTTPS. Over plain HTTP, a phone screen goes dark during a timer. With Caddy:
  `example.com { reverse_proxy localhost:8080 }`. The compose file opens port 8080 on every
  interface. Behind a proxy, change it to `127.0.0.1:8080:8080`.
- Anyone who can reach the server can sign up.

## Configuration

`DATABASE_URL` is the only setting you must give. For the others, copy
[passion.example.yaml](../passion.example.yaml) and set `PASSION_CONFIG` to its path. If a
key or a value is wrong, the server does not start.

With Docker, add two lines to the `app` service:

```yaml
  app:
    environment:
      DATABASE_URL: ...
      PASSION_CONFIG: /etc/passion/passion.yaml
    volumes:
      - ./passion.yaml:/etc/passion/passion.yaml:ro
```

| Key | Default | Notes |
|---|---|---|
| `server.addr` | `:8080` | `PASSION_ADDR` replaces it |
| `database.url` | | Required. `DATABASE_URL` replaces it |
| `auth.token_life` | `720h` | How long a sign-in lasts when it is not used |
| `log.level` | `info` | `debug`, `info`, `warn` or `error` |
| `log.format` | `text` | `text` or `json` |
| `catalog.private` | | Private catalogs, and the email of each owner |

The server loads private catalogs when it starts. It loads them only for owners who have an
account. If a new owner signs up, restart the server.

## Backups

The migrations go forward only. To go back to an earlier version, you must restore a backup.
Back up before you upgrade:

```sh
docker compose exec db pg_dump -U passion passion > backup.sql
```

With the binary, use `pg_dump` on your own database.
