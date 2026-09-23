package policy

import (
	"net/url"
	"path"
	"strings"
)

const (
	ClassLogin    = "login"
	ClassAPI      = "api"
	ClassBrowse   = "browse"
	ClassCheckout = "checkout"
	ClassStatic   = "static_asset"
)

func validClass(class string) bool {
	switch class {
	case ClassLogin, ClassAPI, ClassBrowse, ClassCheckout, ClassStatic:
		return true
	}
	return false
}

// NormalizePath rejects ambiguous escape forms and removes dot segments.
// The caller uses the returned path only for policy matching, not proxy routing.
func NormalizePath(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	if !strings.HasPrefix(raw, "/") || strings.Contains(raw, "\\") {
		return "", false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.Contains(decoded, "\\") || strings.Contains(decoded, "//") || strings.ContainsRune(decoded, 0) {
		return "", false
	}
	if strings.Contains(decoded, "%") {
		return "", false
	}
	return path.Clean(decoded), true
}

// Classify uses fixed, reviewable defaults. Unknown routes are browse.
func Classify(normalizedPath, method string) string {
	p := strings.ToLower(normalizedPath)
	switch {
	case p == "/login" || p == "/signin" || p == "/sign-in" || strings.HasPrefix(p, "/auth/"):
		return ClassLogin
	case p == "/checkout" || strings.HasPrefix(p, "/checkout/") || p == "/cart" || strings.HasPrefix(p, "/payment/"):
		return ClassCheckout
	case p == "/api" || strings.HasPrefix(p, "/api/"):
		return ClassAPI
	case method == "GET" || method == "HEAD":
		switch strings.ToLower(path.Ext(p)) {
		case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".webp", ".avif":
			return ClassStatic
		}
	}
	return ClassBrowse
}
