package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// A visitor must not be able to fake where they're coming from. Any
// X-Forwarded-For they send has to be replaced with their real IP,
// not appended to — per-IP rate limiting and geo checks are only
// worth anything if the IP can't be chosen by the caller.
func TestNewStripsSpoofedForwardedFor(t *testing.T) {
	got := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("X-Forwarded-For")
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	front := httptest.NewServer(p)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case v := <-got:
		if strings.Contains(v, "1.2.3.4") {
			t.Errorf("origin got X-Forwarded-For = %q, want the spoofed 1.2.3.4 gone", v)
		}
		if v == "" {
			t.Error("origin got no X-Forwarded-For at all, want the real client IP")
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
	}
}

// X-Forwarded-For isn't the only header an origin trusts for "who is
// calling". nginx, Cloudflare and Fastly stacks read their own, and
// net/http doesn't strip those — so a visitor could pick their
// apparent IP through whichever one the origin happens to honour.
func TestNewStripsOtherClientIPHeaders(t *testing.T) {
	got := make(chan http.Header, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	front := httptest.NewServer(p)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/", nil)
	for _, h := range clientIPHeaders {
		req.Header.Set(h, "1.2.3.4")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case h := <-got:
		for _, name := range clientIPHeaders {
			if v := h.Get(name); strings.Contains(v, "1.2.3.4") {
				t.Errorf("origin got %s = %q, want the spoofed 1.2.3.4 gone", name, v)
			}
		}
		if h.Get(realIPHeader) == "" {
			t.Errorf("origin got no %s, want the real client IP", realIPHeader)
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
	}
}

// A visitor must not be able to fake the UA-mismatch flag either — a
// scanner setting this to "true" on every request would poison
// scoring, and a real bot setting it to "" wouldn't help them since
// we always recompute it, but the incoming value must never survive.
func TestNewStripsSpoofedUAMismatchHeader(t *testing.T) {
	got := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get(uaMismatchHeader)
	}))
	defer origin.Close()

	p, err := New(origin.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	front := httptest.NewServer(p)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/", nil)
	req.Header.Set(uaMismatchHeader, "true")
	req.Header.Set("User-Agent", "curl/8.6.0") // not claiming to be a browser, so real value is absent
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case v := <-got:
		if v != "" {
			t.Errorf("origin got spoofed %s = %q, want it stripped", uaMismatchHeader, v)
		}
	default:
		t.Fatal("origin was never reached, so this test proved nothing")
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
