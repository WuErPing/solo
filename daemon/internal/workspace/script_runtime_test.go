package workspace

import (
	"testing"
)

func TestScriptManager_RegisterAndGet(t *testing.T) {
	m := NewScriptManager()
	rt := &ScriptRuntime{
		WorkspaceID: "ws1",
		ScriptName:  "dev",
		Hostname:    "proj.main.dev.localhost",
		Port:        3000,
		Status:      ScriptStatusRunning,
	}
	m.Register(rt)

	got, ok := m.Get("ws1", "dev")
	if !ok {
		t.Fatal("expected script to be found")
	}
	if got.Port != 3000 {
		t.Errorf("Port: got %d, want %d", got.Port, 3000)
	}
}

func TestScriptManager_Unregister(t *testing.T) {
	m := NewScriptManager()
	m.Register(&ScriptRuntime{WorkspaceID: "ws1", ScriptName: "dev", Status: ScriptStatusRunning})
	m.Unregister("ws1", "dev")

	if _, ok := m.Get("ws1", "dev"); ok {
		t.Error("expected script to be removed")
	}
}

func TestScriptManager_ListByWorkspace(t *testing.T) {
	m := NewScriptManager()
	m.Register(&ScriptRuntime{WorkspaceID: "ws1", ScriptName: "dev", Status: ScriptStatusRunning})
	m.Register(&ScriptRuntime{WorkspaceID: "ws1", ScriptName: "test", Status: ScriptStatusRunning})
	m.Register(&ScriptRuntime{WorkspaceID: "ws2", ScriptName: "dev", Status: ScriptStatusRunning})

	list := m.ListByWorkspace("ws1")
	if len(list) != 2 {
		t.Errorf("expected 2 scripts for ws1, got %d", len(list))
	}

	list = m.ListByWorkspace("ws-missing")
	if len(list) != 0 {
		t.Errorf("expected 0 scripts for missing workspace, got %d", len(list))
	}
}

func TestScriptManager_MarkStopped(t *testing.T) {
	m := NewScriptManager()
	m.Register(&ScriptRuntime{WorkspaceID: "ws1", ScriptName: "dev", Status: ScriptStatusRunning})
	m.MarkStopped("ws1", "dev", 1)

	got, _ := m.Get("ws1", "dev")
	if got.Status != ScriptStatusStopped {
		t.Errorf("expected status stopped, got %q", got.Status)
	}
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Error("expected exit code 1")
	}
}

func TestBuildHostname(t *testing.T) {
	host := BuildHostname("myproj", "feat-auth", "dev")
	expected := "myproj.feat-auth.dev.localhost"
	if host != expected {
		t.Errorf("BuildHostname: got %q, want %q", host, expected)
	}
}

func TestAllocatePort(t *testing.T) {
	port, err := AllocatePort()
	if err != nil {
		t.Fatalf("AllocatePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("expected valid port, got %d", port)
	}

	// Verify port is actually available
	port2, err := AllocatePort()
	if err != nil {
		t.Fatalf("AllocatePort second: %v", err)
	}
	if port == port2 {
		t.Error("expected different ports")
	}
}
