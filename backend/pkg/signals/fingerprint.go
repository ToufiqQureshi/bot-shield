package signals

import (
	"fmt"

	"github.com/wi1dcard/fingerproxy/pkg/ja4"
)

// JA4Unreadable marks a TLS connection whose handshake we could not read.
const JA4Unreadable = "unreadable"

// ParseJA4 takes a raw ClientHello message and constructs the JA4
// fingerprint according to the standard.
// Why: a bot's HTTP client "shakes hands" differently than a real
// browser, so this ID tells them apart even if the bot fakes its
// User-Agent.
func ParseJA4(clientHello []byte) (string, error) {
	fp := &ja4.JA4Fingerprint{}
	if err := fp.UnmarshalBytes(clientHello, 't'); err != nil {
		return "", fmt.Errorf("proxy: parse ja4: %w", err)
	}
	return fp.String(), nil
}
