package proxy

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/wi1dcard/fingerproxy/pkg/hack"
)

// Known limit: HTTP/2 fingerprinting isn't built yet, so we only
// offer HTTP/1.1 here. See docs/DECISIONS.md.
var captureNextProtos = []string{"http/1.1"}

// maxHandshakes limits how many TLS handshakes can run at once, so a
// flood of connections can't spawn unlimited goroutines.
const maxHandshakes = 1000

type ctxKeyJA4 struct{}

// JA4FromContext returns the JA4 fingerprint captured for this
// request's connection, or "" if none was captured. "" must be
// treated as "unknown", never as "this is a bot" — a broken
// fingerprint should never break the site (fail open).
func JA4FromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyJA4{}).(string)
	return v
}

// ConnContext hands each HTTP request the JA4 fingerprint of the
// connection it came in on. Wire this to http.Server.ConnContext.
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	if cc, ok := c.(*capturedConn); ok {
		return context.WithValue(ctx, ctxKeyJA4{}, cc.ja4)
	}
	return ctx
}

// capturedConn is a finished TLS connection plus the JA4 fingerprint
// read from it. ConnContext reads the fingerprint back off of it.
type capturedConn struct {
	net.Conn
	ja4 string
}

// NewCaptureListener wraps a plain TCP listener so every connection
// gets a TLS handshake and its JA4 fingerprint read before the HTTP
// server sees it. Why here: Go's normal TLS listener throws the raw
// handshake bytes away right after the handshake, and JA4 needs
// those bytes — this is the only point they're still available.
func NewCaptureListener(inner net.Listener, tlsConfig *tls.Config) net.Listener {
	cfg := tlsConfig.Clone()
	cfg.NextProtos = captureNextProtos

	out := hack.NewChannelListener(context.Background())
	sem := make(chan struct{}, maxHandshakes)

	go func() {
		for {
			conn, err := inner.Accept()
			if err != nil {
				return
			}
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				handshakeAndCapture(conn, cfg, out)
			}()
		}
	}()

	return out
}

// handshakeAndCapture does the TLS handshake for one connection, then
// reads its JA4 fingerprint and hands the finished connection to the
// HTTP server. Runs in its own goroutine per connection so one
// slow or malicious client can't block anyone else's request.
func handshakeAndCapture(conn net.Conn, cfg *tls.Config, out *hack.ChannelListener) {
	hijacked := hack.NewHijackClientHelloConn(conn)
	tlsConn := tls.Server(hijacked, cfg)

	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return
	}

	ja4 := ""
	if raw, err := hijacked.GetClientHello(); err == nil {
		if fp, err := ja4Fingerprint(raw); err == nil {
			ja4 = fp
		}
	}

	out.SendToChannel(&capturedConn{Conn: tlsConn, ja4: ja4})
}
