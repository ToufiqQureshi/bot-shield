package decide

import "testing"

// Predict runs on every request when a shadow model is loaded, so its
// cost is a per-request cost for every customer. It must stay a dot
// product: no allocation, no lookups, no growth with traffic.
func BenchmarkPredict(b *testing.B) {
	m, err := Train(testFeatures, trainingSet(), DefaultOptions())
	if err != nil {
		b.Fatalf("Train() error: %v", err)
	}
	fired := bit(0) | bit(2)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = m.Predict(fired)
	}
}

// Explain allocates, which is why it is not part of Predict: it runs only
// where a decision is being recorded, not on every scored request.
func BenchmarkExplain(b *testing.B) {
	m, err := Train(testFeatures, trainingSet(), DefaultOptions())
	if err != nil {
		b.Fatalf("Train() error: %v", err)
	}
	fired := bit(0) | bit(1) | bit(2)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reasonSink = m.Explain(fired)
	}
}

var (
	sink       Prediction
	reasonSink []Contribution
)
