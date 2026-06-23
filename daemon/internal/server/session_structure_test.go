package server

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// handlerFiles enforces the R5 responsibility-split goal: each domain handler
// file must stay under 500 lines so the Session struct does not accumulate
// unrelated concerns.
var handlerFiles = map[string]int{
	"session_terminal.go": 500,
	"session_tmux.go":     500,
	"session_schedule.go": 500,
}

func TestHandlerFilesUnderLineLimit(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	dir := filepath.Dir(testFile)

	for name, limit := range handlerFiles {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer func() { _ = f.Close() }()

			lines := 0
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				lines++
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan %s: %v", name, err)
			}
			if lines >= limit {
				t.Errorf("%s has %d lines, expected < %d; split domain logic into focused files", name, lines, limit)
			}
		})
	}
}
