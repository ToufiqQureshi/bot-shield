import json
from flask import Flask, request

app = Flask(__name__)

@app.route("/")
def index():
    return """
    <script>
    const data = {
        webdriver: navigator.webdriver,
        webdriver_type: typeof navigator.webdriver,
        webdriver_desc: Object.getOwnPropertyDescriptor(navigator, 'webdriver') ? true : false,
        webdriver_proto: navigator.__proto__.hasOwnProperty('webdriver'),
        plugins: navigator.plugins.length,
        languages: navigator.languages,
        ua: navigator.userAgent,
        outerWidth: window.outerWidth,
        outerHeight: window.outerHeight,
        playwright: typeof window.__playwright,
        chrome: typeof window.chrome,
        chrome_runtime: window.chrome ? typeof window.chrome.runtime : 'none',
        permissions: navigator.permissions ? typeof navigator.permissions.query : 'none',
        window_keys: Object.keys(window).filter(k => k.startsWith('_')),
    };
    fetch("/report", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(data)
    });
    </script>
    """

@app.route("/report", methods=["POST"])
def report():
    with open("report.json", "w") as f:
        json.dump(request.json, f, indent=2)
    return "ok"

if __name__ == "__main__":
    app.run(port=5002)
