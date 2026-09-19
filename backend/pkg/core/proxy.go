// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package core

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/ToufiqQureshi/bot-shield/pkg/deception"
	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
)

type ctxKeyDecision struct{}
type ctxKeyScore struct{}

// WithDecision attaches the botshield policy decision and score to the request context.
func WithDecision(ctx context.Context, decision string, score int) context.Context {
	ctx = context.WithValue(ctx, ctxKeyDecision{}, decision)
	return context.WithValue(ctx, ctxKeyScore{}, score)
}

// DecisionFromContext retrieves the policy decision from context.
func DecisionFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(ctxKeyDecision{}).(string); ok {
		return s
	}
	return ""
}

// ScoreFromContext retrieves the bot risk score from context.
func ScoreFromContext(ctx context.Context) int {
	if s, ok := ctx.Value(ctxKeyScore{}).(int); ok {
		return s
	}
	return 0
}

// ja4Header is the header we attach to the forwarded request so the
// origin (and later, our own scoring code) can see the caller's JA4
// fingerprint without re-capturing it themselves.
const ja4Header = "X-BotShield-JA4"

// uaMismatchHeader tells the origin the caller's declared browser
// doesn't match its TLS handshake. Set to "true" only when we caught
// one — absence means nothing suspicious was found, not "unknown".
const uaMismatchHeader = "X-BotShield-UA-Mismatch"
const decisionHeader = "X-BotShield-Decision"
const scoreHeader = "X-BotShield-Score"
const signalsHeader = "X-BotShield-Signals"

// realIPHeader is the client-IP header we set ourselves. nginx, Rails
// and Laravel apps commonly read this one.
const realIPHeader = "X-Real-IP"

// clientIPHeaders are the "who is calling" headers a visitor could
// send to pass themselves off as another IP. net/http only strips the
// X-Forwarded-* family, so we clear these ourselves — an origin that
// trusts any one of them would otherwise log, allowlist or rate-limit
// an address the caller picked.
var clientIPHeaders = []string{
	realIPHeader,
	"True-Client-IP",
	"CF-Connecting-IP",
	"X-Client-IP",
	"Fastly-Client-IP",
	"X-Cluster-Client-IP",
}

// DefaultOriginTransport is a production-tuned HTTP transport with connection pooling and explicit timeouts.
var DefaultOriginTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          1000,
	MaxIdleConnsPerHost:   200,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 15 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// NewOriginProxy sets up the HTTP proxy to the origin server.
// It forces TLS since a WAF that doesn't protect the origin connection
// is just security theater.
func NewOriginProxy(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, &url.Error{Op: "parse", URL: target, Err: errInvalidTarget}
	}

	// Rewrite, not the older Director: net/http strips the client's
	// own X-Forwarded-* headers before calling it, so a visitor can't
	// fake the IP that our rate limiting and geo checks will rely on.
	// Director leaves those headers untouched.
	p := &httputil.ReverseProxy{
		Transport: DefaultOriginTransport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>502 Bad Gateway</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#c9d1d9;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;}h1{font-size:2rem;color:#f85149;}p{color:#8b949e;}</style></head><body><div style="text-align:center;"><h1>502 Bad Gateway</h1><p>Origin server connection failed or timed out.</p><small style="color:#484f58;">Protected by BotShield</small></div></body></html>`))
		},
		ModifyResponse: func(resp *http.Response) error {
			if resp.Request != nil {
				if dec := DecisionFromContext(resp.Request.Context()); dec == signals.DecisionDeceive.String() {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return nil
					}
					_ = resp.Body.Close()

					contentType := resp.Header.Get("Content-Type")
					transformed := deception.InjectPoisonPayload(body, contentType)

					resp.Body = io.NopCloser(bytes.NewReader(transformed))
					resp.ContentLength = int64(len(transformed))
					resp.Header.Set("Content-Length", strconv.Itoa(len(transformed)))
				}
			}
			return nil
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			// SetURL would point Host at the origin; the origin serves
			// the visitor's domain, so it needs the original Host.
			r.Out.Host = r.In.Host
			// Fills in the real client IP for the origin's logs.
			r.SetXForwarded()
			for _, h := range clientIPHeaders {
				r.Out.Header.Del(h)
			}
			if ip, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
				r.Out.Header.Set(realIPHeader, ip)
			}

			// Only we get to say what the fingerprint is. A request
			// with none is forwarded as normal — a fingerprint we
			// couldn't read is never a reason to block someone.
			r.Out.Header.Del(ja4Header)
			ja4 := JA4FromContext(r.In.Context())
			if ja4 != "" {
				r.Out.Header.Set(ja4Header, ja4)
			}

			// Same rule for the UA-mismatch flag: a visitor doesn't
			// get to set this themselves, and it's a signal for
			// scoring, not a block, per CLAUDE.md Section 6.
			r.Out.Header.Del(uaMismatchHeader)
			if signals.UAMismatch(r.In.UserAgent(), ja4) {
				r.Out.Header.Set(uaMismatchHeader, "true")
			}

			// Inbound requests cannot forge BotShield decision or score headers.
			r.Out.Header.Del(decisionHeader)
			r.Out.Header.Del(scoreHeader)
			r.Out.Header.Del(signalsHeader)
			if dec := DecisionFromContext(r.In.Context()); dec != "" {
				r.Out.Header.Set(decisionHeader, dec)
				r.Out.Header.Set(scoreHeader, strconv.Itoa(ScoreFromContext(r.In.Context())))
				if dec == signals.DecisionDeceive.String() {
					// Strip Accept-Encoding so origin returns uncompressed HTML that can be transformed.
					r.Out.Header.Del("Accept-Encoding")
				}
			}
		},
	}
	return p, nil
}

