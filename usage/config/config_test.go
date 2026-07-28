package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandVarEnv(t *testing.T) {
	t.Setenv("SOLO_TEST_KEY", "secret-value")
	cases := []struct{ in, want string }{
		{"${SOLO_TEST_KEY}", "secret-value"},
		{"prefix-${SOLO_TEST_KEY}-suffix", "prefix-secret-value-suffix"},
		{"${SOLO_TEST_UNSET}", "${SOLO_TEST_UNSET}"}, // unset kept as-is
		{"plain", "plain"},
	}
	for _, tc := range cases {
		if got := expandVar(tc.in); got != tc.want {
			t.Errorf("expandVar(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExpandVarFile(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "xiaomimimo.cookie")
	if err := os.WriteFile(secret, []byte("  cookie-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ in, want string }{
		{"${file:" + secret + "}", "cookie-value"},                                                         // trimmed
		{"${file:" + filepath.Join(dir, "missing") + "}", "${file:" + filepath.Join(dir, "missing") + "}"}, // unreadable kept as-is
	}
	for _, tc := range cases {
		if got := expandVar(tc.in); got != tc.want {
			t.Errorf("expandVar(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExpandVarFileHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	secret := filepath.Join(home, ".solo", "qoder.cookie")
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("home-cookie"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := expandVar("${file:~/.solo/qoder.cookie}"); got != "home-cookie" {
		t.Errorf("expandVar = %q, want home-cookie", got)
	}
}

func TestLoadExpandsAllFields(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "cookie")
	if err := os.WriteFile(secret, []byte("cookie-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOLO_TEST_APIKEY", "env-key")

	cfg := filepath.Join(dir, "usage.json")
	content := `{
		"providers": {
			"kimi": {"enabled": true, "apiKey": "${SOLO_TEST_APIKEY}"},
			"xiaomimimo": {"enabled": true, "extra": {"cookie": "${file:` + secret + `}"}}
		}
	}`
	if err := os.WriteFile(cfg, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Providers["kimi"].APIKey; got != "env-key" {
		t.Errorf("kimi apiKey = %q, want env-key", got)
	}
	if got := f.Providers["xiaomimimo"].Extra["cookie"]; got != "cookie-value" {
		t.Errorf("xiaomimimo cookie = %q, want cookie-value", got)
	}
}
