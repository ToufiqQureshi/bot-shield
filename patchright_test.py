import asyncio
import time
from patchright.async_api import async_playwright

BASE_URL = "http://localhost:8080"

async def test_scenario_1_stealth_bypass(p):
    print("\n" + "=" * 70)
    print(" 🔴 [SCENARIO 1] Stealth Headful Browser Initial Access Test")
    print(" Goal: Test if Patchright stealth patches can sneak past challenge probe")
    print("=" * 70)

    browser = await p.chromium.launch(
        headless=False,
        args=["--start-maximized", "--disable-blink-features=AutomationControlled"]
    )
    context = await browser.new_context(no_viewport=True)
    page = await context.new_page()

    try:
        print("[1.1] Navigating to http://localhost:8080/ ...")
        resp = await page.goto(f"{BASE_URL}/", timeout=15000)
        print(f"      Initial HTTP Status: {resp.status if resp else 'None'}")

        print("[1.2] Waiting 4 seconds for in-browser anti-automation probes...")
        await asyncio.sleep(4)

        cookies = await context.cookies()
        passed = any(c['name'] == 'X-BotShield-Passed' for c in cookies)
        body = await page.evaluate("document.body.textContent")
        
        print(f"      Passed Cookie: {passed}")
        print(f"      Page Text    : {body.strip()[:60]}...")
        if "Verification failed" in body or not passed:
            print("  🛡️ RESULT: SUCCESS! Stealth probe caught the automation framework.")
        else:
            print("  ⚠️ RESULT: Reached destination.")
    except Exception as e:
        print(f"      Encountered: {e}")
    finally:
        await browser.close()


async def test_scenario_2_human_mouse_simulation(p):
    print("\n" + "=" * 70)
    print(" 🔴 [SCENARIO 2] Human Interaction Simulation (Mouse curves + Scroll)")
    print(" Goal: Test if simulating human mouse movement and scrolling fools BotShield")
    print("=" * 70)

    browser = await p.chromium.launch(headless=False, args=["--start-maximized"])
    context = await browser.new_context(no_viewport=True)
    page = await context.new_page()

    try:
        print("[2.1] Navigating to challenge page...")
        await page.goto(f"{BASE_URL}/", timeout=15000)

        print("[2.2] Simulating natural human mouse movement & scrolling...")
        for x, y in [(100, 100), (250, 300), (500, 400), (300, 200), (150, 450)]:
            await page.mouse.move(x, y, steps=10)
            await asyncio.sleep(0.3)
        await page.mouse.wheel(0, 300)
        await asyncio.sleep(0.5)

        cookies = await context.cookies()
        passed = any(c['name'] == 'X-BotShield-Passed' for c in cookies)
        body = await page.evaluate("document.body.textContent")
        
        print(f"      Passed Cookie: {passed}")
        print(f"      Page Text    : {body.strip()[:60]}...")
        if not passed or "Verification failed" in body:
            print("  🛡️ RESULT: SUCCESS! BotShield is not fooled by fake mouse curves.")
        else:
            print("  ⚠️ RESULT: Passed.")
    except Exception as e:
        print(f"      Encountered: {e}")
    finally:
        await browser.close()


async def test_scenario_3_multitab_crawling(p):
    print("\n" + "=" * 70)
    print(" 🔴 [SCENARIO 3] Multi-Tab Concurrent Scraper (5 Tabs Parallel Crawl)")
    print(" Goal: Test crawling pattern and rapid navigation across product paths")
    print("=" * 70)

    browser = await p.chromium.launch(headless=False)
    context = await browser.new_context()

    try:
        urls = [f"{BASE_URL}/products/item-{i}" for i in range(1, 6)]
        print(f"[3.1] Opening {len(urls)} parallel tabs to scrape product pages...")

        async def fetch_tab(url, idx):
            page = await context.new_page()
            try:
                print(f"      Tab {idx} loading {url}...")
                resp = await page.goto(url, timeout=10000)
                await asyncio.sleep(2)
                body = await page.evaluate("document.body.textContent")
                return f"Tab {idx}: HTTP {resp.status} - {body.strip()[:30]}"
            except Exception as e:
                return f"Tab {idx} Error: {e}"
            finally:
                await page.close()

        results = await asyncio.gather(*(fetch_tab(url, i+1) for i, url in enumerate(urls)))
        print("[3.2] Multi-Tab Crawl Results:")
        for res in results:
            print(f"      • {res}")
    finally:
        await browser.close()


async def test_scenario_4_in_browser_api_flood(p):
    print("\n" + "=" * 70)
    print(" 🔴 [SCENARIO 4] In-Browser Rapid API Extraction (XHR / Fetch Flood)")
    print(" Goal: Test if scraper using browser fetch() to scrape API gets rate-limited")
    print("=" * 70)

    browser = await p.chromium.launch(headless=False)
    context = await browser.new_context()
    page = await context.new_page()

    try:
        print("[4.1] Opening browser and running 25 rapid in-page API fetches...")
        await page.goto(f"{BASE_URL}/", timeout=10000)
        
        # Execute 25 fetches in the browser
        script = """
        async () => {
            let results = [];
            for (let i = 0; i < 25; i++) {
                try {
                    let r = await fetch('/api/pricing');
                    results.push(r.status);
                } catch(e) {
                    results.push(0);
                }
            }
            return results;
        }
        """
        statuses = await page.evaluate(script)
        count_429 = statuses.count(429)
        count_403 = statuses.count(403)
        count_200 = statuses.count(200)

        print(f"[4.2] In-Browser Fetch Results (Total {len(statuses)} requests):")
        print(f"      • HTTP 200 OK           : {count_200}")
        print(f"      • HTTP 429 Rate Limited : {count_429}")
        print(f"      • HTTP 403 Forbidden    : {count_403}")

        if count_429 > 0 or count_403 > 0:
            print("  🛡️ RESULT: SUCCESS! In-browser high-speed API scraper was throttled/blocked.")
        else:
            print(f"  Status distribution: {statuses[:10]}...")
    except Exception as e:
        print(f"      Encountered: {e}")
    finally:
        await browser.close()


async def main():
    print("=" * 70)
    print(" 🛡️ BOT-SHIELD ENTERPRISE HEADFUL ADVERSARIAL TEST SUITE")
    print(" Real-world browser automation attacks using Patchright (Playwright)")
    print("=" * 70)

    async with async_playwright() as p:
        # Scenario 1
        await test_scenario_1_stealth_bypass(p)
        await asyncio.sleep(1)

        # Scenario 2
        await test_scenario_2_human_mouse_simulation(p)
        await asyncio.sleep(1)

        # Scenario 3
        await test_scenario_3_multitab_crawling(p)
        await asyncio.sleep(1)

        # Scenario 4
        await test_scenario_4_in_browser_api_flood(p)

    print("\n" + "=" * 70)
    print(" 🏁 ALL HEADFUL REAL-WORLD SCENARIOS EXECUTED!")
    print("=" * 70)

if __name__ == "__main__":
    asyncio.run(main())
