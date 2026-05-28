package util

import (
	"runtime"
	"testing"
)

func TestIsSameOrDescendantPath_Basic(t *testing.T) {
	if !IsSameOrDescendantPath("/home/user", "/home/user") {
		t.Error("expected same path to be true")
	}
	if !IsSameOrDescendantPath("/home/user", "/home/user/projects") {
		t.Error("expected descendant path to be true")
	}
	if IsSameOrDescendantPath("/home/user", "/home/user2") {
		t.Error("expected non-descendant to be false")
	}
	if IsSameOrDescendantPath("/home/user", "/home/user2/projects") {
		t.Error("expected non-descendant to be false")
	}
}

func TestIsSameOrDescendantPath_Empty(t *testing.T) {
	if IsSameOrDescendantPath("", "/home") {
		t.Error("expected empty parent to be false")
	}
	if IsSameOrDescendantPath("/home", "") {
		t.Error("expected empty child to be false")
	}
}

func TestIsSameOrDescendantPath_TrailingSlash(t *testing.T) {
	if !IsSameOrDescendantPath("/home/user/", "/home/user/projects") {
		t.Error("expected trailing slash parent to match")
	}
	if !IsSameOrDescendantPath("/home/user", "/home/user/projects/") {
		t.Error("expected trailing slash child to match")
	}
}

func TestIsSameOrDescendantPath_WindowsCase(t *testing.T) {
	if runtime.GOOS == "windows" {
		if !IsSameOrDescendantPath("C:\\Users", "C:\\Users\\Docs") {
			t.Error("expected Windows case-insensitive match")
		}
		if !IsSameOrDescendantPath("C:\\Users", "c:\\users\\docs") {
			t.Error("expected Windows lowercase match")
		}
	} else {
		if IsSameOrDescendantPath("/home/User", "/home/user/projects") {
			t.Error("expected case-sensitive mismatch on non-Windows")
		}
	}
}

func TestIsSameOrDescendantPath_NotPrefix(t *testing.T) {
	// /home/user is a prefix of /home/user2 but not a path prefix
	if IsSameOrDescendantPath("/home/user", "/home/user2") {
		t.Error("expected sibling directory to be false")
	}
}
