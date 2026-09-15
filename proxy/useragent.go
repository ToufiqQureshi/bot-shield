package proxy

import "strings"

// browserMarkers only appear in a real browser's own User-Agent.
var browserMarkers = []string{"Chrome/", "Firefox/", "Safari/", "Edg/"}

// crawlerMarkers mark a User-Agent that openly admits it's a bot
// (Googlebot, etc). That's not a lie, so it's not a mismatch target.
var crawlerMarkers = []string{"bot", "spider", "crawl"}

// claimsBrowser reports whether ua claims to be a real browser,
// rather than a script or a bot that's already honest about itself.
func claimsBrowser(ua string) bool {
	lower := strings.ToLower(ua)
	for _, m := range crawlerMarkers {
		if strings.Contains(lower, m) {
			return false
		}
	}
	for _, m := range browserMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}

// UAMismatch flags a request that claims to be a browser but whose
// TLS handshake says otherwise. Real, current browsers only speak
// TLS 1.2/1.3 and never trigger the record-fragmentation evasion
// (JA4Unreadable) — so either one, from something calling itself
// Chrome or Firefox, means the User-Agent is faked.
func UAMismatch(ua, ja4 string) bool {
	if !claimsBrowser(ua) {
		return false // not claiming to be a browser, nothing to catch
	}
	if ja4 == "" {
		return false // no TLS to compare against - fail open
	}
	if ja4 == JA4Unreadable {
		return true
	}
	if len(ja4) < 3 {
		return false
	}
	version := ja4[1:3] // e.g. "13" in "t13d1516h2_..."
	return version == "10" || version == "11"
}
