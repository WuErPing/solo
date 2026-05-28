package redact

import (
	"strings"
	"testing"
)

func TestBuildRedactor_DisabledByDefault(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "PASSWORD=hunter2 and sk-abcdefghijklmnopqrstuvwxyz"
	if got := r.Redact(in); got != in {
		t.Errorf("disabled redactor modified input:\ngot:  %q\nwant: %q", got, in)
	}
}

func TestBuildRedactor_ApiKeys_RedactsProviderTokens(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{APIKeys: true})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "OPENAI=sk-abcdefghijklmnopqrstuvwxyz"
	got := r.Redact(in)
	if strings.Contains(got, "sk-abc") {
		t.Errorf("OpenAI token leaked: %q", got)
	}
}

func TestBuildRedactor_EnvFiles_RedactsSensitiveEnvLines(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{EnvFiles: true})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "PASSWORD=hunter2\nOTHER=safe"
	got := r.Redact(in)
	if strings.Contains(got, "hunter2") {
		t.Errorf("PASSWORD leaked: %q", got)
	}
	if !strings.Contains(got, "OTHER=safe") {
		t.Errorf("non-sensitive line damaged: %q", got)
	}
}

func TestBuildRedactor_BothEnabled_CombinesRedactors(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{APIKeys: true, EnvFiles: true})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "PASSWORD=hunter2\nOPENAI=sk-abcdefghijklmnopqrstuvwxyz\nSAFE=ok"
	got := r.Redact(in)
	if strings.Contains(got, "hunter2") {
		t.Errorf("PASSWORD leaked: %q", got)
	}
	if strings.Contains(got, "sk-abc") {
		t.Errorf("OpenAI token leaked: %q", got)
	}
	if !strings.Contains(got, "SAFE=ok") {
		t.Errorf("safe line damaged: %q", got)
	}
}

func TestBuildRedactor_CustomRegexes_Applied(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{
		CustomRegexes: []string{`CUSTOM-[A-Z]{8}`},
	})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "tok=CUSTOM-ABCDEFGH rest"
	got := r.Redact(in)
	if strings.Contains(got, "CUSTOM-ABCDEFGH") {
		t.Errorf("custom regex did not fire: %q", got)
	}
}

func TestBuildRedactor_InvalidRegex_ReturnsError(t *testing.T) {
	if _, err := BuildRedactor(RedactorConfig{CustomRegexes: []string{"(unclosed"}}); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestBuildRedactor_CustomSensitiveKeys_Honored(t *testing.T) {
	r, err := BuildRedactor(RedactorConfig{
		EnvFiles:      true,
		SensitiveKeys: []string{"MY_SPECIAL_KEY"},
	})
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "MY_SPECIAL_KEY=topsecret\nOTHER=safe"
	got := r.Redact(in)
	if strings.Contains(got, "topsecret") {
		t.Errorf("custom sensitive key leaked: %q", got)
	}
}
