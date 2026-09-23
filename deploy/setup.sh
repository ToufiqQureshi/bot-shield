#!/usr/bin/env bash
# One-time provisioning for a fresh Ubuntu box. Run as root.
#
# Written against a DigitalOcean droplet (docs/DEPLOYMENT.md §3) but there is
# nothing provider-specific in it — it works on any Ubuntu host that gives you
# a plain public IP and port 443.
#
#   bash deploy/setup.sh neurofiq.in you@example.com
#
# It is deliberately not idempotent-magic: each step prints what it does so
# you can run them by hand instead if something looks wrong.
set -euo pipefail

DOMAIN="${1:?usage: setup.sh <domain> <email>}"
EMAIL="${2:?usage: setup.sh <domain> <email>}"

echo "==> Packages"
apt-get update -qq
apt-get install -y -qq ufw certbot docker.io docker-compose-plugin

echo "==> Firewall"
# Default-deny inbound. 80 stays open because certbot's HTTP-01 challenge
# needs it at renewal time, not just at issue time.
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   comment 'ssh'
ufw allow 80/tcp   comment 'certbot http-01 renewal'
ufw allow 443/tcp  comment 'hakaishield'
ufw --force enable

echo "==> TLS certificate for ${DOMAIN}"
# --standalone binds :80 itself, so nothing else may be listening there.
# Without a certificate hakaishield cannot terminate TLS, which means no
# ClientHello, which means no JA4 - the product's core signal.
certbot certonly --standalone --non-interactive --agree-tos \
    -m "${EMAIL}" -d "${DOMAIN}"

echo "==> Certificate renewal"
# certbot's packaged timer handles renewal. The deploy hook restarts
# whatever is serving so it picks up the new certificate - a renewed cert
# that nothing reloaded is an outage 90 days after install.
mkdir -p /etc/letsencrypt/renewal-hooks/deploy
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-hakaishield.sh <<'HOOK'
#!/usr/bin/env bash
set -eu
if systemctl is-active --quiet hakaishield; then
    systemctl restart hakaishield
elif [ -f /opt/hakaishield/deploy/docker-compose.yml ]; then
    docker compose -f /opt/hakaishield/deploy/docker-compose.yml restart hakaishield
fi
HOOK
chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-hakaishield.sh
systemctl enable --now certbot.timer

echo "==> Verifying renewal actually works"
# Do this now, not in 90 days when it silently fails.
certbot renew --dry-run

echo
echo "Done. Next:"
echo "  1. cp deploy/.env.example deploy/.env  and fill it in"
echo "  2. docker compose -f deploy/docker-compose.yml up -d --build"
echo "  3. Point ${DOMAIN}'s A record at this box"
echo
echo "It starts in shadow mode: it records decisions and acts on none."
echo "Watch it for a few days before switching HAKAISHIELD_MODE to enforce."
