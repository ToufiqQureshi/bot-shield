package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/wi1dcard/fingerproxy/pkg/hack"
)

// Known limit: HTTP/2 fingerprinting isn't built yet, so we only
// offer HTTP/1.1 here. See docs/DECISIONS.md.
var captureNextProtos = []string{"http/1.1"}

// maxHandshakes limits how many TLS handshakes can run at once, so a
// flood of connections can't spawn unlimited goroutines.
const maxHandshakes = 1000

// handshakeTimeout caps how long one connection's TLS handshake can
// take. Without this, a client that never finishes handshaking would
// hold its goroutine (and its slot in maxHandshakes) forever — a
// small number of such clients would be enough to block everyone
// else from connecting. Stored atomically so a test can safely shrink
// it instead of waiting out the real timeout.
var handshakeTimeout atomic.Int64

func init() {
	handshakeTimeout.Store(int64(10 * time.Second))
}

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
		// backoff handles a transient Accept error (e.g. the process
		// briefly hit its file-descriptor limit) the same way
		// net/http's own server does: wait a little and keep trying,
		// instead of a single hiccup permanently killing the whole
		// listener for every future visitor.
		backoff := time.Millisecond
		for {
			conn, err := inner.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return // listener was closed on purpose (shutdown)
				}
				log.Printf("botshield: accept error: %v, retrying in %s", err, backoff)
				time.Sleep(backoff)
				if backoff < time.Second {
					backoff *= 2
				}
				continue
			}
			backoff = time.Millisecond
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
// HTTP server. Runs in its own goroutine per connection so one slow
// or malicious client can't block anyone else's request.
//
// The recover() here is not optional: this parses attacker-controlled
// bytes (the ClientHello), and Go crashes the *entire process* on an
// unrecovered panic in any goroutine, not just this one connection.
// One bad ClientHello must never be able to take bot-shield, and
// every client behind it, offline.
func handshakeAndCapture(conn net.Conn, cfg *tls.Config, out *hack.ChannelListener) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("botshield: recovered panic while handling connection: %v", r)
			conn.Close()
		}
	}()

	hijacked := hack.NewHijackClientHelloConn(conn)
	tlsConn := tls.Server(hijacked, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(handshakeTimeout.Load()))
	defer cancel()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
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
