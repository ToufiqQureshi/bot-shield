from patchright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(headless=False)
    context = browser.new_context(ignore_https_errors=True)
    page = context.new_page()
    page.goto('http://127.0.0.1:5002/')
    page.wait_for_timeout(2000)
    browser.close()
