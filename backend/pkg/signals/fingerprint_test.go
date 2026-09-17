package signals

import (
	"encoding/hex"
	"testing"
)

// A real curl 8.6.0 ClientHello, hex-encoded, with its published JA4.
// Kept as a shared fixture so the malformed-input test can chop up a
// genuinely valid handshake instead of inventing fake bytes.
const curlClientHello = "1603010200010001fc030345b0e945658446fb98136c30e1be82ed4bd81e16d332b9f3317a553fcb88e4262032776135cd2a213dcd935ee9f471768d714d8a9e3292102e1a2e840f52644b0100204a4a130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010001934a4a00000000001900170000146c707461672e6c697665706572736f6e2e6e65740033002b00291a1a000100001d0020a0a1a353c499704a9b56af77f3f87cfdd287e33009eda54f9ab9b43fb2f595630010000e000c02683208687474702f312e3100170000ff0100010000120000002b000706dada03040303000d0012001004030804040105030805050108060601000a000a00081a1a001d00170018002d0002010100050005010000000000230000000b00020100446900050003026832001b0003020002eaea000100001500c3000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

// Checks our function against 2 real handshakes with already-known,
// correct JA4 answers (borrowed from fingerproxy's own tests), so we
// know we're not just matching our own guess.
func TestParseJA4(t *testing.T) {
	cases := []struct {
		name        string
		clientHello string // hex-encoded raw ClientHello record
		want        string
	}{
		{
			name:        "curl 8.6.0",
			clientHello: curlClientHello,
			want:        "t13d1516h2_8daaf6152771_e5627efa2ab1",
		},
		{
			name:        "PSK extension",
			clientHello: "160301020b0100020703037f020a187f3aa7329f24155b77abff130dd616e200f6ef7d6c2d4657bf48218a20d945e74ab5e723901b3948e36cd39e248009489982497543815cdd74c3da32620076130213031301c02fc02bc030c02c009ec0270067c028006b00a3009fcca9cca8ccaac0afc0adc0a3c09fc05dc061c057c05300a2c0aec0acc0a2c09ec05cc060c056c052c024006ac0230040c00ac01400390038c009c01300330032009dc0a1c09dc051009cc0a0c09cc050003d003c0035002f00ff010001480000001b0019000016736869627579612e6170692e7375627363616e2e696f000b000403000102000a000c000a001d0017001e00190018002300000016000000170000000d0030002e040305030603080708080809080a080b080408050806040105010601030302030301020103020202040205020602002b00050403040303002d00020101003300260024001d00207289331a6f55556a98dfe0c96d52fc31d897644a5f87c3d71506b98fc198602300290094006f0069eb56145bbba79db5b290bd16a6133dea5d88e79857b13f7ac21c07962ca58afc84c0f1e8f29205c345c5eeeb67237ace5f6838feadfd2acadc5e464ddf7c9b3a9560d9dd6a8f030c452d6ea621b45e5c07e899184648adcc8a5d898ff6dc6050627de2070b9cd0efcea059033500212061b4238d30f5cda4b6559bd1061936b2912bd69a8b49610246db2d7bbae4b73c",
			want:        "t13d591100_a33745022dd6_a11995863d32",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := hex.DecodeString(c.clientHello)
			if err != nil {
				t.Fatalf("bad test fixture: %v", err)
			}
			got, err := ParseJA4(raw)
			if err != nil {
				t.Fatalf("JA4Fingerprint: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Broken/fake input should give an error and an empty fingerprint —
// never a crash, and never a made-up value the scoring layer would
// trust. Bots send malformed handshakes on purpose, so these are
// normal inputs here, not rare edge cases.
func TestJA4FingerprintRejectsGarbage(t *testing.T) {
	realHello, err := hex.DecodeString(curlClientHello)
	if err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}

	cases := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"random bytes", []byte{0x00, 0x01, 0x02}},
		{"not a handshake record", []byte{0x17, 0x03, 0x03, 0x00, 0x05, 1, 2, 3, 4, 5}},
		{"truncated mid-handshake", realHello[:40]},
		{"header only, body missing", realHello[:5]},
		{"length claims more than sent", append([]byte{0x16, 0x03, 0x01, 0xff, 0xff}, realHello[5:60]...)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// t.Run isolates a panic to this subtest's goroutine, so a
			// crash here fails loudly instead of being missed.
			fp, err := ParseJA4(c.input)
			if err == nil {
				t.Errorf("got fingerprint %q with no error, want an error", fp)
			}
			if fp != "" {
				t.Errorf("got fingerprint %q on bad input, want empty", fp)
			}
		})
	}
}
