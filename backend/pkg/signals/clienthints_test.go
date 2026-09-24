package signals

import (
	"net/http"
	"slices"
	"testing"
)

func TestShadowSignalsClientHintVersionMismatch(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		hint string
		want bool
	}{
		{"matching chrome", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", `"Chromium";v="120", "Google Chrome";v="120"`, false},
		{"forged chrome major", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", `"Chromium";v="119", "Google Chrome";v="119"`, true},
		{"edge chromium major", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36 Edg/120.0", `"Chromium";v="119", "Microsoft Edge";v="120"`, true},
		{"missing hint", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", "", false},
		{"grease only", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", `"Not A;Brand";v="99"`, false},
		{"malformed version", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", `"Chromium";v="xx"`, false},
		{"firefox exempt", "Mozilla/5.0 Firefox/120.0", `"Chromium";v="119"`, false},
		{"crawler exempt", "Googlebot Chrome/120.0", `"Chromium";v="119"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("Sec-CH-UA", tc.hint)
			fired := slices.Contains(ShadowSignals(RequestFacts{UA: tc.ua, Header: h}), "client_hint_major_mismatch")
			if fired != tc.want {
				t.Fatalf("shadow mismatch = %v, want %v", fired, tc.want)
			}
		})
	}
}

func TestShadowSignalsGreaseBrandMissing(t *testing.T) {
	const chromeUA = "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36"
	cases := []struct {
		name string
		ua   string
		hint string
		want bool
	}{
		{"real chrome with grease", chromeUA, `"Not/A)Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`, false},
		{"grease alternate spelling", chromeUA, `"Not;A=Brand";v="24", "Chromium";v="120"`, false},
		{"grease dot spelling", chromeUA, `"Not.A/Brand";v="99", "Chromium";v="120"`, false},
		{"fabricated header no grease", chromeUA, `"Chromium";v="120", "Google Chrome";v="120"`, true},
		{"single brand no grease", chromeUA, `"Chromium";v="120"`, true},
		{"missing hint entirely", chromeUA, "", false},
		{"firefox exempt", "Mozilla/5.0 Firefox/120.0", `"Chromium";v="120"`, false},
		{"crawler exempt", "Googlebot Chrome/120.0", `"Chromium";v="120"`, false},
		{"grease only, no real brand", chromeUA, `"Not/A)Brand";v="8"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("Sec-CH-UA", tc.hint)
			fired := slices.Contains(ShadowSignals(RequestFacts{UA: tc.ua, Header: h}), "client_hint_missing_grease_brand")
			if fired != tc.want {
				t.Fatalf("grease-missing = %v, want %v", fired, tc.want)
			}
		})
	}
}

func TestShadowSignalsPlatformAndMobileConsistency(t *testing.T) {
	cases := []struct {
		name, ua, platform, mobile string
		want                       []string
	}{
		{"desktop chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", `"Windows"`, "?0", nil},
		{"android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", `"Android"`, "?1", nil},
		{"platform contradiction", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", `"macOS"`, "?0", []string{"client_hint_platform_mismatch"}},
		{"mobile contradiction", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", `"Windows"`, "?1", []string{"client_hint_mobile_mismatch"}},
		{"both contradictions", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", `"Android"`, "?1", []string{"client_hint_platform_mismatch", "client_hint_mobile_mismatch"}},
		{"tablet neutral", "Mozilla/5.0 (Linux; Android 14; Tablet) Chrome/120.0.0.0", `"Android"`, "?0", nil},
		{"malformed hint neutral", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", `"Unknown"`, "yes", nil},
		{"crawler neutral", "Googlebot Chrome/120.0 (Windows NT 10.0)", `"Android"`, "?1", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("Sec-CH-UA-Platform", tc.platform)
			h.Set("Sec-CH-UA-Mobile", tc.mobile)
			got := ShadowSignals(RequestFacts{UA: tc.ua, Header: h})
			if !slices.Equal(got, tc.want) {
				t.Fatalf("shadow signals = %v, want %v", got, tc.want)
			}
		})
	}
}
