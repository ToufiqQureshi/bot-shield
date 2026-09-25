package signals

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		path, method, want string
	}{
		{"/login", "POST", ClassLogin},
		{"/signin", "GET", ClassLogin},
		{"/auth/session", "GET", ClassLogin},
		{"/checkout", "POST", ClassCheckout},
		{"/checkout/", "GET", ClassCheckout},
		{"/cart", "GET", ClassCheckout},
		{"/payment/card", "POST", ClassCheckout},
		{"/api", "GET", ClassAPI},
		{"/api/v1/users", "POST", ClassAPI},
		{"/static/app.js", "GET", ClassStatic},
		{"/static/logo.png", "GET", ClassStatic},
		{"/logo.png", "POST", ClassBrowse}, // assets are only assets on GET/HEAD
		{"/pricing", "GET", ClassBrowse},
		{"/", "GET", ClassBrowse},
		{"/pricing", "POST", ClassBrowse},
	}
	for _, tc := range cases {
		if got := Classify(tc.path, tc.method); got != tc.want {
			t.Errorf("Classify(%q,%q) = %q, want %q", tc.path, tc.method, got, tc.want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in       string
		wantPath string
		wantOK   bool
	}{
		{"/a/b", "/a/b", true},
		{"", "/", true},
		{"/a/./b", "/a/b", true},
		{"/a/%62", "/a/b", true}, // one level of unescaping is fine
		{"/a/%252e", "", false},  // still percent-encoded after unescape: ambiguous
		{"relative", "", false},
		{"/a//b", "", false},
	}
	for _, tc := range cases {
		gotPath, ok := NormalizePath(tc.in)
		if ok != tc.wantOK || (ok && gotPath != tc.wantPath) {
			t.Errorf("NormalizePath(%q) = (%q,%v), want (%q,%v)", tc.in, gotPath, ok, tc.wantPath, tc.wantOK)
		}
	}
}

// ScoreFromFired must reproduce exactly what Evaluate adds up, or the
// offline rule baseline would disagree with the live scorer.
func TestScoreFromFiredMatchesEvaluateWeights(t *testing.T) {
	var fired uint32
	want := 0
	for i, c := range checks {
		if c.name == "scripting_tool" {
			fired |= 1 << i
			want += c.weight
		}
	}
	if got := ScoreFromFired(fired); got != want {
		t.Errorf("ScoreFromFired = %d, want %d", got, want)
	}
	if ScoreFromFired(0) != 0 {
		t.Error("ScoreFromFired(0) must be 0")
	}
}
