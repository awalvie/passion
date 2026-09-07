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
