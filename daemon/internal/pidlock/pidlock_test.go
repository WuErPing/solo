package pidlock

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquire_CreatesPIDFile(t *testing.T) {
	home := t.TempDir()
	release, err := Acquire(home)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	pidPath := filepath.Join(home, "solo.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
	}
}

func TestAcquire_BlocksDuplicate(t *testing.T) {
	home := t.TempDir()
	release1, err := Acquire(home)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release1()

	_, err = Acquire(home)
	if err == nil {
		t.Fatal("expected second Acquire to fail")
	}
}

func TestAcquire_CleansUpStalePID(t *testing.T) {
	home := t.TempDir()
	pidPath := filepath.Join(home, "solo.pid")
	// Write a PID that cannot be a running process (max int)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(2147483647)), 0644)

	release, err := Acquire(home)
	if err != nil {
		t.Fatalf("Acquire after stale PID: %v", err)
	}
	defer release()

	data, _ := os.ReadFile(pidPath)
	pid, _ := strconv.Atoi(string(data))
	if pid != os.Getpid() {
		t.Errorf("expected our pid after cleanup, got %d", pid)
	}
}

func TestAcquire_ReleaseAllowsReacquire(t *testing.T) {
	home := t.TempDir()
	release, err := Acquire(home)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()

	release2, err := Acquire(home)
	if err != nil {
		t.Fatalf("re-Acquire after release: %v", err)
	}
	release2()
}

func TestAcquire_CreatesSoloHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "nested", "solo")
	release, err := Acquire(home)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Error("expected solo home to be created")
	}
}
