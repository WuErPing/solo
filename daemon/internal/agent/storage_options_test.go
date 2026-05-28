package agent

import (
	"testing"
)

func TestWithTitleSetsOption(t *testing.T) {
	opts := &snapshotOptions{}
	fn := WithTitle("My Title")
	fn(opts)
	if opts.title == nil || *opts.title != "My Title" {
		t.Error("expected title to be set")
	}
}

func TestWithInternalSetsOption(t *testing.T) {
	opts := &snapshotOptions{}
	fn := WithInternal(true)
	fn(opts)
	if opts.internal == nil || !*opts.internal {
		t.Error("expected internal to be set to true")
	}
}
