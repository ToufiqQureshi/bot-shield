package core

import (
	"context"
	"net/http"
)

// WithMethod carries the HTTP method from Guard into the origin proxy's
// Rewrite hook, which only sees the outbound request. The proxy already
// reads other Guard-resolved facts (client IP, JA4) the same way.
type methodKey struct{}

// WithMethod attaches the request method to the request context.
func WithMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, methodKey{}, method)
}

// MethodFromContext returns the method WithMethod stored, or "".
func MethodFromContext(ctx context.Context) string {
	m, _ := ctx.Value(methodKey{}).(string)
	return m
}

// measureWriter wraps a response writer to count the bytes actually
// sent to the visitor. hakaishield proxies customer origin responses,
// so egress bandwidth is the dominant hosting cost (the plan's
// cloud-bill section): the dashboard's cost number must come from what
// we really served, not from a request count multiplied by a guess.
type measureWriter struct {
	http.ResponseWriter
	written int64
}

// NewMeasureWriter wraps w so BytesWritten reports the body bytes
// passed through it.
func NewMeasureWriter(w http.ResponseWriter) *measureWriter {
	return &measureWriter{ResponseWriter: w}
}

func (mw *measureWriter) Write(p []byte) (int, error) {
	n, err := mw.ResponseWriter.Write(p)
	mw.written += int64(n)
	return n, err
}

func (mw *measureWriter) BytesWritten() int64 { return mw.written }
