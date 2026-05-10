#!/usr/bin/env bash
set -euo pipefail

# ── Config — edit these before running ────────────────────────────────────────
DOMAIN="passion.awalvie.me"
DEPLOY_USER="${SUDO_USER:-$(whoami)}"
# ──────────────────────────────────────────────────────────────────────────────

echo "==> Installing Caddy"
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends caddy

echo "==> Writing Caddyfile"
sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
${DOMAIN} {
    reverse_proxy localhost:3000
}
EOF
sudo systemctl reload caddy

echo "==> Creating app directory"
sudo mkdir -p /opt/passion/catalog /opt/passion/templates /opt/passion/static
sudo chown -R "${DEPLOY_USER}:${DEPLOY_USER}" /opt/passion

echo "    Note: passion.yaml is written automatically by the GitHub Actions deploy workflow."
echo "    Add DEPLOY_JWT_SECRET to your repository secrets before pushing."

echo "==> Writing systemd service"
sudo tee /etc/systemd/system/passion.service > /dev/null <<EOF
[Unit]
Description=Passion Training App
After=network.target

[Service]
WorkingDirectory=/opt/passion
ExecStart=/opt/passion/passion -config /opt/passion/passion.yaml
Restart=always
User=${DEPLOY_USER}

[Install]
WantedBy=multi-user.target
EOF

echo "==> Enabling systemd service"
sudo systemctl daemon-reload
sudo systemctl enable passion

echo ""
echo "Done. Next steps:"
echo "  1. Point ${DOMAIN} to this server's IP"
echo "  2. Allow port 80/443 in your Oracle security list / iptables:"
echo "       sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT"
echo "       sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT"
echo "  3. Add these GitHub Actions secrets:"
echo "       DEPLOY_HOST        = <this server's IP>"
echo "       DEPLOY_USER        = ${DEPLOY_USER}"
echo "       DEPLOY_SSH_KEY     = <private key whose public key is in ~/.ssh/authorized_keys>"
echo "       DEPLOY_JWT_SECRET  = <$(openssl rand -hex 32)>"
echo "  4. Push to main — the service will start automatically and the config will be deployed"
