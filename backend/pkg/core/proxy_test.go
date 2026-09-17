package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOriginProxy(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the headers were correctly modified
		if r.Header.Get("X-Real-IP") == "" {
			t.Error("expected X-Real-IP to be set")
		}
		if r.Header.Get("X-Forwarded-For") != "" {
			// httputil.ReverseProxy sets X-Forwarded-For automatically
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
