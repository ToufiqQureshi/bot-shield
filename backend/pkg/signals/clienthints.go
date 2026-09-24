package signals

import (
	"regexp"
	"strconv"
	"strings"
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
	uaMatch := chromiumUAMajor.FindStringSubmatch(f.UA)
	if len(uaMatch) != 2 {
		return nil
	}
	uaMajor, _ := strconv.Atoi(uaMatch[1])
	if uaMajor == 0 {
		return nil
	}
	var found []string
	if hint := f.Header.Get("Sec-CH-UA"); len(hint) <= 512 {
		for _, match := range chromiumHintMajor.FindAllStringSubmatch(hint, 4) {
			major, _ := strconv.Atoi(match[1])
			if major != uaMajor {
				found = append(found, "client_hint_major_mismatch")
				break
			}
		}
	}
	if hint := f.Header.Get("Sec-CH-UA-Platform"); len(hint) <= 64 {
		claimed := chromiumUAPlatform(f.UA)
		observed := normalizeHintPlatform(hint)
		if claimed != "" && observed != "" && claimed != observed {
			found = append(found, "client_hint_platform_mismatch")
		}
	}
	if mobile := f.Header.Get("Sec-CH-UA-Mobile"); len(mobile) <= 4 {
		switch mobile {
		case "?1":
			if p := chromiumUAPlatform(f.UA); p == "windows" || p == "macos" || p == "chromeos" || p == "linux" {
				found = append(found, "client_hint_mobile_mismatch")
			}
		case "?0":
			if strings.Contains(f.UA, "Mobile") || strings.Contains(f.UA, "iPhone") || strings.Contains(f.UA, "iPod") {
				found = append(found, "client_hint_mobile_mismatch")
			}
		}
	}
	return found
}

func chromiumUAPlatform(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT"):
		return "windows"
	case strings.Contains(ua, "Android"):
		return "android"
	case strings.Contains(ua, "CrOS"):
		return "chromeos"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		return "ios"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return "macos"
	case strings.Contains(ua, "Linux"):
		return "linux"
	default:
		return ""
	}
}

func normalizeHintPlatform(hint string) string {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(hint), `"`)) {
	case "windows":
		return "windows"
	case "android":
		return "android"
	case "chrome os":
		return "chromeos"
	case "ios":
		return "ios"
	case "macos":
		return "macos"
	case "linux":
		return "linux"
	default:
		return ""
	}
}
