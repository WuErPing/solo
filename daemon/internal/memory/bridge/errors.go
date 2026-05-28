package bridge

import "errors"

// errNilRecorder is returned by New when the caller passes a nil recorder.
var errNilRecorder = errors.New("bridge: recorder is required")
