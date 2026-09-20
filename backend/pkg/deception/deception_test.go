package deception

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
)

func TestInjectPayloadBeforeBody(t *testing.T) {
	html := []byte("<html><body><h1>Hello World</h1></body></html>")

	got := InjectPayload(html)

	want := "<h1>Hello World</h1>" + string(payload) + "</body></html>"
	if !bytes.Contains(got, []byte(want)) {
		t.Fatalf("payload should sit just inside </body>, got:\n%s", got)
	}
}

func TestInjectPayloadBeforeHTMLWhenBodyMissing(t *testing.T) {
	html := []byte("<html><div>Content</div></html>")

	got := InjectPayload(html)

	want := "<div>Content</div>" + string(payload) + "</html>"
	if !bytes.Contains(got, []byte(want)) {
		t.Fatalf("payload should fall back to </html>, got:\n%s", got)
	}
}

func TestInjectPayloadHandlesUppercaseTags(t *testing.T) {
	// Real pages in the wild still ship uppercase tags; matching only
	// lowercase would append after </BODY>, outside the document.
	html := []byte("<HTML><BODY><p>hi</p></BODY></HTML>")

	got := InjectPayload(html)

	if !bytes.Contains(got, []byte("<p>hi</p>"+string(payload)+"</BODY>")) {
		t.Fatalf("payload should be inserted before an uppercase </BODY>, got:\n%s", got)
	}
}

func TestInjectPayloadAppendsToFragment(t *testing.T) {
	html := []byte("<div>Partial HTML</div>")

	got := InjectPayload(html)

	if !bytes.HasSuffix(got, payload) {
		t.Fatalf("a fragment with no closing tags should get the payload appended, got:\n%s", got)
	}
}

func TestInjectPayloadEmptyBody(t *testing.T) {
	if got := InjectPayload(nil); len(got) != 0 {
		t.Fatalf("empty body should stay empty, got %q", got)
	}
}

func TestPayloadLinksTheHoneypotPath(t *testing.T) {
	// The trap only catches anything if the bait is actually in the
	// page, and only if it points at the path the guard watches.
	if !bytes.Contains(payload, []byte(signals.HoneypotPath)) {
		t.Fatalf("payload must link the honeypot path %q so the trap is reachable", signals.HoneypotPath)
	}
}

func TestPayloadIsHiddenFromAssistiveTech(t *testing.T) {
	// A blind visitor must never have the poison text read out, and the
	// bait link must not be reachable by keyboard: either would turn a
	// bot trap into an accessibility failure (CLAUDE.md Section 14).
	p := string(payload)

	if !strings.Contains(p, `aria-hidden="true"`) {
		t.Error("payload must carry a real aria-hidden attribute, not a CSS lookalike")
	}
	if !strings.Contains(p, `tabindex="-1"`) {
		t.Error("the bait link must be out of the keyboard tab order")
	}
	if !strings.Contains(p, `rel="nofollow"`) {
		t.Error("the bait link must tell well-behaved crawlers to skip it")
	}
	if !strings.Contains(p, "display:none") {
		t.Error("payload must not render for sighted visitors")
	}
}

func TestIsInjectableContentType(t *testing.T) {
	injectable := []string{
		"text/html",
		"text/html; charset=utf-8",
		"TEXT/HTML; charset=UTF-8",
		"application/xhtml+xml",
	}
	for _, ct := range injectable {
		if !IsInjectableContentType(ct) {
			t.Errorf("%q should be injectable", ct)
		}
	}

	// Anything here that returned true would have the whole response
	// buffered in memory for nothing.
	notInjectable := []string{
		"application/json",
		"image/png",
		"video/mp4",
		"application/zip",
		"text/plain",
		"",
	}
	for _, ct := range notInjectable {
		if IsInjectableContentType(ct) {
			t.Errorf("%q must not be injectable", ct)
		}
	}
}
