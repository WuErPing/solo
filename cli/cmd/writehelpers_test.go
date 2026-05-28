package cmd

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// errWriter is an io.Writer that always returns an error.
type errWriter struct{ err error }

func (e errWriter) Write([]byte) (int, error) { return 0, e.err }

func TestErrFprintf_Success(t *testing.T) {
	var buf bytes.Buffer
	if err := errFprintf(&buf, "hello %s %d", "world", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "hello world 42" {
		t.Errorf("got %q, want %q", got, "hello world 42")
	}
}

func TestErrFprintf_WriteError(t *testing.T) {
	want := errors.New("disk full")
	err := errFprintf(errWriter{err: want}, "hello %s", "world")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestErrFprintln_Success(t *testing.T) {
	var buf bytes.Buffer
	if err := errFprintln(&buf, "line one"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "line one\n" {
		t.Errorf("got %q, want %q", got, "line one\n")
	}
}

func TestErrFprintln_WriteError(t *testing.T) {
	want := errors.New("pipe broken")
	err := errFprintln(errWriter{err: want}, "x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestErrFprint_Success(t *testing.T) {
	var buf bytes.Buffer
	if err := errFprint(&buf, "no newline"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "no newline" {
		t.Errorf("got %q, want %q", got, "no newline")
	}
}

func TestErrFprint_WriteError(t *testing.T) {
	want := errors.New("broken")
	err := errFprint(errWriter{err: want}, "x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

// Verify helpers return the byte count from the underlying writer.
func TestErrFprintf_ByteCount(t *testing.T) {
	var buf bytes.Buffer
	if err := errFprintf(&buf, "12345"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 5 {
		t.Errorf("wrote %d bytes, want 5", buf.Len())
	}
}

// Ensure io.EOF is propagated unchanged.
func TestErrFprintf_EOF(t *testing.T) {
	err := errFprintf(errWriter{err: io.EOF}, "x")
	if !errors.Is(err, io.EOF) {
		t.Errorf("got %v, want io.EOF", err)
	}
}
