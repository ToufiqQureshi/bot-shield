package api

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "10.0.1.50:8080", wantErr: true},
		{in: "http://10.0.1.50:8080", wantErr: true},
		{in: "http://169.254.169.254", wantErr: true},
		{in: "https://origin.example.com:8443", want: "https://origin.example.com:8443"},
		{in: "example.com", want: "http://example.com"},
		{in: "", wantErr: true},
		{in: "http://", wantErr: true},
		{in: "://bad", wantErr: true},
	}
	for _, c := range cases {
		got, err := normalizeOrigin(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("normalizeOrigin(%q) = %q, nil; want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeOrigin(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
