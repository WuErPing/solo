package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProjectConfig_SoloJson(t *testing.T) {
	dir := t.TempDir()
	soloData := []byte(`{"worktree":{"setup":["echo hello"]},"scripts":{"dev":{"type":"service","command":"npm run dev","port":3000}}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), soloData, 0644)

	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if len(cfg.Worktree.Setup) != 1 || cfg.Worktree.Setup[0] != "echo hello" {
		t.Errorf("expected solo.json setup, got %v", cfg.Worktree.Setup)
	}
}

func TestReadProjectConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for missing files, got %+v", cfg)
	}
}

func TestReadProjectConfig_StringSetup(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"setup":"echo single"}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if len(cfg.Worktree.Setup) != 1 || cfg.Worktree.Setup[0] != "echo single" {
		t.Errorf("expected string normalized to array, got %v", cfg.Worktree.Setup)
	}
}

func TestReadProjectConfig_StringArraySetup(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"setup":["echo one","echo two"]}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if len(cfg.Worktree.Setup) != 2 {
		t.Errorf("expected 2 setup commands, got %d", len(cfg.Worktree.Setup))
	}
}

func TestReadProjectConfig_Teardown(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"teardown":["echo cleanup"]}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if len(cfg.Worktree.Teardown) != 1 || cfg.Worktree.Teardown[0] != "echo cleanup" {
		t.Errorf("expected teardown, got %v", cfg.Worktree.Teardown)
	}
}

func TestReadProjectConfig_Scripts(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"scripts":{"dev":{"type":"service","command":"npm run dev","port":3000},"build":{"command":"npm run build"}}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if len(cfg.Scripts) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(cfg.Scripts))
	}
	dev, ok := cfg.Scripts["dev"]
	if !ok || dev.Type != "service" || dev.Command != "npm run dev" {
		t.Errorf("dev script mismatch: %+v", dev)
	}
	if dev.Port == nil || *dev.Port != 3000 {
		t.Error("expected dev port 3000")
	}
}

func TestReadRawSoloConfigForEdit(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"setup":["echo hello"]}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	result, err := ReadRawSoloConfigForEdit(dir)
	if err != nil {
		t.Fatalf("ReadRawSoloConfigForEdit: %v", err)
	}
	if result.Config == nil {
		t.Fatal("expected config map")
	}
	if result.Revision == nil {
		t.Fatal("expected revision")
	}
}

func TestReadRawSoloConfigForEdit_Missing(t *testing.T) {
	dir := t.TempDir()
	result, err := ReadRawSoloConfigForEdit(dir)
	if err != nil {
		t.Fatalf("ReadRawSoloConfigForEdit: %v", err)
	}
	if result.Config != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestWriteRawSoloConfigForEdit(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"setup":["echo hello"]}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	readResult, _ := ReadRawSoloConfigForEdit(dir)
	rev := readResult.Revision

	newCfg := map[string]interface{}{"worktree": map[string]interface{}{"setup": []string{"echo updated"}}}
	writeResult, err := WriteRawSoloConfigForEdit(dir, newCfg, rev)
	if err != nil {
		t.Fatalf("WriteRawSoloConfigForEdit: %v", err)
	}
	if writeResult.Revision == nil {
		t.Error("expected new revision")
	}

	// Verify written content
	cfg, err := ReadProjectConfig(dir)
	if err != nil {
		t.Fatalf("ReadProjectConfig after write: %v", err)
	}
	if cfg == nil || cfg.Worktree == nil || len(cfg.Worktree.Setup) != 1 || cfg.Worktree.Setup[0] != "echo updated" {
		t.Fatalf("expected updated config, got %+v", cfg)
	}
}

func TestWriteRawSoloConfigForEdit_StaleRevision(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"worktree":{"setup":["echo hello"]}}`)
	_ = os.WriteFile(filepath.Join(dir, "solo.json"), data, 0644)

	wrongRev := &ProjectConfigRevision{MtimeMs: 0, Size: 0}
	_, err := WriteRawSoloConfigForEdit(dir, map[string]interface{}{}, wrongRev)
	if err == nil {
		t.Fatal("expected stale revision error")
	}
	if _, ok := err.(*StaleProjectConfigError); !ok {
		t.Fatalf("expected StaleProjectConfigError, got %T", err)
	}
}

func TestProjectConfigRevisionsEqual(t *testing.T) {
	a := &ProjectConfigRevision{MtimeMs: 100.5, Size: 200}
	b := &ProjectConfigRevision{MtimeMs: 100.5, Size: 200}
	c := &ProjectConfigRevision{MtimeMs: 100.5, Size: 201}

	if !projectConfigRevisionsEqual(a, b) {
		t.Error("expected equal revisions")
	}
	if projectConfigRevisionsEqual(a, c) {
		t.Error("expected unequal revisions")
	}
	if !projectConfigRevisionsEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
	if projectConfigRevisionsEqual(a, nil) {
		t.Error("expected nil != non-nil")
	}
}

func TestNormalizeLifecycleCommands(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		expected []string
	}{
		{"empty", ``, nil},
		{"string", `"echo hello"`, []string{"echo hello"}},
		{"array", `["echo one","echo two"]`, []string{"echo one", "echo two"}},
		{"invalid", `123`, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLifecycleCommands([]byte(tc.raw))
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tc.expected[i], got[i])
				}
			}
		})
	}
}
