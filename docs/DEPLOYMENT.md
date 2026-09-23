# Deployment — where, why, and what it costs

**What this document is for:** why hakaishield is hosted where it is, which
options were rejected and on what grounds, and what the bill does as traffic
grows. Written so that in six months nobody has to re-derive the reasoning, and
so that "let's just put it behind Cloudflare" gets answered before it costs a
week.

It also records what the large vendors do differently, because one of those
differences is worth more than every cost optimisation on this page combined.

Researched and written 2026-09-22. Prices checked against sources listed at the
end; re-check before trusting them.

---

## 1. The constraint that decides everything

hakaishield reads the **raw TLS ClientHello** to compute the JA4 fingerprint.
That is the product's foundational signal — the one thing a scraper cannot fake
without genuinely changing its TLS stack.

To see the ClientHello, **hakaishield has to terminate TLS itself.**

Everything else on this page follows from that one sentence. Any platform that
terminates TLS before our process sees the connection hands us *its* handshake,
not the visitor's. The fingerprint becomes a constant, `ua_mismatch` and
`ja4_blocklist` stop meaning anything, and the product quietly becomes a
header-checker.

It would still run. It would still return decisions. They would just be worth
much less, and nothing would fail loudly enough to notice.

So the requirement is: **TCP port 443 reaching our process, unterminated.**

---

## 2. What that rules out

| Platform | Why not |
|---|---|
| **Vercel, Netlify** | Terminate TLS at their edge. No raw ClientHello. Also built for apps, not long-lived inline proxies. |
| **Railway** | Terminates TLS at its edge. Otherwise a good fit — this is the only reason it was rejected. |
| **Cloudflare (proxied DNS)** | Terminates TLS. Ironically, Cloudflare in front of hakaishield means Cloudflare's bot detection runs and ours sees Cloudflare's handshake. TCP passthrough exists (Spectrum) but is enterprise-priced. |
| **AWS ALB / CloudFront** | Both terminate TLS. Putting either in front breaks the product in exactly the same way. |
| **Fly.io** | *Works*, with `handlers = []` on port 443 for TCP passthrough. A genuine option, listed here because it was not rejected on technical grounds. |

**What works:** anything that gives you a plain public IP and port 443. A VPS,
an EC2 instance, a bare-metal box. That is the whole requirement.

If a load balancer is ever needed, it must be an **AWS NLB in TCP mode**, not
HTTPS mode — TCP mode passes bytes through without terminating.

---

## 3. Why Hetzner

**Decided 2026-09-23: Hetzner Cloud.**

The comparison that decided it, for a product whose dominant cost is
bandwidth (§4):

| | Price | Bandwidth included | Latency from India |
|---|---|---|---|
| **Hetzner CX22** | ~€4.5/mo | **20 TB**, then ~$0.001/GB | 60–150 ms (Singapore) |
| AWS Mumbai (free tier) | free for 12 months | 100 GB, then $0.09/GB | best |
| DigitalOcean Bangalore | $4/mo | 1 TB | best |
| Contabo | €5/mo | 32 TB | 150–200 ms |

**Hetzner is ~90x cheaper per GB of egress than AWS**, and egress is the line
item that grows with the product. 20 TB included covers a great deal of real
traffic before anything is billed at all.

**What is given up, stated plainly:** Hetzner has no India datacenter.
Singapore is the closest region, so Indian visitors pay 60–150 ms of network
latency that AWS Mumbai would not charge them. For a proxy sitting in front of
a site, that latency is added to every request. It is a real cost, accepted
because bandwidth economics decide this product's viability and latency does
not.

AWS Mumbai was the other serious candidate — free for a year and physically
close. Rejected because the free tier ends, and at that point its 100 GB
egress allowance and $0.09/GB rate make it the most expensive option on this
list for exactly the thing we do most of.

Hetzner is not a small operator, which is worth recording since the website
does not suggest otherwise: founded 1997, over 500,000 servers, more than a
million cloud instances, a 99.9% monthly SLA, and datacenters in Germany,
Finland, the US and Singapore.

### The setup

```text
Hetzner CX22 (~€4.5/mo, Singapore)   Ubuntu, plain public IP
  2 vCPU, 4 GB RAM, 20 TB traffic
  Firewall: 443 (traffic), 80 (certbot renewal), 22 (SSH, your IP only)

Postgres   -> Supabase. Managed backups, and a dead box does not take the
              customer data with it.
Redis      -> Docker on the same box, no published port. Velocity counters
              only; losing them fails those signals open.
Dashboard  -> Vercel or Cloudflare Pages. Static files, free, and NOT in the
              request path — so TLS termination there does not matter.
TLS certs  -> certbot / Let's Encrypt, auto-renewed with a reload hook.
```

