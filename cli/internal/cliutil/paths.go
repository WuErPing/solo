package cliutil

import (
	"path/filepath"
	"runtime"
	"strings"
)

// IsSameOrDescendantPath checks if child is the same directory as or inside parent.
func IsSameOrDescendantPath(parent, child string) bool {
	if parent == "" || child == "" {
		return false
	}

	// Normalize separators
	p := filepath.ToSlash(parent)
	c := filepath.ToSlash(child)

	// Strip trailing slashes
	p = strings.TrimRight(p, "/")
	c = strings.TrimRight(c, "/")

	// Windows paths are case-insensitive
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
		c = strings.ToLower(c)
	}

	if c == p {
		return true
	}
	return strings.HasPrefix(c, p+"/")
}
