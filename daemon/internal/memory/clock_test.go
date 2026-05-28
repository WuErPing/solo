package memory

import "testing"

func TestSystemClock_NowReturnsNonZero(t *testing.T) {
	got := SystemClock{}.Now()
	if got.IsZero() {
		t.Error("SystemClock.Now() returned zero time")
	}
}

func TestSystemClock_ImplementsClock(t *testing.T) {
	t.Helper()
	var _ Clock = SystemClock{}
}
