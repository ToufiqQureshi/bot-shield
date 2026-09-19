package deception

import (
	"bytes"
	"testing"
)

func TestInjectPoisonPayload_HTMLWithBodyTag(t *testing.T) {
	html := []byte("<html><body><h1>Hello World</h1></body></html>")
	contentType := "text/html; charset=utf-8"

	transformed := InjectPoisonPayload(html, contentType)
	if !bytes.Contains(transformed, []byte(DefaultPoisonPayload)) {
		t.Fatalf("expected transformed HTML to contain poison payload")
	}

	if !bytes.Contains(transformed, []byte("<h1>Hello World</h1>"+DefaultPoisonPayload+"</body></html>")) {
		t.Fatalf("expected payload to be inserted right before </body>")
	}
}

func TestInjectPoisonPayload_HTMLWithHtmlTagOnly(t *testing.T) {
	html := []byte("<html><div>Content</div></html>")
	contentType := "text/html"

	transformed := InjectPoisonPayload(html, contentType)
	if !bytes.Contains(transformed, []byte(DefaultPoisonPayload)) {
		t.Fatalf("expected transformed HTML to contain poison payload")
	}

	if !bytes.Contains(transformed, []byte("<div>Content</div>"+DefaultPoisonPayload+"</html>")) {
		t.Fatalf("expected payload to be inserted right before </html>")
	}
}

func TestInjectPoisonPayload_NonHTML(t *testing.T) {
	jsonPayload := []byte(`{"status":"ok","message":"hello"}`)
	contentType := "application/json"

	transformed := InjectPoisonPayload(jsonPayload, contentType)
	if !bytes.Equal(transformed, jsonPayload) {
		t.Fatalf("expected non-HTML content to remain untransformed")
	}
}

func TestInjectPoisonPayload_EmptyBody(t *testing.T) {
	transformed := InjectPoisonPayload(nil, "text/html")
	if len(transformed) != 0 {
		t.Fatalf("expected empty body to return empty")
	}
}

func TestInjectPoisonPayload_HTMLWithoutClosingTags(t *testing.T) {
	html := []byte("<div>Partial HTML</div>")
	contentType := "text/html"

	transformed := InjectPoisonPayload(html, contentType)
	if !bytes.Contains(transformed, []byte(DefaultPoisonPayload)) {
		t.Fatalf("expected payload to be appended at the end of partial HTML")
	}
}
