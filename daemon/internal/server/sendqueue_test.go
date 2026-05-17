package server

import (
	"sync"
	"testing"
)

func TestSendQueue_PushPop_SingleItem(t *testing.T) {
	q := newSendQueue()
	item := sendQueueItem{msgType: 1, data: []byte("hello")}
	if !q.Push(item) {
		t.Fatal("Push returned false for open queue")
	}
	got, ok := q.Pop()
	if !ok {
		t.Fatal("Pop returned false with item pending")
	}
	if got.msgType != item.msgType || string(got.data) != string(item.data) {
		t.Fatalf("Pop returned %+v, want %+v", got, item)
	}
}

func TestSendQueue_PushPop_ManyItems_FIFO(t *testing.T) {
	q := newSendQueue()
	n := 10000
	for i := 0; i < n; i++ {
		q.Push(sendQueueItem{msgType: 1, data: []byte{byte(i)}})
	}
	for i := 0; i < n; i++ {
		got, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop #%d returned false", i)
		}
		if got.data[0] != byte(i) {
			t.Fatalf("Pop #%d got %d, want %d", i, got.data[0], byte(i))
		}
	}
	if q.Len() != 0 {
		t.Fatalf("Len() = %d after draining all items, want 0", q.Len())
	}
}

func TestSendQueue_PopBlocksOnEmpty(t *testing.T) {
	q := newSendQueue()
	done := make(chan struct{})
	go func() {
		q.Pop()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("Pop returned on empty queue — should have blocked")
	default:
	}
	q.Push(sendQueueItem{msgType: 1, data: []byte("x")})
	<-done
}

func TestSendQueue_CloseWithPending_DrainsAll(t *testing.T) {
	q := newSendQueue()
	for i := 0; i < 5; i++ {
		q.Push(sendQueueItem{msgType: 1, data: []byte{byte(i)}})
	}
	q.Close()
	for i := 0; i < 5; i++ {
		_, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop #%d returned false before drain complete", i)
		}
	}
	_, ok := q.Pop()
	if ok {
		t.Fatal("Pop returned true after drain — should return false")
	}
}

func TestSendQueue_PushAfterClose_ReturnsFalse(t *testing.T) {
	q := newSendQueue()
	q.Close()
	if q.Push(sendQueueItem{msgType: 1}) {
		t.Fatal("Push returned true after Close — should return false")
	}
}

func TestSendQueue_Drain(t *testing.T) {
	q := newSendQueue()
	for i := 0; i < 5; i++ {
		q.Push(sendQueueItem{msgType: 1, data: []byte{byte(i)}})
	}
	q.Drain()
	_, ok := q.Pop()
	if ok {
		t.Fatal("Pop returned true after Drain, expected false")
	}
	if q.Len() != 0 {
		t.Fatalf("Len() = %d after Drain, want 0", q.Len())
	}
}

func TestSendQueue_IsClosed(t *testing.T) {
	q := newSendQueue()
	if q.IsClosed() {
		t.Fatal("IsClosed returned true on new queue")
	}
	q.Close()
	if !q.IsClosed() {
		t.Fatal("IsClosed returned false after Close")
	}
}

func TestSendQueue_Len(t *testing.T) {
	q := newSendQueue()
	if q.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", q.Len())
	}
	q.Push(sendQueueItem{msgType: 1})
	q.Push(sendQueueItem{msgType: 1})
	if q.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", q.Len())
	}
	q.Pop()
	if q.Len() != 1 {
		t.Fatalf("Len() = %d after Pop, want 1", q.Len())
	}
}

func TestSendQueue_ConcurrentPushPop(t *testing.T) {
	q := newSendQueue()
	var wg sync.WaitGroup
	n := 1000
	producers := 10

	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < n; j++ {
				q.Push(sendQueueItem{msgType: 1, data: []byte{byte(base)}})
			}
		}(i)
	}

	received := 0
	done := make(chan struct{})
	go func() {
		for {
			_, ok := q.Pop()
			if !ok {
				break
			}
			received++
			if received == producers*n {
				break
			}
		}
		close(done)
	}()

	wg.Wait()
	q.Close()
	<-done

	if received != producers*n {
		t.Fatalf("received %d items, want %d", received, producers*n)
	}
}

func TestSendQueue_ReadySignalNotLost(t *testing.T) {
	q := newSendQueue()
	n := 5000
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			q.Push(sendQueueItem{msgType: 1, data: []byte{byte(v)}})
		}(i)
	}
	wg.Wait()

	q.Close()
	count := 0
	for {
		_, ok := q.Pop()
		if !ok {
			break
		}
		count++
	}
	if count != n {
		t.Fatalf("drained %d items, want %d", count, n)
	}
}

func TestSendQueue_DoubleClose_NoPanic(t *testing.T) {
	q := newSendQueue()
	q.Close()
	// Second Close must not panic
	q.Close()
	_, ok := q.Pop()
	if ok {
		t.Fatal("Pop returned true after double close — should return false")
	}
}

func TestSendQueue_DrainWakesBlockedPop(t *testing.T) {
	q := newSendQueue()
	done := make(chan struct{})
	go func() {
		q.Pop()
		close(done)
	}()
	// Pop should be blocked
	select {
	case <-done:
		t.Fatal("Pop should be blocked on empty queue")
	default:
	}
	// Drain from another goroutine should wake Pop
	q.Drain()
	q.Close()
	<-done
}

func TestSendQueue_EmptyClose(t *testing.T) {
	q := newSendQueue()
	q.Close()
	_, ok := q.Pop()
	if ok {
		t.Fatal("Pop returned true on empty closed queue — should return false")
	}
}
