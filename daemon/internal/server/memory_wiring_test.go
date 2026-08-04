package server

import (
	"errors"
	"testing"
)

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
