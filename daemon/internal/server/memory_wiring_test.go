package server

import (
	"context"
	"errors"
	"testing"
)

// stubMemoryBridge implements MemoryBridge with no-ops for testing.
type stubMemoryBridge struct{}

func (stubMemoryBridge) OnUserTurn(_, _, _ string)       {}
func (stubMemoryBridge) OnAssistantTurn(_, _, _ string)  {}
func (stubMemoryBridge) OnAssistantChunk(_, _, _ string) {}
func (stubMemoryBridge) OnAssistantTurnEnd(_, _ string)  {}
func (stubMemoryBridge) OnSystemTurn(_, _, _ string)     {}
func (stubMemoryBridge) Close() error                    { return nil }

// stubMemoryRecorder implements MemoryRecorder with no-ops for testing.
type stubMemoryRecorder struct{}

func (stubMemoryRecorder) Flush(_ context.Context) error { return nil }
func (stubMemoryRecorder) Close() error                  { return nil }

func TestBuildMemoryFeature_NilBuilder(t *testing.T) {
	orig := memoryFeatureBuilder
	memoryFeatureBuilder = nil
	defer func() { memoryFeatureBuilder = orig }()

	feat, err := buildMemoryFeature(nil)
	if feat != nil {
		t.Errorf("expected nil feature, got %v", feat)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestBuildMemoryFeature_WithBuilder(t *testing.T) {
	orig := memoryFeatureBuilder
	defer func() { memoryFeatureBuilder = orig }()

	expectedFeature := &MemoryFeature{
		Bridge:   stubMemoryBridge{},
		Recorder: stubMemoryRecorder{},
	}
	RegisterMemoryFeatureBuilder(func(cfg interface{}) (*MemoryFeature, error) {
		if cfg == nil {
			t.Error("expected non-nil cfg")
		}
		return expectedFeature, nil
	})

	feat, err := buildMemoryFeature("test-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if feat != expectedFeature {
		t.Error("expected feature returned from builder")
	}
}

func TestBuildMemoryFeature_BuilderError(t *testing.T) {
	orig := memoryFeatureBuilder
	defer func() { memoryFeatureBuilder = orig }()

	testErr := errors.New("builder failed")
	RegisterMemoryFeatureBuilder(func(_ interface{}) (*MemoryFeature, error) {
		return nil, testErr
	})

	feat, err := buildMemoryFeature("test-config")
	if feat != nil {
		t.Errorf("expected nil feature on error, got %v", feat)
	}
	if err != testErr {
		t.Errorf("expected builder error, got %v", err)
	}
}

func TestRegisterMemoryFeatureBuilder_Overwrite(t *testing.T) {
	orig := memoryFeatureBuilder
	defer func() { memoryFeatureBuilder = orig }()

	callCount := 0
	RegisterMemoryFeatureBuilder(func(_ interface{}) (*MemoryFeature, error) {
		callCount++
		return nil, nil
	})
	RegisterMemoryFeatureBuilder(func(_ interface{}) (*MemoryFeature, error) {
		callCount += 10
		return nil, nil
	})

	buildMemoryFeature("test")
	if callCount != 10 {
		t.Errorf("expected second builder to be called (count=10), got %d", callCount)
	}
}
