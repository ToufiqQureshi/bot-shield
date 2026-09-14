package proxy

import (
	"fmt"

	"github.com/wi1dcard/fingerproxy/pkg/ja4"
)

// ja4 turns a raw TLS ClientHello record into its JA4 hash — a
// fingerprint of the TLS client's cipher/extension order that stays
// stable across IPs and survives basic User-Agent spoofing. Plain
// scripted HTTP clients (raw requests/curl, unconfigured libraries)
// produce a JA4 that doesn't match a real browser, which is exactly
// the naive-bot signal ROADMAP.md item 2 asks for. We don't parse
// TLS ourselves (per DECISIONS.md "don't reinvent TLS parsing") —
// this wraps fingerproxy's already-correct implementation.
func ja4Fingerprint(clientHello []byte) (string, error) {
	fp := &ja4.JA4Fingerprint{}
	if err := fp.UnmarshalBytes(clientHello, 't'); err != nil {
		return "", fmt.Errorf("proxy: parse ja4: %w", err)
	}
	return fp.String(), nil
}
