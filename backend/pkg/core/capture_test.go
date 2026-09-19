package core

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

// mockConn is a fake net.Conn for testing
type mockConn struct{}

func (mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (mockConn) Close() error                       { return nil }
func (mockConn) LocalAddr() net.Addr                { return nil }
func (mockConn) RemoteAddr() net.Addr               { return nil }
func (mockConn) SetDeadline(t time.Time) error      { return nil }
func (mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestJA4FromContext(t *testing.T) {
	t.Run("missing conn", func(t *testing.T) {
		ctx := context.Background()
		ja4 := JA4FromContext(ctx)
		if ja4 != "" {
			t.Errorf("expected empty string, got %s", ja4)
		}
	})

	t.Run("non-tls conn", func(t *testing.T) {
		ctx := ConnContext(context.Background(), mockConn{})
		ja4 := JA4FromContext(ctx)
		if ja4 != "" {
			t.Errorf("expected empty string, got %s", ja4)
		}
	})

	// Testing valid TLS connection fingerprinting would require hacking a raw ClientHello stream,
	// which is complex to mock without a real network connection. We cover the failure paths here.
}

func TestNewCaptureListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	cfg := &tls.Config{}
	cl := NewCaptureListener(l, cfg)

	if cl == nil {
		t.Fatal("expected non-nil capture listener")
	}
}