**No load balancer, no CDN in front.** Both terminate TLS and would break the
product — see §1.

### Moving later costs nothing

There is nothing host-specific in the deployment: a Go binary, a Redis
container and a certificate. If the India latency turns out to matter more
than the bandwidth bill, moving to AWS Mumbai or DigitalOcean Bangalore is an
afternoon.

---

## 4. The cost problem nobody sees until the bill

This is the most important section on the page.

### We carry the bytes. The large vendors do not.

Look at how DataDome actually deploys. It is a **module** — an Akamai
EdgeWorker, a CloudFront Lambda@Edge function, a Fastly or nginx module. On
each request the module makes a **sideband call** to the nearest DataDome
endpoint with request *metadata*, gets back a verdict in about 2ms over a
keep-alive connection, and then the CDN serves the content.

**DataDome never touches the response body.** The customer's page bytes go
CDN → visitor, and DataDome's infrastructure never sees them. Akamai's Bot
Manager is the same idea from the other direction: the detection runs on the
same hop that was already delivering the traffic, so there is no second copy.

hakaishield today is a **full reverse proxy**. Every byte of every response
passes through our box and out of our network interface. We pay egress on all
of it.

### What that costs

AWS charges nothing for data *in* and $0.09/GB for data *out* to the internet
after the first 100 GB/month free (then $0.085 over 10 TB, $0.07 over 50 TB,
$0.05 over 150 TB).

At an average response of 100 KB:

| Requests/month | Egress | AWS cost/month |
|---|---|---|
| 1 M | 100 GB | ~$0 (free tier) |
| **10 M** | **1 TB** | **~$81** |
| 100 M | 10 TB | ~$810 |
| 1 B | 100 TB | ~$6,900 |

The compute is a rounding error next to that. **Bandwidth is the business
model**, and a reverse proxy pays it twice over compared to a sideband
design — once to pull from origin (if the origin is outside our network) and
once to push to the visitor.

### What this means strategically

At 10 M requests a month the proxy architecture is completely fine — $81 is
nothing. It stays fine into the tens of millions.

At hundreds of millions it stops being fine, and the answer is not a cheaper
instance. It is the architecture: **a sideband decision API that the customer's
own CDN or nginx calls**, exactly like DataDome's modules. We keep the
detection and the evidence; the customer's existing infrastructure keeps
carrying its own bytes.

That is a real future roadmap item and it should be written down as one rather
than discovered at $7,000 a month. Recording it here is the point of this
document.

Worth noting: the proxy model is not only a cost. It is also **why our evidence
is better** — we see the whole request, not a summary someone else chose to
send us. The sideband model would be an option for high-volume customers, not
a replacement.

---

## 5. Reducing the bill, ranked by what it actually saves

**1. Block early — the product pays for itself here.**
A blocked request sends ~100 bytes instead of a 100 KB page. If 20% of traffic
is automated and gets blocked or challenged, that is roughly **20% off the
bandwidth bill**, before counting the origin load saved. The challenge page is
a few KB; deception responses can be made deliberately small. This is the one
"cost optimisation" that is also the feature.

**2. On AWS, never put a NAT Gateway in the path.** (Not applicable on
Hetzner, kept because the trap is expensive and someone will evaluate AWS
again.)
It bills twice: ~$0.045/hour (~$32/month per AZ) **plus $0.045/GB** for every
byte through it. At 1 TB that is another $45 on top of the $81 egress, for
nothing. Put the instance in a **public subnet with an Internet Gateway**,
which has no per-GB charge. Note that a Compute Savings Plan does **not** cover
NAT Gateway charges — people get caught by this.

**3. Keep everything in one Availability Zone.**
Cross-AZ traffic is billed in both directions. For a single-box deployment
there is no reason to cross an AZ at all.

**4. Graviton (ARM) instances.**
Go cross-compiles to ARM64 with no code changes. `t4g` is meaningfully cheaper
than `t3` for the same work. Free tier terms vary, so check before switching.

**5. Compression.**
Make sure responses are compressed on the way out. Egress is billed on bytes on
the wire, so gzip/brotli on a 100 KB HTML page cuts that line item by 60–80%.
Verify hakaishield is not accidentally decompressing and re-sending plain.

**6. Reserved Instances / Savings Plans — only once usage is steady.**
Up to ~70% off compute. Irrelevant while compute is a rounding error next to
bandwidth, and useless against bandwidth itself.

**7. When bandwidth dominates, leave AWS for the data plane.**
Hetzner includes ~20 TB per server and charges roughly **$0.001/GB** on
overage — about **90× cheaper** than AWS's $0.09/GB. At 100 TB/month the same
traffic is thousands of dollars on AWS and a rounding error on Hetzner. Egress
pricing, not compute pricing, is what should pick the host for this product at
scale.

