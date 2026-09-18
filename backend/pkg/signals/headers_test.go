package signals

import "testing"

// This is the honest, real, current-browser contract HeaderAnomaly
// relies on: a real Chrome/Edge/Safari/Firefox navigation request sends
// both a Sec-Fetch-* metadata header and a Sec-CH-UA/Sec-CH-UA-Mobile
// client-hint brand. A scripted HTTP client faking a browser UA almost
// never does. These tests pin that contract so a future change to the
// header set can't silently loosen the check.

func TestHeaderAnomalyRealBrowserNavigation(t *testing.T) {
	// A real Chromium navigation carries accepted response headers,
	// fetch-metadata and the client-hint brand. None of these fire.
	// Keys use HTTP-canonical form, matching what Go's server produces.
	headers := map[string][]string{
		"Accept":          {"text/html,application/xhtml+xml"},
		"Accept-Language": {"en-US,en;q=0.9"},
		"Sec-Ch-Ua":       {`"Chromium";v="120"`},
		"Sec-Fetch-Dest":  {"document"},
		"Sec-Fetch-Mode":  {"navigate"},
		"Sec-Fetch-Site":  {"same-origin"},
	}
	got := HeaderAnomaly("Mozilla/5.0 ... Chrome/120.0.0.0", headers)
	if got {
		t.Fatalf("HeaderAnomaly() = true for a real browser request; want false")
	}
}

func TestHeaderAnomalyScriptedClientFakingBrowser(t *testing.T) {
	// A scripted HTTP client that copies a browser UA but sends none of
	// the headers every real browser sends — the exact evasion shape
	// this signal exists to catch.
	headers := map[string][]string{
		"accept": {"*/*"},
	}
	if !HeaderAnomaly("Mozilla/5.0 ... Chrome/120.0.0.0", headers) {
		t.Fatalf("HeaderAnomaly() = false for a scripted UA-faking client; want true")
	}
}

func TestHeaderAnomalyHonestScriptingTool(t *testing.T) {
	// curl isn't claiming to be a browser, so missing browser headers is
	// expected and must not fire — a browser claim is required for a lie.
	headers := map[string][]string{}
	if HeaderAnomaly("curl/8.6.0", headers) {
		t.Fatalf("HeaderAnomaly() = true for an honest curl client; want false")
	}
}

func TestHeaderAnomalyEmptyUA(t *testing.T) {
	// No browser claim to judge, so no anomaly. Fail open.
	if HeaderAnomaly("", map[string][]string{}) {
		t.Fatalf("HeaderAnomaly() = true for an empty UA; want false")
	}
}

func TestHeaderAnomalyCrawlerExempt(t *testing.T) {
	// Googlebot openly naming itself isn't lying about being a browser.
	headers := map[string][]string{} // no browser headers at all
	if HeaderAnomaly("Mozilla/5.0 Googlebot/2.1", headers) {
		t.Fatalf("HeaderAnomaly() = true for an honest crawler; want false")
	}
}

func TestHeaderAnomalyMissingFetchMetadataFires(t *testing.T) {
	// Claims Chromium but omits the Sec-Fetch-* family that all real
	// navigations carry — a browser-claiming client missing every one of
	// them is not a browser navigation.
	headers := map[string][]string{
		"sec-ch-ua": {`"Chromium";v="120"`},
	}
	if !HeaderAnomaly("Mozilla/5.0 ... Chrome/120.0.0.0", headers) {
		t.Fatalf("HeaderAnomaly() = false for a browser-claim with no Sec-Fetch-*; want true")
	}
}

func TestHeaderAnomalyCaseInsensitiveHeaderLookup(t *testing.T) {
	// Browsers and Go both normalize header names, but the caller may
	// pass mixed-case keys; the lookup must be case-insensitive.
	headers := map[string][]string{
		"Sec-Fetch-Dest": {"document"},
		"Sec-Fetch-Site": {"same-origin"},
	}
	if HeaderAnomaly("Mozilla/5.0 ... Chrome/120.0.0.0", headers) {
		t.Fatalf("HeaderAnomaly() = true despite mixed-case fetch-metadata headers; want false")
	}
}
