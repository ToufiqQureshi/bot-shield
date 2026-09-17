"""
locustfile.py - Brutal load test for Bot-Shield proxy.

Usage:
    locust --headless -u 200 -r 20 --run-time 60s --host https://localhost:8443

Scenarios:
    - NormalBrowser   : Clean traffic (expect 200 pass-through)
    - BotScript       : Python-urllib UA (suspicious, may trigger challenge)  
    - HeavyConcurrent : Hammer the dashboard stats API too
"""
from locust import HttpUser, task, between, constant_throughput
import random
import string
import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

TENANT = "default"
EVIDENCE_TOKEN = ""  # Set this if you run with -evidence-token


class BaseUser(HttpUser):
    abstract = True
    def on_start(self):
        self.client.verify = False


def rand_path():
    return "/" + "".join(random.choices(string.ascii_lowercase, k=8))


class NormalBrowserUser(BaseUser):
    """Simulates real browser traffic hitting the proxy."""
    wait_time = between(0.01, 0.05)  # 20-100 req/s per user

    @task(10)
    def browse(self):
        self.client.get("/", headers={
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        })

    @task(3)
    def browse_random_path(self):
        self.client.get(rand_path(), headers={
            "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
        })

    @task(1)
    def dashboard_stats(self):
        """Read the public stats endpoint - no auth needed."""
        self.client.get(f"/api/v1/dashboard/stats?tenant={TENANT}")


class BotLikeUser(BaseUser):
    """Simulates bot/scraper traffic - should trigger signals."""
    wait_time = constant_throughput(50)  # 50 req/s constant

    @task(5)
    def scrape_plain_python(self):
        self.client.get("/", headers={
            "User-Agent": "Python-urllib/3.9",
        })

    @task(3)
    def scrape_curl(self):
        self.client.get(rand_path(), headers={
            "User-Agent": "curl/7.68.0",
        })

    @task(2)
    def hammer_api(self):
        self.client.get(f"/api/v1/dashboard/stats?tenant={TENANT}")


class ChaosUser(BaseUser):
    """Sends malformed / edge-case requests to expose crashes."""
    wait_time = between(0.1, 0.5)

    @task(3)
    def giant_path(self):
        self.client.get("/" + "A" * 8000)

    @task(3)
    def empty_ua(self):
        self.client.get("/", headers={"User-Agent": ""})

    @task(2)
    def null_tenant(self):
        self.client.get("/api/v1/dashboard/stats?tenant=")

    @task(2)
    def unknown_tenant(self):
        self.client.get("/api/v1/dashboard/stats?tenant=doesnotexist")

    @task(1)
    def huge_header(self):
        self.client.get("/", headers={
            "X-Forwarded-For": ".".join(["999"] * 300),
            "User-Agent": "Mozilla/5.0",
        })

    @task(1)
    def method_override(self):
        self.client.post("/api/v1/dashboard/stats?tenant=default")
