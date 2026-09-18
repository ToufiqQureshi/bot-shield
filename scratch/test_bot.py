from playwright.sync_api import sync_playwright

def run(playwright):
    browser = playwright.chromium.launch(headless=False)
    # Using ignore_https_errors because we will use a self-signed cert
    context = browser.new_context(ignore_https_errors=True)
    page = context.new_page()

    print("Navigating to Bot-Shield proxy (https://localhost:8443)...")
    try:
        response = page.goto("https://localhost:8443/", wait_until="networkidle")
        print(f"Status: {response.status}")
        
        # We can wait a little to see if the JS challenge redirects us
        page.wait_for_timeout(3000)
        
        print("Final URL after 3 seconds:", page.url)
        content = page.content()
        if "Checking your browser" in content or "Verification failed" in content:
            print("=> Bot was CAUGHT by the JS Challenge!")
        elif "Welcome to the Real Site" in content:
            print("=> Bot bypassed the protection and reached the origin!")
        else:
            print("=> Unexpected response. Body:", content[:200])
            
    except Exception as e:
        print("Error during navigation:", e)
    
    browser.close()

with sync_playwright() as playwright:
    run(playwright)
