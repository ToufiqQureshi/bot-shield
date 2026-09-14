package proxy

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/wi1dcard/fingerproxy/pkg/hack"
)

type ctxKeyConn struct{}

// NewCaptureListener wraps a plain TCP listener so each connection
// keeps a copy of its raw TLS handshake. Go normally throws those
// bytes away as soon as the handshake finishes, and the JA4
// fingerprint can only be built from them.
func NewCaptureListener(inner net.Listener, tlsConfig *tls.Config) net.Listener {
	cfg := tlsConfig.Clone()
	// HTTP/2 fingerprinting isn't built yet, so don't offer h2 — see
	// docs/DECISIONS.md.
	cfg.NextProtos = []string{"http/1.1"}
	return &captureListener{Listener: inner, config: cfg}
}

type captureListener struct {
	net.Listener
	config *tls.Config
}

// Accept hands back a real *tls.Conn without handshaking it here.
// That matters: net/http only recognises a real *tls.Conn, and when
// it does it runs the handshake itself, with its own timeout, error
// handling and connection limits — all of which we would otherwise
// have to rewrite, worse.
func (l *captureListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return tls.Server(hack.NewHijackClientHelloConn(conn), l.config), nil
}

// ConnContext gives each request a way to reach the connection it
// arrived on, which is where the saved handshake lives. Wire this to
// http.Server.ConnContext.
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, ctxKeyConn{}, c)
}

// JA4Unreadable marks a TLS connection whose handshake we could not
// read. A normal client never causes this; a bot splitting its
// handshake across TLS records to dodge fingerprinting does. So it is
// a signal to score later, and must never look the same as a plain
// HTTP request that simply has no fingerprint.
const JA4Unreadable = "unreadable"

// JA4FromContext returns the JA4 fingerprint of the connection this
// request came in on, JA4Unreadable if it was TLS but we couldn't
// read it, or "" if it wasn't TLS at all. None of these block a
// request on their own — that's the scoring layer's job.
func JA4FromContext(ctx context.Context) string {
	conn, _ := ctx.Value(ctxKeyConn{}).(*tls.Conn)
	if conn == nil {
		return "" // plain HTTP, or capture isn't set up
	}
	hijacked, ok := conn.NetConn().(*hack.HijackClientHelloConn)
	if !ok {
		return ""
	}
	raw, err := hijacked.GetClientHello()
	if err != nil {
		return JA4Unreadable
	}
	fp, err := ja4Fingerprint(raw)
	if err != nil {
		return JA4Unreadable
	}
	return fp
}
