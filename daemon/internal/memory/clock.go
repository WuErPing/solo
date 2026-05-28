package memory

import "time"

// Clock is the minimal time source used by memory components. Production
// code uses SystemClock; tests inject a fake.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the real wall clock.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }
