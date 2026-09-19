package signals

import (
	"context"
	"testing"
)

func TestRecordHoneypotTrigger(t *testing.T) {
	ja4 := "t13d1516h2_8daaf6152771_e5627efa2ab1"
	ip := "192.0.2.1"

	RecordHoneypotTrigger(context.Background(), ja4, ip, nil)

	isScraper, tool := IsKnownScraperJA4(ja4)
	if !isScraper {
		t.Fatalf("expected JA4 %s to be registered as scraper after honeypot trigger", ja4)
	}
	if tool != "honeypot_trap" {
		t.Fatalf("expected tool name 'honeypot_trap', got %q", tool)
	}
}

func TestRecordHoneypotTrigger_UnreadableJA4Ignored(t *testing.T) {
	RecordHoneypotTrigger(context.Background(), JA4Unreadable, "192.0.2.2", nil)

	isScraper, _ := IsKnownScraperJA4(JA4Unreadable)
	if isScraper {
		t.Fatalf("unreadable JA4 should not be registered as a fixed scraper fingerprint")
	}
}
