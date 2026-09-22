package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactCredentials(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "password redacted, username kept",
			in:   "postgresql://postgres.abc123:supersecretpassword@aws-0-ap-south-1.pooler.supabase.com:5432/postgres",
			want: "postgresql://postgres.abc123:REDACTED@aws-0-ap-south-1.pooler.supabase.com:5432/postgres",
		},
		{
			name: "no credentials, untouched",
			in:   "postgresql://localhost:5432/hakaishield",
			want: "postgresql://localhost:5432/hakaishield",
		},
		{
			name: "username only, no password added",
			in:   "postgresql://postgres@localhost:5432/hakaishield",
			want: "postgresql://postgres@localhost:5432/hakaishield",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactCredentials(c.in)
			if got != c.want {
				t.Errorf("redactCredentials(%q) = %q, want %q", c.in, got, c.want)
			}
			if got == c.in && c.name == "password redacted, username kept" {
				t.Error("password appeared unredacted in output")
			}
		})
	}
}

func TestRedactCredentials_NeverLeaksPasswordSubstring(t *testing.T) {
	// The specific regression this guards: a raw password string must
	// never appear in the redacted output, regardless of URL shape.
	password := "i4A1jmYKSFcZwEOe-not-a-real-secret"
	in := "postgresql://postgres.xyz:" + password + "@aws-0-ap-south-1.pooler.supabase.com:5432/postgres"
	got := redactCredentials(in)
	if strings.Contains(got, password) {
		t.Fatalf("redactCredentials leaked the password into: %q", got)
	}
}

func TestRedactCredentials_UnparseableInputDoesNotPanic(t *testing.T) {
	got := redactCredentials("://not a valid url at all")
	if got == "" {
		t.Error("expected a non-empty fallback message for unparseable input")
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "DATABASE_URL=postgresql://example/db\n# a comment\n\nSUPABASE_URL=\"https://example.supabase.co\"\nQUOTED_SINGLE='value'\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test .env: %v", err)
	}

	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("QUOTED_SINGLE")
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SUPABASE_URL")
		os.Unsetenv("QUOTED_SINGLE")
	})

	loadDotEnv(envPath)

	if got := os.Getenv("DATABASE_URL"); got != "postgresql://example/db" {
		t.Errorf("DATABASE_URL = %q, want %q", got, "postgresql://example/db")
	}
	if got := os.Getenv("SUPABASE_URL"); got != "https://example.supabase.co" {
		t.Errorf("SUPABASE_URL = %q, want %q (quotes should be stripped)", got, "https://example.supabase.co")
	}
	if got := os.Getenv("QUOTED_SINGLE"); got != "value" {
		t.Errorf("QUOTED_SINGLE = %q, want %q", got, "value")
	}
}

func TestLoadDotEnv_DoesNotOverrideExistingEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("SOME_VAR=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("writing test .env: %v", err)
	}

	os.Setenv("SOME_VAR", "from-real-shell-env")
	t.Cleanup(func() { os.Unsetenv("SOME_VAR") })

	loadDotEnv(envPath)

	if got := os.Getenv("SOME_VAR"); got != "from-real-shell-env" {
		t.Errorf("SOME_VAR = %q, want the pre-existing shell value to win over .env", got)
	}
}

func TestLoadDotEnv_MissingFileIsNotAnError(t *testing.T) {
	// Must not panic or os.Exit; a missing .env is the normal case in
	// production where config comes from real env vars.
	loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist.env"))
}
