from patchright.sync_api import sync_playwright

with sync_playwright() as p:
    for i in range(5):
        print(f"\n--- Iteration {i+1} of 5 ---")
        browser = p.chromium.launch(headless=False)
        context = browser.new_context(ignore_https_errors=True)
        page = context.new_page()
        
        print("Navigating to https://127.0.0.1:8443/ ...")
        response = page.goto('https://127.0.0.1:8443/')
        
        print(f"Status Code: {response.status}")
        print(f"Page Title: {page.title()}")
        print("Waiting for 15 seconds...")
        
        page.wait_for_timeout(15000)
        
        # After waiting, check title again as it might have bypassed the challenge
        print(f"Page Title after wait: {page.title()}")
        page.screenshot(path=f'example-{p.chromium.name}-{i+1}.png')
        print(f"Screenshot saved as example-{p.chromium.name}-{i+1}.png")
        
        browser.close()
        print(f"Closed instance {i+1}")
