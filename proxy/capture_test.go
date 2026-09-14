package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

// selfSignedCert makes a throwaway TLS certificate so the test can
// run a real TLS handshake without needing a real domain's cert.
func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// A real client, over a real TLS handshake, through our capture
// listener, should reach the origin with a non-empty JA4 header set —
// this is the actual end-to-end path bot-shield will run in
// production, not just the fingerprint math in isolation.
func TestCaptureListenerEndToEnd(t *testing.T) {
	gotJA4 := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJA4 <- r.Header.Get(ja4Header)
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := rawLn.Addr().String()
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	ln := NewCaptureListener(rawLn, tlsConfig)

	srv := &http.Server{Handler: p, ConnContext: ConnContext}
	go srv.Serve(ln)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://" + addr + "/hello")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("got body %q, want %q", body, "ok")
	}

	// Checking the value actually looks like a JA4, not just that
	// something was set — a wrong or garbage fingerprint is worse than
	// none, because the scoring layer would trust it.
	select {
	case got := <-gotJA4:
		if !ja4Shape.MatchString(got) {
			t.Errorf("origin got %s = %q, want a JA4 like t13d1516h2_8daaf6152771_e5627efa2ab1", ja4Header, got)
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
	}
}

// What a real JA4 looks like: t13d1516h2_8daaf6152771_e5627efa2ab1 —
// protocol, TLS version, SNI flag, cipher/extension counts and ALPN,
// then two 12-character hashes.
var ja4Shape = regexp.MustCompile(`^[tq]\d{2}[di]\d{4}[a-z0-9]{2}_[0-9a-f]{12}_[0-9a-f]{12}$`)

// A client that opens a connection and then never sends a ClientHello
// at all (unlike the garbage-bytes case below, which fails fast) must
// still be dropped eventually, not held open forever — that's what
// handshakeTimeout is for. A slow/silent client is a normal thing for
// a bot-detection proxy to see on purpose.
func TestCaptureListenerTimesOutSlowHandshake(t *testing.T) {
	old := handshakeTimeout.Load()
	handshakeTimeout.Store(int64(200 * time.Millisecond))
	defer handshakeTimeout.Store(old)

	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := rawLn.Addr().String()
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	ln := NewCaptureListener(rawLn, tlsConfig)
	defer ln.Close()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// connect but never send anything - simulates a stalled/slow client

	// The read deadline is deliberately much longer than the handshake
	// timeout. If the server never closes the connection, this Read
	// ends in *our own* deadline instead of EOF — that difference is
	// the only thing separating "the timeout works" from "the test got
	// bored waiting", so check which one actually happened.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	_, err = conn.Read(make([]byte, 1))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("connection stayed open, want it closed after the handshake timeout")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("read hit its own %s deadline: the server never closed the connection, so the handshake timeout did not fire", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("connection closed after %s, want it closed near the 200ms handshake timeout", elapsed)
	}
}

// fakeFlakyListener returns one fake error from Accept before behaving
// normally, so tests can check the capture listener survives a
// transient accept error instead of dying silently.
type fakeFlakyListener struct {
	net.Listener
	failedOnce bool
}

func (l *fakeFlakyListener) Accept() (net.Conn, error) {
	if !l.failedOnce {
		l.failedOnce = true
		return nil, errors.New("fake transient accept error")
	}
	return l.Listener.Accept()
}

// A one-off Accept error (e.g. the OS briefly hit a file-descriptor
// limit) must not permanently kill the listener - real deployments
// hit transient errors like this, and one hiccup silently taking down
// every future visitor's connection would be worse than the error
// itself.
func TestCaptureListenerSurvivesTransientAcceptError(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := rawLn.Addr().String()
	flaky := &fakeFlakyListener{Listener: rawLn}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	ln := NewCaptureListener(flaky, tlsConfig)
	defer ln.Close()

	// the listener must still accept a real connection after the
	// fake transient error above
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("dial after transient accept error: %v", err)
	}
	conn.Close()
}

// A connection that never TLS-handshakes correctly (a port scanner, a
// plain-HTTP client hitting the TLS port) must not hang or crash the
// listener — it just gets dropped, and the listener keeps working for
// everyone else.
func TestCaptureListenerDropsBadHandshake(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := rawLn.Addr().String()
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	ln := NewCaptureListener(rawLn, tlsConfig)
	defer ln.Close()

	// send garbage instead of a TLS ClientHello
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("not a tls handshake"))
	conn.Close()

	// the listener must still accept a real, well-behaved connection
	// afterwards
	good, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("dial after bad handshake: %v", err)
	}
	good.Close()
}
