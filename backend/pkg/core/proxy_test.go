package core

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
)

func TestNewOriginProxy(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the headers were correctly modified
		if r.Header.Get("X-Real-IP") == "" {
			t.Error("expected X-Real-IP to be set")
		}
		if got := r.Header.Get("X-Forwarded-For"); got == "" {
			t.Error("expected httputil.ReverseProxy to set X-Forwarded-For automatically")
		}
		// Spoofed headers should be stripped
		if r.Header.Get("True-Client-IP") != "" {
			t.Error("expected spoofed IP header to be stripped")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	proxy, err := NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("True-Client-IP", "8.8.8.8") // Malicious spoof attempt

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestNewOriginProxy_InvalidTarget(t *testing.T) {
	_, err := NewOriginProxy("://invalid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	_, err = NewOriginProxy("not-a-url")
	if err == nil {
		t.Fatal("expected error for URL without scheme/host")
	}
}

// deceiveResp builds a response as the origin would return it, already
// tagged with the deceive decision so ModifyResponse acts on it.
func deceiveResp(t *testing.T, contentType string, body []byte, status int) *http.Response {
	t.Helper()
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req = req.WithContext(WithDecision(req.Context(), signals.DecisionDeceive.String(), 100))

	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode:    status,
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func TestDeceiveResponseInjectsIntoHTML(t *testing.T) {
	resp := deceiveResp(t, "text/html; charset=utf-8", []byte("<html><body>hi</body></html>"), http.StatusOK)

	if err := deceiveResponse(resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(got, []byte(signals.HoneypotPath)) {
		t.Fatal("deceived HTML must carry the honeypot bait link")
	}
	if resp.Header.Get("Content-Length") != strconv.Itoa(len(got)) {
		t.Errorf("Content-Length %q does not match the rewritten body length %d",
			resp.Header.Get("Content-Length"), len(got))
	}
	if resp.Header.Get("Etag") != "" {
		t.Error("a rewritten body must not keep the origin's ETag")
	}
}

func TestDeceiveResponseLeavesNonHTMLUntouched(t *testing.T) {
	body := []byte(`{"price": 42}`)
	resp := deceiveResp(t, "application/json", body, http.StatusOK)

	if err := deceiveResponse(resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("non-HTML must pass through byte for byte, got %q", got)
	}
}

// TestDeceiveResponseDoesNotBufferLargeBodies is the memory guard. The
// earlier version of this hook called io.ReadAll on the origin body, so
// one deceived request for a large file pinned that whole file in heap.
// The response must still arrive intact, just unmodified.
func TestDeceiveResponseDoesNotBufferLargeBodies(t *testing.T) {
	// Far larger than the cap, so "memory stayed near the cap" is a
	// claim about the cap and not about this body's size.
	large := bytes.Repeat([]byte("a"), 8*maxDeceptionBodyBytes)
	resp := deceiveResp(t, "text/html", large, http.StatusOK)
	// An origin that streams (chunked) declares no length, so the cap
	// has to hold on the read path rather than on the header alone.
	resp.ContentLength = -1

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if err := deceiveResponse(resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runtime.ReadMemStats(&after)
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 2*maxDeceptionBodyBytes {
		t.Errorf("hook allocated %d bytes for an over-cap body; it must not buffer the whole response", grew)
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the passed-through body: %v", err)
	}
	if !bytes.Equal(got, large) {
		t.Fatalf("an over-cap body must reach the visitor unchanged: got %d bytes, want %d", len(got), len(large))
	}
}

func TestDeceiveResponseSkipsCompressedAndPartial(t *testing.T) {
	html := []byte("<html><body>hi</body></html>")

	compressed := deceiveResp(t, "text/html", html, http.StatusOK)
	compressed.Header.Set("Content-Encoding", "gzip")
	if err := deceiveResponse(compressed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := io.ReadAll(compressed.Body); !bytes.Equal(got, html) {
		t.Error("a compressed body must not be rewritten as if it were plain HTML")
	}

	// A 206 carries a slice of a document; injecting into it corrupts
	// the client's reassembly. A 304 carries no body at all.
	partial := deceiveResp(t, "text/html", html, http.StatusPartialContent)
	if err := deceiveResponse(partial); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := io.ReadAll(partial.Body); !bytes.Equal(got, html) {
		t.Error("a 206 Partial Content response must not be rewritten")
	}
}

func TestDeceiveResponseIgnoresUndeceivedTraffic(t *testing.T) {
	html := []byte("<html><body>hi</body></html>")
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
		Body:       io.NopCloser(bytes.NewReader(html)),
		Request:    req,
	}

	if err := deceiveResponse(resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := io.ReadAll(resp.Body); !bytes.Equal(got, html) {
		t.Fatal("a normal visitor's page must never be rewritten")
	}
}
