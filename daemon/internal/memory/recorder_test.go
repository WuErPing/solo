package memory

import (
	"errors"
	"testing"
)

// ---------- ErrClosed sentinel ----------

func TestErrClosed_NonNilAndStable(t *testing.T) {
	if ErrClosed == nil {
		t.Fatal("ErrClosed must be non-nil")
	}
	// errors.Is identity must be preserved across packages.
	if !errors.Is(ErrClosed, ErrClosed) {
		t.Error("errors.Is(ErrClosed, ErrClosed) must be true")
	}
}
