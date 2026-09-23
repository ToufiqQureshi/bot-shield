package labels

import "testing"

// Record runs on the request path for every challenged or trapped
// request, so its cost is a per-request cost. It must stay a cap lookup
// and a non-blocking send.
func BenchmarkRecord(b *testing.B) {
	c := NewCollector(&fakeWriter{})
	defer c.Close()

	s := validSample()
	s.Identity = "tenant-1|203.0.113.5|t13d1516h2"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Record(s)
	}
}
