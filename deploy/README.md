# Deploying hakaishield

The reasoning — why this host, why not the managed ones, what bandwidth
costs — is in [`../docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md). This file is
the commands.

**Read `DEPLOYMENT.md` §1 before substituting any platform.** hakaishield
terminates TLS itself to read the ClientHello. Anything that terminates TLS
first (Cloudflare proxied, Railway, Vercel, an ALB) silently turns JA4 into a
constant and quietly guts detection. Nothing errors.

---

## What goes where

```text
DigitalOcean Basic Droplet, 2 vCPU / 4 GB, region BLR1 (Bangalore).
$24/mo, 4 TB transfer included, $0.01/GiB after. ~5–40ms from Indian
cities. Keep the ORIGIN in the same region, or a visitor pays that hop
three times (see docs/DEPLOYMENT.md §3).
├── hakaishield   :443, in Docker
└── Redis         no published port; only hakaishield reaches it

Supabase        Postgres. Managed backups, and a dead box does not
                take the customer data with it.
Vercel / Pages  the dashboard. Static files, free, not in the request
                path — so TLS termination there does not matter.
```

Do not self-host Postgres on the same box. Redis is fine there: it holds
velocity counters, and losing them fails the signals open rather than losing
anything that matters.

---

## First time

```bash
# On a fresh box, as root
git clone <repo> /opt/hakaishield
cd /opt/hakaishield
bash deploy/setup.sh neurofiq.in you@example.com
```

That installs Docker and certbot, sets a default-deny firewall open on 22/80/443,
issues the certificate, installs a renewal hook that restarts hakaishield when
the certificate rolls, and **verifies renewal works now** rather than in 90 days.

Then:

```bash
cp deploy/.env.example deploy/.env
$EDITOR deploy/.env                 # EVIDENCE_TOKEN: openssl rand -hex 32
docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml logs -f
```

Point the domain's A record at the box.

---

## It starts in shadow mode

`HAKAISHIELD_MODE=shadow` — the full pipeline runs, every decision is recorded,
and nothing is acted on. Every visitor reaches the origin untouched.

Watch it for a few days:

```bash
curl -s -H "Authorization: Bearer $EVIDENCE_TOKEN" \
  https://neurofiq.in/api/v1/dashboard/evidence | jq '.[:20]'
```

Switch to `HAKAISHIELD_MODE=enforce` only once nothing legitimate is being
caught. Then `docker compose ... up -d` again.

---

## Testing it

```bash
pip install requests playwright && playwright install chromium
python3 bot-testing/ladder/ladder.py --url https://neurofiq.in/ --token "$EVIDENCE_TOKEN"
```

Seven rungs, each adding exactly one capability over the last — stdlib client,
`requests`, a lying user agent, full browser headers, a crawl pattern, headless
Playwright, headful Playwright. **The rung where detection stops is your
answer.** Run it from your own machine, not from the server: DigitalOcean acts
on abuse reports, and outbound attack traffic from a droplet is how you lose an
account — the same is true of every provider on that list.

**Keep `-collect-labels` off while you do this.** It is deliberately absent
from the compose file. Every solved challenge and honeypot trip would become a
training sample, and the model would learn what *your* bots look like rather
than what real ones do. See [`../docs/LEARNED_SCORING.md`](../docs/LEARNED_SCORING.md).

---

## Running without Docker

`hakaishield.service` runs the binary directly under systemd, with
`CAP_NET_BIND_SERVICE` so it binds :443 without being root, and the rest of
the sandbox taken away. Use that **or** compose, not both.

```bash
sudo useradd --system --no-create-home hakaishield
sudo cp deploy/hakaishield.service /etc/systemd/system/
sudo cp deploy/.env /opt/hakaishield/.env
sudo systemctl daemon-reload && sudo systemctl enable --now hakaishield
sudo journalctl -u hakaishield -f
```

---

## Before the first real visitor

- [ ] Renewal proven: `certbot renew --dry-run`
- [ ] SSH restricted to your IP, not `0.0.0.0/0`
- [ ] `EVIDENCE_TOKEN` random, and not in shell history
- [ ] Started in `shadow`
- [ ] `-collect-labels` off while testing
- [ ] Billing alert set with the host
- [ ] Reboot the box once and confirm it comes back up by itself

That last one is the only way to know the restart configuration is real.

---

## When something is wrong

```bash
docker compose -f deploy/docker-compose.yml logs --tail=100 hakaishield
curl -sk https://localhost/__hakaishield/healthz     # from the box itself
openssl s_client -connect neurofiq.in:443 -servername neurofiq.in </dev/null 2>&1 | head -20
```

**Every JA4 looks identical** → something is terminating TLS in front of you.
Re-read `DEPLOYMENT.md` §1.

**Certificate errors 90 days in** → the renewal hook never ran. That is what
`certbot renew --dry-run` was meant to catch.

**Port 443 refused** → in Docker the process listens on 8443 and the host
publishes 443; check the port mapping before the process.
