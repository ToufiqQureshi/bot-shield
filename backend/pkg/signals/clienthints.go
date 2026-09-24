package signals

import (
	"regexp"
	"strconv"
)

var (
	chromiumUAMajor   = regexp.MustCompile(`(?:Chrome|Chromium)/([0-9]{1,3})`)
	chromiumHintMajor = regexp.MustCompile(`"(?:Chromium|Google Chrome|Microsoft Edge)";v="([0-9]{1,3})"`)
)

// ShadowSignals reports new detection candidates without changing the score.
// Client hints can expose a forged Chromium version, but browser variants and
// privacy tools need real traffic review before this can affect enforcement.
func ShadowSignals(f RequestFacts) []string {
	if !claimsBrowser(f.UA) || len(f.UA) > 512 {
		return nil
	}
	hint := f.Header.Get("Sec-CH-UA")
	if hint == "" || len(hint) > 512 {
		return nil
	}
	uaMatch := chromiumUAMajor.FindStringSubmatch(f.UA)
	if len(uaMatch) != 2 {
		return nil
	}
	uaMajor, _ := strconv.Atoi(uaMatch[1])
	if uaMajor == 0 {
		return nil
	}
	for _, match := range chromiumHintMajor.FindAllStringSubmatch(hint, 4) {
		major, _ := strconv.Atoi(match[1])
		if major != uaMajor {
			return []string{"client_hint_major_mismatch"}
		}
	}
	return nil
}
