package cliutil

import (
	"testing"
	"time"
)

func TestParseDuration_BareSeconds(t *testing.T) {
	d, err := ParseDuration("90")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 90*time.Second {
		t.Errorf("expected 90s, got %v", d)
	}
}

func TestParseDuration_Seconds(t *testing.T) {
	d, err := ParseDuration("30s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 30*time.Second {
		t.Errorf("expected 30s, got %v", d)
	}
}

func TestParseDuration_Minutes(t *testing.T) {
	d, err := ParseDuration("5m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}
}

func TestParseDuration_Hours(t *testing.T) {
	d, err := ParseDuration("2h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", d)
	}
}

func TestParseDuration_Composite(t *testing.T) {
	d, err := ParseDuration("2h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 2*time.Hour + 30*time.Minute
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDuration_CompositeAllUnits(t *testing.T) {
	d, err := ParseDuration("1h2m3s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 1*time.Hour + 2*time.Minute + 3*time.Second
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDuration_Empty(t *testing.T) {
	_, err := ParseDuration("")
	if err == nil {
		t.Error("expected error for empty duration")
	}
}

func TestParseDuration_InvalidFormat(t *testing.T) {
	_, err := ParseDuration("abc")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestParseDuration_Whitespace(t *testing.T) {
	d, err := ParseDuration("  5m  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}
}

func TestParseDuration_MixedInvalid(t *testing.T) {
	_, err := ParseDuration("5x")
	if err == nil {
		t.Error("expected error for invalid unit")
	}
}

func TestIsDigits(t *testing.T) {
	if !isDigits("123") {
		t.Error("expected true for digits")
	}
	if isDigits("") {
		t.Error("expected false for empty")
	}
	if isDigits("12a") {
		t.Error("expected false for mixed")
	}
	if isDigits("-5") {
		t.Error("expected false for negative")
	}
}

func TestTrim(t *testing.T) {
	if trim("  hello") != "hello" {
		t.Errorf("expected hello, got %q", trim("  hello"))
	}
	if trim("\thello") != "hello" {
		t.Errorf("expected hello, got %q", trim("\thello"))
	}
	if trim("hello") != "hello" {
		t.Errorf("expected hello, got %q", trim("hello"))
	}
}
