package proxy

import (
	"fmt"

	"github.com/wi1dcard/fingerproxy/pkg/ja4"
)

// ja4Fingerprint turns raw TLS handshake bytes into a JA4 ID.
// Why: a bot's HTTP client "shakes hands" differently than a real
// browser, so this ID tells them apart even if the bot fakes its
// User-Agent.
func ja4Fingerprint(clientHello []byte) (string, error) {
	fp := &ja4.JA4Fingerprint{}
	if err := fp.UnmarshalBytes(clientHello, 't'); err != nil {
		return "", fmt.Errorf("proxy: parse ja4: %w", err)
	}
	return fp.String(), nil
}
