# Deploy

Passion is one Docker image. It needs a Postgres 18 database.

```sh
make image
docker run -e DATABASE_URL=postgres://... -p 8080:8080 passion:dev
```

The server updates the database when it starts.

Serve the app over HTTPS. If you use plain HTTP, a phone screen goes dark during a timer.

Each person who can get to the server can sign up.

`/healthz` gives 503 when the database is not available.

## Configuration

The settings are in a YAML file. Set `PASSION_CONFIG` to the path of this file.
[passion.example.yaml](../passion.example.yaml) shows all the keys. If a key or a value is
wrong, the server does not start.

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

## The deploy workflow

The workflow builds the image and pushes it to GHCR. Then it connects to your server with SSH
and starts the app with Postgres.

To start the workflow, run `gh workflow run deploy.yml --ref master`.

If you fork the repository, change the image name and the catalog repository name. They are
in `deploy.yml` and in `deploy/compose.yaml`.

To prepare the server:

1. Install Docker with the compose plugin.
2. Add the deploy user to the `docker` group.
3. Make the folder `/srv/passion`. The deploy user must own it.
4. Send traffic from your reverse proxy to `127.0.0.1:8081`. For Caddy, use
   `example.com { reverse_proxy 127.0.0.1:8081 }`.

Add these secrets to the repository:

- `DEPLOY_HOST`, `DEPLOY_USER` and `DEPLOY_SSH_KEY`.
- `DEPLOY_KNOWN_HOSTS`. Get it with `ssh-keyscan`.
- `PRIVATE_CATALOG_DEPLOY_KEY`.
- `POSTGRES_PASSWORD`. Use only letters, digits and `. _ ~ -`.

Add the variable `CATALOG_OWNER`. It is the email of the account that gets the private
catalog.

Postgres reads the password only when it makes its volume. To change the password:

1. Run `ALTER USER` in the database.
2. Change the secret.

## Backups

The migrations go forward only. To go back to an earlier version, you must restore a backup.
Before you deploy a new migration, back up the `db` volume with `pg_dump`.