**8. Set a billing alarm before anything else.**
AWS Budgets, alert at a number that would hurt. Free tier ending is silent, and
a misconfigured test loop is not.

---

## 6. What the big vendors do, and what is worth stealing

Read for architecture, not feature envy (`CLAUDE.md` §0: do not add features
because a large vendor has them).

### Layered scoring with an exposed reason — we already agree

Cloudflare gives every request a bot score of 1–99 from several engines:
**Heuristics** (score 1 for high-confidence, 29 while confidence is being
assessed), **Machine Learning** (2–99, the majority of detections),
**JavaScript Detections** (headless and automation fingerprints), and a
deprecated Anomaly Detection engine. Alongside the score they expose **Bot
Score Source**, **Detection IDs** and **Bot Tags** — machine-readable reasons.

Akamai scores 0–100 and groups responses into Cautious / Strict / Aggressive
bands that the customer tunes.

Two things to take from this:

- **Layered engines feeding one score is the right shape**, and it is the shape
  `pkg/signals` plus `pkg/decide` already has. Our band structure
  (allow / challenge / deceive / block) is their response segments.
- **They expose a reason, but only as a tag.** "Detection ID 1234, tag
  `automated_browser`." Our `Explain` gives the per-signal contribution in
  log-odds with the arithmetic checkable. That gap is the product's whole
  positioning, and it survives contact with what the leaders actually ship.

### Decision caching and keep-alive — worth taking

DataDome uses **persistent keep-alive connections** between the module and its
API so the decision call does not pay TCP and TLS setup each time. The
equivalent for us: keep-alive to the origin (which `httputil.ReverseProxy`
does by default — verify it is not being disabled), and if a sideband API is
ever built, that is the pattern to copy.

### Bloom and cuckoo filters — the actual engineering lesson

Worth understanding properly before reaching for one.

**The Cloudflare lesson** ("When Bloom filters don't bloom"): they built a
Bloom-filter deduplicator for ~1 billion IP records. The maths said it should
be fast. It took 12 seconds where hashing alone took 2 — the filter operations
themselves cost 10 seconds. The cause was **random memory access**: a large bit
array does not fit in cache, and every probe is a cache miss. A Bloom filter
sized past L2/L3 is far slower than the arithmetic predicts.

**Cuckoo filters** (Fan et al., CoNEXT 2014) are usually the better modern
choice: they support deletion, are more space-efficient than Bloom below a 3%
false-positive rate, and — the part that matters here — always read **at most
two cache lines**, so their cost is predictable whether the answer is yes or
no. A space-optimised Bloom filter at a 1% false-positive rate needs 7 probes,
each a potential miss.

**And the rule that matters most for us**, stated plainly in the Perfect Cuckoo
Filter paper: if a filter decides whether to *block* an IP, a false positive
**disables communication from a legitimate address**. The authors name this as
a case where a plain Bloom filter cannot be used.

That is `CLAUDE.md` §14 arrived at independently by network researchers. So if
hakaishield ever builds a large IP or fingerprint blocklist:

- The filter is a **fast negative** — "definitely not in the list, stop here."
- A positive is a **hint**, never a verdict. Confirm against the real list.
- Size it to fit in cache, or measure and be disappointed.

It maps exactly onto the no-lone-signal rule we already enforce.

### What is explicitly not worth copying

Cloudflare and Akamai win on **scale of data** — 40 billion bot requests a day,
5 trillion signals. We will never out-sample them and should not try. Our
advantage is per-request evidence a customer can argue with, and use cases
where a 450-PoP CDN is the wrong shape.

---

## 7. The actual deployment

> **The files are in [`deploy/`](../deploy/).** `setup.sh` provisions a fresh
> box (Docker, certbot, firewall, renewal hook, and a renewal dry-run so it is
> proven now rather than in 90 days), `docker-compose.yml` runs hakaishield and
> Redis, and `hakaishield.service` is the non-Docker path. Step-by-step
> commands are in [`deploy/README.md`](../deploy/README.md).
>
> The strength ladder is `bot-testing/ladder/ladder.py` — seven rungs, each
> adding one capability, run from your own machine against your own site.



Production, on a domain you own.

```bash
# --- on the EC2 box ---

# 1. Certificate (port 80 must be open for the challenge)
sudo certbot certonly --standalone -d neurofiq.in

# 2. Redis for velocity signals (fails open without it, but the signals go quiet)
docker run -d --restart=always --name redis -p 127.0.0.1:6379:6379 redis:7-alpine

# 3. hakaishield — SHADOW FIRST, ALWAYS
sudo ./hakaishield \
  -addr :443 \
  -target http://127.0.0.1:3000 \
  -tls-cert /etc/letsencrypt/live/neurofiq.in/fullchain.pem \
  -tls-key  /etc/letsencrypt/live/neurofiq.in/privkey.pem \
  -mode shadow \
  -evidence-token "$(openssl rand -hex 32)" \
  -redis-url redis://localhost:6379 \
  -db-url "$DATABASE_URL"
```

