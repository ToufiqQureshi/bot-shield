package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A request through the proxy should reach the origin unchanged and
// the origin's response should come back unchanged. This is the
// baseline every detection layer will sit in front of later.
func TestPassthrough(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hello" {
			t.Errorf("origin got path %q, want /hello", r.URL.Path)
		}
		w.Write([]byte("hello from origin"))
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	front := httptest.NewServer(p)
	defer front.Close()

	resp, err := http.Get(front.URL + "/hello")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from origin" {
		t.Errorf("got body %q, want %q", body, "hello from origin")
	}
}

// An invalid target URL must fail at construction time, not at the
// first request — a bad config should never reach production traffic.
func TestNewRejectsBadTarget(t *testing.T) {
	if _, err := New("://not-a-url"); err == nil {
		t.Error("New with invalid URL: got nil error, want error")
	}
}

// A visitor must not be able to fake a JA4 fingerprint by just
// setting the header themselves — that header is meant to come only
// from bot-shield's own TLS capture.
func TestNewStripsSpoofedJA4Header(t *testing.T) {
	got := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get(ja4Header)
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	front := httptest.NewServer(p)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/", nil)
	req.Header.Set(ja4Header, "t13d1516h2_fake_fake")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	// Asserting inside the handler alone would let this test pass by
	// accident if the request never reached the origin at all.
	select {
	case v := <-got:
		if v != "" {
			t.Errorf("origin got spoofed %s = %q, want it stripped", ja4Header, v)
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
	}
}
