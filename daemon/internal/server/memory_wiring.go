package server

// MemoryFeature is the object returned by a registered memory builder.
// It pairs the bridge injected into sessions with the recorder the
// daemon must close on shutdown.
type MemoryFeature struct {
	Bridge   MemoryBridge
	Recorder MemoryRecorder
}

// MemoryFeatureBuilder constructs a MemoryFeature from a MemoryConfig.
// It is registered at init time by the memorysetup package so that the
// server module does not need to import concrete memory implementations.
//
// A nil builder (the default) means the memory feature is unavailable,
// in which case buildMemoryFeature returns (nil, nil).
type MemoryFeatureBuilder func(cfg interface{}) (*MemoryFeature, error)

// memoryFeatureBuilder is the registered builder. Access only through
// RegisterMemoryFeatureBuilder / buildMemoryFeature.
var memoryFeatureBuilder MemoryFeatureBuilder

// RegisterMemoryFeatureBuilder installs the package-level builder. It
// is intended to be called from an init() in a wiring file that imports
// the concrete memory implementation.
func RegisterMemoryFeatureBuilder(b MemoryFeatureBuilder) {
	memoryFeatureBuilder = b
}

// buildMemoryFeature invokes the registered builder, forwarding cfg as
// an interface{} so this package does not depend on config.MemoryConfig
// directly beyond the field on config.Config.
func buildMemoryFeature(cfg interface{}) (*MemoryFeature, error) {
	if memoryFeatureBuilder == nil {
		return nil, nil
	}
	return memoryFeatureBuilder(cfg)
}
