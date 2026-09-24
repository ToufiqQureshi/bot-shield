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
