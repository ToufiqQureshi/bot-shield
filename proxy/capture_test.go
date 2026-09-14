package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

// What a real JA4 looks like: t13d1516h2_8daaf6152771_e5627efa2ab1 —
// protocol, TLS version, SNI flag, cipher/extension counts and ALPN,
// then two 12-character hashes.
var ja4Shape = regexp.MustCompile(`^[tq]\d{2}[di]\d{4}[a-z0-9]{2}_[0-9a-f]{12}_[0-9a-f]{12}$`)

// quietLogger keeps the expected TLS handshake errors in these tests
// (deliberately broken handshakes) out of the test output.
func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

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

// startCapture runs the real thing a client would hit: a TLS
// capture listener, an http.Server configured the way cmd/botshield
// configures it, and the reverse proxy pointing at origin.
func startCapture(t *testing.T, origin string, readHeaderTimeout time.Duration) (addr string) {
	t.Helper()

	p, err := New(origin)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = rawLn.Addr().String()

	cfg := &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}}
	srv := &http.Server{
		Handler:           p,
		ConnContext:       ConnContext,
		ReadHeaderTimeout: readHeaderTimeout,
		ErrorLog:          quietLogger(),
	}
	go srv.Serve(NewCaptureListener(rawLn, cfg))
	t.Cleanup(func() { srv.Close() })

	return addr
}

// A real client, over a real TLS handshake, through the capture
// listener, should reach the origin with its JA4 attached. This is
// the whole path bot-shield runs in production, not the fingerprint
// maths in isolation.
func TestCaptureListenerEndToEnd(t *testing.T) {
	gotJA4 := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJA4 <- r.Header.Get(ja4Header)
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	addr := startCapture(t, origin.URL, 10*time.Second)

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
	if body, _ := io.ReadAll(resp.Body); string(body) != "ok" {
		t.Errorf("got body %q, want %q", body, "ok")
	}

	// Checking the value actually looks like a JA4, not just that
	// something was set — a wrong fingerprint is worse than none,
	// because the scoring layer would trust it.
	select {
	case got := <-gotJA4:
		if !ja4Shape.MatchString(got) {
			t.Errorf("origin got %s = %q, want a JA4 like t13d1516h2_8daaf6152771_e5627efa2ab1", ja4Header, got)
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
	}
}

// A client that connects and then never sends a ClientHello must get
// dropped, not held open forever.
//
// Scope, checked by mutation and worth being honest about: this only
// proves *a* timeout closed the connection. It still passes if Accept
// stops handing net/http a real *tls.Conn (the connection then gets
// treated as plain HTTP and closed by the same ReadHeaderTimeout).
// TestCaptureListenerEndToEnd is what catches that one.
func TestCaptureListenerTimesOutSlowHandshake(t *testing.T) {
	addr := startCapture(t, "http://127.0.0.1:1", 200*time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// connected, but deliberately silent from here on

	// The read deadline is far longer than the handshake timeout. If
	// the server never closes the connection, this Read ends in *our
	// own* deadline instead of EOF — that difference is the only
	// thing separating "the timeout worked" from "the test got bored
	// waiting", so check which one actually happened.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	_, err = conn.Read(make([]byte, 1))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("connection stayed open, want it closed after the handshake timeout")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("read hit its own %s deadline: the server never closed the connection, so no handshake timeout applied", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("connection closed after %s, want it closed near the 200ms handshake timeout", elapsed)
	}
}

// Garbage on the TLS port (a port scanner, a plain-HTTP client) must
// not wedge the listener for everyone else.
func TestCaptureListenerSurvivesBadHandshake(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer origin.Close()

	addr := startCapture(t, origin.URL, 10*time.Second)

	bad, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	bad.Write([]byte("not a tls handshake"))
	bad.Close()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatalf("request after a bad handshake: %v", err)
	}
	resp.Body.Close()
}
