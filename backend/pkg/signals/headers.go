package signals

import "net/http"

// fetchMetadataNames are the Sec-Fetch-* request headers that every
// current real browser sends on a navigation (dest, mode, site, and *
// the newer optional user one). A scripted HTTP client that copies a
// browser User-Agent almost never sends any of them — the request
// originates inside an HTTP library, not a tab, so there is no
// navigation context to describe.
var fetchMetadataNames = []string{
	"Sec-Fetch-Dest",
	"Sec-Fetch-Mode",
	"Sec-Fetch-Site",
	"Sec-Fetch-User",
}

// secCHUANames are the "client hint brand" request headers Chrome/Edge
// always attach and Safari/Firefox attach on most platforms. Real
// browsers expose their brand through both Sec-CH-UA and
// Sec-CH-UA-Mobile; scripted clients faking a browser UA skip them.
var secCHUANames = []string{
	"Sec-CH-UA",
	"Sec-CH-UA-Mobile",
}

// HeaderAnomaly flags a request that claims to be a browser (by UA)
// but omits the header family every current real browser sends on a
// navigation. It is deliberately never a block on its own — a page
// fetched inside a real browser's devtools-driven fetch, a privacy
// browser, or any unusual-but-real client can drop one of these, so the
// result is a scoring signal (ROADMAP item 8's consistency layer), not
// a decision. A UA that isn't claiming to be a browser, or that openly
// names itself a crawler, is exempt — there is no claimed identity to
// hold to its headers (fail open, CLAUDE.md Section 8).
func HeaderAnomaly(ua string, header http.Header) bool {
	if !claimsBrowser(ua) {
		return false
	}
	// Every real navigation must send at least one fetch-metadata header
	// OR a client-hint brand. Missing all of them from a browser-claiming
	// UA is the anomaly.
	for _, n := range fetchMetadataNames {
		if header.Get(n) != "" {
			return false
		}
	}
	for _, n := range secCHUANames {
		if header.Get(n) != "" {
			return false
		}
	}
	return true
}
