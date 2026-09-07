# scripts

## server_setup.sh

One-time server setup. Installs Caddy, creates a systemd service, and writes `/opt/passion/passion.yaml` with a generated JWT secret.

Copy to the server and run:

```sh
scp scripts/server_setup.sh user@your-server-ip:/tmp/server_setup.sh
ssh user@your-server-ip "chmod +x /tmp/server_setup.sh && /tmp/server_setup.sh"
```

Edit `DOMAIN` at the top of the script before running.

After running, open ports 80 and 443:

```sh
sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
```

On Oracle Cloud also add ingress rules for ports 80/443 in the OCI console under Networking → VCN → Security Lists.

## verify-catalog.sh

Checks the catalog is in one piece, by comparing the live database against the newest
`passion.db.bak-*` beside it. Read-only. Ships with the deploy, so it is already on the
server:

```sh
cd /opt/passion && ./scripts/verify-catalog.sh
```

Takes an optional directory argument, so it also runs against a local copy:

```sh
./scripts/verify-catalog.sh /path/to/a/copy
```

It confirms no rows are left under a removed owner, that your runs and ticks are
unchanged, that no completion points at a missing exercise, and that no new foreign key
violations or orphaned exercises appeared. Exits non-zero on any failure.

Two things it deliberately does **not** treat as failures. It counts live rows rather than
totals, because the importer retires a template's child rows and writes a fresh generation
on every run — totals climb legitimately. And orphaned exercises are reported as a
before/after delta rather than a pass/fail: 426 of them predate the September importer fix,
and `--purge-orphans` is the tool for those.

Written during the 4 September 2026 catalog recovery — see
[docs/RECOVERY.md](../docs/RECOVERY.md).
