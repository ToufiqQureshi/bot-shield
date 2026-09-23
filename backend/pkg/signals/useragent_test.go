package signals

import "testing"

func TestUAMismatch(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		ja4  string
		want bool
	}{
		{
			name: "real Chrome on modern TLS",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			ja4:  "t13d1516h2_8daaf6152771_e5627efa2ab1",
			want: false,
		},
		{
			name: "claims Chrome but handshake is unreadable (fragmentation evasion)",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36",
			ja4:  JA4Unreadable,
			want: true,
		},
		{
			name: "claims Firefox but negotiates ancient TLS 1.0",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Firefox/120.0",
			ja4:  "t10d1516h2_8daaf6152771_e5627efa2ab1",
			want: true,
		},
		{
			name: "claims Safari but negotiates TLS 1.1",
			ua:   "Mozilla/5.0 (Macintosh) AppleWebKit/605.1.15 Safari/605.1.15",
			ja4:  "t11d1516h2_8daaf6152771_e5627efa2ab1",
			want: true,
		},
		{
			name: "curl is honest about being curl - nothing to catch",
			ua:   "curl/8.6.0",
			ja4:  "t10d1516h2_8daaf6152771_e5627efa2ab1",
			want: false,
		},
		{
			name: "Googlebot openly declares itself, even with 'Chrome' in the string",
			ua:   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html) Chrome/120.0.0.0",
			ja4:  "t10d1516h2_8daaf6152771_e5627efa2ab1",
			want: false,
		},
		{
			name: "no JA4 at all (plain HTTP) - nothing to compare, fail open",
			ua:   "Mozilla/5.0 Chrome/120.0.0.0",
			ja4:  "",
			want: false,
		},
		{
			name: "empty User-Agent",
			ua:   "",
			ja4:  "t13d1516h2_8daaf6152771_e5627efa2ab1",
			want: false,
		},
		{
			name: "garbage JA4 too short to read a version from",
			ua:   "Mozilla/5.0 Chrome/120.0.0.0",
			ja4:  "t1",
			want: false,
		},
		{
			name: "claims Chrome but JA4 is python-requests scraper",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36",
			ja4:  "t12d190800_4464c1bd5eb7_b3394627b738",
			want: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UAMismatch(c.ua, c.ja4)
			if got != c.want {
				t.Errorf("UAMismatch(%q, %q) = %v, want %v", c.ua, c.ja4, got, c.want)
			}
		})
	}
}

func TestIsScriptingTool_CatchesKnownTools(t *testing.T) {
	uas := []string{
		"curl/8.6.0",
		"Wget/1.21.3",
		"python-requests/2.32.5",
		"python-urllib3/2.0",
		"python-httpx/0.27.0",
		"Python/3.11 aiohttp/3.9.0",
		"Go-http-client/1.1",
		"go-resty/2.11.0",
		"okhttp/4.12.0",
		"Java/17.0.1",
		"Apache-HttpClient/4.5.13",
		"node-fetch/3.3.2",
		"axios/1.6.0",
		"libwww-perl/6.72",
		"GuzzleHttp/7.8",
		"PostmanRuntime/7.36.0",
		"insomnia/2023.5.8",
		"Mozilla/5.0 Locust/2.20.0",
		"Apache-HttpClient/4.5 (Java) - JMeter",
		"HeadlessChrome/120.0.0.0",
		"Mozilla/5.0 (compatible; PhantomJS/2.1.1)",
		"selenium/4.16.0 (python webdriver)",
		"Mozilla/5.0 Playwright/1.40.0",
		"Mozilla/5.0 patchright/1.0",
		"Mozilla/5.0 (compatible; Puppeteer/21.0.0)",
		"Scrapy/2.11.0 (+https://scrapy.org)",
		"nmap scripting engine",
		"sqlmap/1.7.11",
		"Nuclei - Open-source project (github.com/projectdiscovery/nuclei)",
		"Mozilla/5.0 (compatible; Nessus)",
	}
	for _, ua := range uas {
		t.Run(ua, func(t *testing.T) {
			if !IsScriptingTool(ua) {
				t.Errorf("IsScriptingTool(%q) = false, want true", ua)
			}
		})
	}
}

// A real browser's UA must never trip this check — it is one of the two
// signals allowed to score 100 alone (CLAUDE.md Section 6/14), so a
// false positive here hard-blocks every visitor on that browser.
func TestIsScriptingTool_RealBrowsersNeverMatch(t *testing.T) {
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	}
	for _, ua := range uas {
		t.Run(ua, func(t *testing.T) {
			if IsScriptingTool(ua) {
				t.Errorf("IsScriptingTool(%q) = true, want false — this would hard-block a real browser", ua)
			}
		})
	}
}
