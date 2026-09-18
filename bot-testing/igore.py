from patchright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(ignore_https_errors=True)
    page = context.new_page()
    print("Navigating to https://127.0.0.1:8443/ ...")
    response = page.goto('https://127.0.0.1:8443/')
    print(f"Status Code: {response.status}")
    print(f"Page Title: {page.title()}")
    print("Waiting for 10 seconds...")
    page.wait_for_timeout(10000)
    page.screenshot(path=f'example-{p.chromium.name}.png')
    print("Screenshot saved.")
    browser.close()
