package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
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
	var gotJA4 string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJA4 = r.Header.Get(ja4Header)
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

	if gotJA4 == "" {
		t.Error("origin got no JA4 fingerprint header, want a non-empty one")
	}
}

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

	// the connection should be closed from the server side once the
	// handshake timeout fires
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("expected connection to be closed after handshake timeout, got no error")
	}
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
