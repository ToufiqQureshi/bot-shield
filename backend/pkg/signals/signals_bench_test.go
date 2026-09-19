package signals_test

import (
	"encoding/hex"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
)

const curlClientHello = "1603010200010001fc030345b0e945658446fb98136c30e1be82ed4bd81e16d332b9f3317a553fcb88e4262032776135cd2a213dcd935ee9f471768d714d8a9e3292102e1a2e840f52644b0100204a4a130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010001934a4a00000000001900170000146c707461672e6c697665706572736f6e2e6e65740033002b00291a1a000100001d0020a0a1a353c499704a9b56af77f3f87cfdd287e33009eda54f9ab9b43fb2f595630010000e000c02683208687474702f312e3100170000ff0100010000120000002b000706dada03040303000d0012001004030804040105030805050108060601000a000a00081a1a001d00170018002d0002010100050005010000000000230000000b00020100446900050003026832001b0003020002eaea000100001500c3000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

func BenchmarkParseJA4(b *testing.B) {
	raw, _ := hex.DecodeString(curlClientHello)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = signals.ParseJA4(raw)
		}
	})
}

func BenchmarkScore(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = signals.Score(signals.RequestFacts{JA4: "unreadable", UA: "Mozilla/5.0 (Windows NT 10.0)"})
		}
	})
}

func BenchmarkAnalyze(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = signals.Analyze(signals.RequestFacts{JA4: "unreadable", UA: "bot/1.0"})
		}
	})
}

// FuzzParseJA4 exercises the TLS parser with arbitrary input — it should
// never panic, only return an error on invalid data.
func FuzzParseJA4(f *testing.F) {
	raw, _ := hex.DecodeString(curlClientHello)
	f.Add(raw)
	f.Add([]byte{})
	f.Add([]byte{0x16, 0x03, 0x01, 0x00, 0x00})
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic. Error is fine, empty string is fine.
		fp, err := signals.ParseJA4(data)
		if err == nil && fp == "" {
			t.Error("got empty fingerprint with no error")
		}
	})
}

// FuzzUAMismatch exercises the UA mismatch detector with arbitrary input.
func FuzzUAMismatch(f *testing.F) {
	f.Add("t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0")
	f.Add("unreadable", "curl/7.68")
	f.Add("", "")
	f.Add("some_fp", "Python-urllib/3.9")

	f.Fuzz(func(t *testing.T, ja4, ua string) {
		// Must never panic.
		_ = signals.UAMismatch(ua, ja4)
	})
}