**DNS:** `neurofiq.in` A record → the Elastic IP. The real site stays on
`127.0.0.1:3000` behind it.

**Note the flag that is not there.** `-collect-labels` is deliberately absent:
while you are attacking your own site, every solved challenge and honeypot trip
would become a training sample, and the model would learn what *your* bots look
like. Turn it on when real traffic arrives, not before. See
`docs/LEARNED_SCORING.md`.

**Shadow first, always.** Watch what it *would* do for a few days:

```bash
curl -H "Authorization: Bearer $TOKEN" https://neurofiq.in/api/v1/dashboard/evidence
```

Only switch to `-mode enforce` once the evidence trail shows nothing legitimate
is being caught.

### Checklist before the first real visitor

- [ ] Billing alarm set (§5.8)
- [ ] Elastic IP attached, DNS pointing at it
- [ ] Certificate auto-renew tested (`certbot renew --dry-run`)
- [ ] `systemd` unit so it restarts on crash and on reboot
- [ ] Security group: 22 restricted to your IP, not `0.0.0.0/0`
- [ ] `-evidence-token` is random and not in shell history
- [ ] Started in `-mode shadow`
- [ ] `-collect-labels` **off** while you are testing

---

## 8. Open questions

- **When does the sideband model become necessary?** §4 says "hundreds of
  millions of requests". The number should be measured, not guessed, once
  there is a real traffic profile. It is a roadmap item, not a surprise.
- **HTTP/2 and the TLS constraint.** HTTP/2 fingerprinting is not built. When
  it is, ALPN negotiation has to stay under our control, which reinforces
  everything in §1.
- **Multi-region.** Everything here is one box in one AZ. Anycast and multiple
  PoPs are what make a 2ms decision possible for the large vendors; that is a
  different architecture, not a bigger instance.

---

## Sources

Checked 2026-09-22. Prices and vendor behaviour change — re-verify.

- AWS data transfer out pricing (100 GB/month free, then $0.09/GB tiering):
  [EgressCost](https://egresscost.com/aws/data-transfer-pricing/),
  [Economize](https://www.economize.cloud/blog/aws-data-transfer-costs-2026/)
- NAT Gateway hourly + per-GB charges, and Savings Plans not covering them:
  [CloudForecast](https://www.cloudforecast.io/blog/aws-nat-gateway-pricing-and-cost/),
  [enforza](https://enforza.io/aws-nat-gateway-cost/)
- Hetzner included bandwidth and ~$0.001/GB overage:
  [Cherry Servers](https://www.cherryservers.com/blog/free-egress-cloud-providers),
  [EgressCost comparison](https://egresscost.com/compare/)
- DataDome's sideband/edge-module architecture, ~2ms decisions, keep-alive
  connections, Akamai EdgeWorker and CloudFront Lambda@Edge modules:
  [DataDome CDN integrations](https://datadome.co/changelog/cdn-integration-at-the-edge/),
  [AWS APN blog](https://aws.amazon.com/blogs/apn/preventing-online-fraud-and-attacks-with-aws-and-datadome-real-time-bot-protection/)
- Cloudflare bot score 1–99, detection engines, Bot Tags and Detection IDs:
  [Cloudflare Reference Architecture](https://developers.cloudflare.com/reference-architecture/diagrams/bots/bot-management/)
- Akamai Bot Manager 0–100 scoring and response segments:
  [Akamai Bot Manager](https://www.akamai.com/products/bot-manager)
- Bloom filter cache-miss lesson:
  [When Bloom filters don't bloom, Cloudflare](https://blog.cloudflare.com/when-bloom-filters-dont-bloom/)
- Cuckoo filters, two-cache-line lookups, deletion support:
  [Cuckoo Filter: Practically Better Than Bloom (CoNEXT 2014)](https://conferences2.sigcomm.org/co-next/2014/CoNEXT_papers/p75.pdf)
- False positives being unacceptable for IP blocklists:
  [Perfect Cuckoo Filters (CoNEXT 2021)](https://pontarelli.di.uniroma1.it/publication/conext21/Conext21.pdf)

---

## Related

- `docs/WHAT_IS_BUILT.md` — what actually works today
- `docs/ARCHITECTURE.md` — the request path this deploys
- `docs/LEARNED_SCORING.md` — why `-collect-labels` stays off during testing
- `README.md` — every flag
