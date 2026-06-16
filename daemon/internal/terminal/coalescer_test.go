package terminal

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestOutputCoalescer_BasicFlush(t *testing.T) {
	var flushed [][]byte
	var mu sync.Mutex
	c := NewOutputCoalescer(func(payload []byte) {
		mu.Lock()
		flushed = append(flushed, append([]byte(nil), payload...))
		mu.Unlock()
	})

	c.Add([]byte("hello"))
	c.Add([]byte(" world"))

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}
	if !bytes.Equal(flushed[0], []byte("hello world")) {
		t.Errorf("expected 'hello world', got %q", flushed[0])
	}
	mu.Unlock()
}

func TestOutputCoalescer_ManualFlush(t *testing.T) {
	var flushed [][]byte
	var mu sync.Mutex
	c := NewOutputCoalescer(func(payload []byte) {
		mu.Lock()
		flushed = append(flushed, append([]byte(nil), payload...))
		mu.Unlock()
	})

	c.Add([]byte("abc"))
	c.Flush()

	mu.Lock()
	if len(flushed) != 1 || !bytes.Equal(flushed[0], []byte("abc")) {
		t.Errorf("expected ['abc'], got %v", flushed)
	}
	mu.Unlock()
}

func TestOutputCoalescer_StopFlushesPending(t *testing.T) {
	var flushed [][]byte
	var mu sync.Mutex
	c := NewOutputCoalescer(func(payload []byte) {
		mu.Lock()
		flushed = append(flushed, append([]byte(nil), payload...))
		mu.Unlock()
	})

	c.Add([]byte("pending"))
	c.Stop()

	mu.Lock()
	if len(flushed) != 1 || !bytes.Equal(flushed[0], []byte("pending")) {
		t.Errorf("expected ['pending'], got %v", flushed)
	}
	mu.Unlock()

	// After stop, Add should be ignored
	c.Add([]byte("ignored"))
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	if len(flushed) != 1 {
		t.Errorf("expected no additional flush after stop, got %d flushes", len(flushed))
	}
	mu.Unlock()
}

func TestOutputCoalescer_MultipleFlushes(t *testing.T) {
	var flushed [][]byte
	var mu sync.Mutex
	c := NewOutputCoalescer(func(payload []byte) {
		mu.Lock()
		flushed = append(flushed, append([]byte(nil), payload...))
		mu.Unlock()
	})

	c.Add([]byte("a"))
	c.Flush()
	c.Add([]byte("b"))
	c.Flush()

	mu.Lock()
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushes, got %d", len(flushed))
	}
	if !bytes.Equal(flushed[0], []byte("a")) || !bytes.Equal(flushed[1], []byte("b")) {
		t.Errorf("unexpected flushes: %v", flushed)
	}
	mu.Unlock()
}

func TestOutputCoalescer_EmptyData(t *testing.T) {
	var flushCount int
	var mu sync.Mutex
	c := NewOutputCoalescer(func(_ []byte) {
		mu.Lock()
		flushCount++
		mu.Unlock()
	})

	c.Flush()

	mu.Lock()
	if flushCount != 0 {
		t.Errorf("expected 0 flushes for empty data, got %d", flushCount)
	}
	mu.Unlock()
}
