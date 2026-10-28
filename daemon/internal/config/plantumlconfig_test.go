package config

import (
	"testing"
	"time"
)

func TestPlantUMLConfig_Timeout(t *testing.T) {
	if got := (PlantUMLConfig{}).Timeout(); got != DefaultPlantUMLTimeoutSeconds*time.Second {
		t.Errorf("zero timeout: got %v", got)
	}
	if got := (PlantUMLConfig{TimeoutSeconds: 5}).Timeout(); got != 5*time.Second {
		t.Errorf("explicit timeout: got %v", got)
	}
}

func TestPlantUMLConfig_ApplyDefaults(t *testing.T) {
	cfg := PlantUMLConfig{}
	cfg.ApplyDefaults()
	if cfg.TimeoutSeconds != DefaultPlantUMLTimeoutSeconds {
		t.Errorf("TimeoutSeconds: got %d", cfg.TimeoutSeconds)
	}

	explicit := PlantUMLConfig{TimeoutSeconds: 7}
	explicit.ApplyDefaults()
	if explicit.TimeoutSeconds != 7 {
		t.Errorf("explicit TimeoutSeconds overwritten: got %d", explicit.TimeoutSeconds)
	}
}
