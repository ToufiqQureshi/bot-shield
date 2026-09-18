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
