#!/usr/bin/env python3
"""Run increasingly sophisticated clients against your own hakaishield
deployment and report what it decided about each one.

This is a strength test, not training data. Every client here is an
off-the-shelf tool used as-is: the point is to find the rung where
detection stops working, not to invent ways past it.

    python3 ladder.py --url https://neurofiq.in/ --token "$EVIDENCE_TOKEN"

Only run this against a site you own. And keep -collect-labels OFF on the
server while you do: these requests would otherwise become training
samples and teach the model what YOUR bots look like, not what real ones
do (docs/LEARNED_SCORING.md).

Higher rungs need optional packages; each one reports itself as skipped
rather than failing the run:

    pip install requests playwright && playwright install chromium
"""

import argparse
import json
import sys
import time
import urllib.request

# Each rung is (name, what it exercises, callable -> status code or None).
# The order is deliberate: every rung adds exactly one capability over the
# one before it, so the rung where detection stops is the answer.


def rung_urllib(url):
    """Plain stdlib client. No browser headers, honest user agent."""
    req = urllib.request.Request(url)
    with urllib.request.urlopen(req, timeout=15) as r:
        return r.status


def rung_requests(url):
    """requests with its default user agent - openly a script."""
    import requests
    return requests.get(url, timeout=15).status_code


def rung_requests_browser_ua(url):
    """requests claiming to be Chrome. The user agent is a lie the TLS
    handshake does not back up - this is what ua_mismatch exists for."""
    import requests
    return requests.get(url, timeout=15, headers={
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                      "AppleWebKit/537.36 (KHTML, like Gecko) "
                      "Chrome/120.0.0.0 Safari/537.36",
    }).status_code


def rung_requests_full_headers(url):
    """The same lie, told properly: every header a real Chrome navigation
    sends. Header checks go quiet; the handshake still does not match."""
    import requests
    return requests.get(url, timeout=15, headers={
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                      "AppleWebKit/537.36 (KHTML, like Gecko) "
                      "Chrome/120.0.0.0 Safari/537.36",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Accept-Language": "en-US,en;q=0.9",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "none",
        "Sec-Fetch-User": "?1",
        "Sec-CH-UA": '"Chromium";v="120", "Not(A:Brand";v="24"',
        "Sec-CH-UA-Mobile": "?0",
        "Sec-CH-UA-Platform": '"Windows"',
        "Upgrade-Insecure-Requests": "1",
    }).status_code


def rung_crawl(url):
    """Twelve distinct paths in quick succession, fetching no subresources.
    A real browser never browses like this - crawl_pattern's whole point."""
    import requests
    session = requests.Session()
    last = None
    for i in range(12):
        last = session.get(f"{url.rstrip('/')}/page-{i}", timeout=15).status_code
        time.sleep(0.05)
    return last


def rung_playwright_headless(url):
    """A real browser engine, headless. Runs JavaScript and renders a
    canvas, so the challenge itself is passable - but the automation
    framework leaves globals behind."""
    from playwright.sync_api import sync_playwright
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        resp = page.goto(url, timeout=30000)
        status = resp.status if resp else None
        browser.close()
        return status


def rung_playwright_headful(url):
    """The same browser with a real window. Removes the headless renderer
    tell; the automation globals are still there."""
    from playwright.sync_api import sync_playwright
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=False)
        page = browser.new_page()
        resp = page.goto(url, timeout=30000)
        status = resp.status if resp else None
        browser.close()
        return status


LADDER = [
    ("1. urllib (stdlib)",        "no browser headers, honest UA",     rung_urllib),
    ("2. requests (default UA)",  "openly a script",                   rung_requests),
    ("3. requests + Chrome UA",   "UA lies, handshake does not",       rung_requests_browser_ua),
    ("4. requests + all headers", "header checks satisfied",           rung_requests_full_headers),
    ("5. crawl pattern",          "12 paths, no subresources",         rung_crawl),
    ("6. Playwright headless",    "real engine, runs JS",              rung_playwright_headless),
    ("7. Playwright headful",     "real window, real renderer",        rung_playwright_headful),
]


def evidence(base, token, limit):
    """Read back what hakaishield recorded, so the report says which checks
    fired rather than only what status came back."""
    if not token:
        return []
    root = base.split("/", 3)
    api = f"{root[0]}//{root[2]}/api/v1/dashboard/evidence?limit={limit}"
    req = urllib.request.Request(api, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            body = json.load(r)
    except Exception as err:
        print(f"  (could not read evidence: {err})", file=sys.stderr)
        return []
    # The endpoint's envelope has changed shape before; accept either.
    if isinstance(body, dict):
        return body.get("evidence") or body.get("data") or []
    return body


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", required=True, help="a site you own, e.g. https://neurofiq.in/")
    ap.add_argument("--token", default="", help="-evidence-token, to read back what fired")
    ap.add_argument("--only", type=int, help="run a single rung by number")
    args = ap.parse_args()

    print(f"\nhakaishield strength ladder -> {args.url}")
    print("Each rung adds one capability. The rung where detection stops is the answer.\n")

    results = []
    for i, (name, what, fn) in enumerate(LADDER, start=1):
        if args.only and args.only != i:
            continue
        try:
            status = fn(args.url)
            outcome = str(status)
        except ImportError as err:
            outcome = f"skipped ({err.name} not installed)"
        except Exception as err:
            # A refused connection or a 403 is a result, not a crash.
            outcome = f"{type(err).__name__}: {err}"
        results.append((name, what, outcome))
        print(f"  {name:<28} {what:<34} -> {outcome}")
        time.sleep(1)

    recent = evidence(args.url, args.token, limit=len(results) * 4)
    if recent:
        print("\nWhat hakaishield recorded (newest first):\n")
        for e in recent[:20]:
            signals = ",".join(e.get("signals") or []) or "-"
            enforced = "" if e.get("enforced", True) else "  (shadow: not acted on)"
            print(f"  {e.get('decision','?'):<10} score={e.get('score',0):<4} {signals}{enforced}")
    elif args.token:
        print("\nNo evidence returned. Check the token, and that the endpoint is enabled.")
    else:
        print("\nPass --token to see which checks actually fired.")

    print("\nReading it: a 200 means the rung got through. A 403 means blocked.")
    print("An HTML body with __hakaishield in it means challenged - which for a")
    print("scripted client is a stop, since it cannot solve the puzzle.")
    print("In shadow mode everything returns 200 by design; read the evidence")
    print("lines above for what it WOULD have done.\n")


if __name__ == "__main__":
    main()
