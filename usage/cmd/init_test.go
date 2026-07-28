package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WuErPing/solo/usage/config"
)

// withInitFlags swaps the package-level flag globals for the duration of a test.
func withInitFlags(t *testing.T, configPath string, force, print bool) {
	t.Helper()
	oldCfg, oldForce, oldPrint := cfgPath, initForce, initPrint
	cfgPath, initForce, initPrint = configPath, force, print
	t.Cleanup(func() {
		cfgPath, initForce, initPrint = oldCfg, oldForce, oldPrint
	})
}

func runInitToBuffer(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	t.Cleanup(func() { initCmd.SetOut(nil) })
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	return buf
}

func TestInitWritesTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solo", "usage.json")
	withInitFlags(t, path, false, false)

	buf := runInitToBuffer(t)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(data) != configTemplate {
		t.Error("written file does not match embedded template")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config permissions = %o, want 600", perm)
	}
	if !strings.Contains(buf.String(), "Wrote "+path) {
		t.Errorf("output missing next-step guidance, got %q", buf.String())
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := os.WriteFile(path, []byte(`{"providers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	withInitFlags(t, path, false, false)

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error when file exists without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want mention of existing file", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != `{"providers":{}}` {
		t.Error("existing file was modified without --force")
	}
}

func TestInitForceOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := os.WriteFile(path, []byte(`{"providers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	withInitFlags(t, path, true, false)

	runInitToBuffer(t)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read overwritten config: %v", err)
	}
	if string(data) != configTemplate {
		t.Error("--force did not overwrite with the template")
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config permissions after --force = %o, want 600", perm)
	}
}

func TestInitPrint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	withInitFlags(t, path, false, true)

	buf := runInitToBuffer(t)

	if buf.String() != configTemplate {
		t.Error("--print output does not match embedded template")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("--print must not write the config file")
	}
}

func TestTemplateIsValidConfig(t *testing.T) {
	var f config.File
	if err := json.Unmarshal([]byte(configTemplate), &f); err != nil {
		t.Fatalf("template is not valid JSON: %v", err)
	}
	for _, name := range []string{"kimi", "deepseek", "qoder", "xiaomimimo"} {
		entry, ok := f.Providers[name]
		if !ok {
			t.Errorf("template missing provider %q", name)
			continue
		}
		if !entry.Enabled {
			t.Errorf("template provider %q is not enabled", name)
		}
	}
	if got := f.Providers["qoder"].Extra["organizationId"]; got == "" {
		t.Error("template qoder entry missing extra.organizationId (org mode)")
	}
	if got := f.Providers["xiaomimimo"].Extra["cookie"]; got == "" {
		t.Error("template xiaomimimo entry missing extra.cookie")
	}
}
