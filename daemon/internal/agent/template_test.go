package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestAgentTemplate(t *testing.T) {
	cfg := AgentTemplate(nil)
	if cfg.Provider != "" || cfg.Cwd != "" {
		t.Errorf("nil template: got provider=%q cwd=%q, want zero value", cfg.Provider, cfg.Cwd)
	}

	tmpl := &protocol.AgentTemplate{
		Provider:     "claude",
		Cwd:          "/tmp",
		SystemPrompt: "be concise",
	}
	cfg = AgentTemplate(tmpl)
	if cfg.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", cfg.Provider)
	}
	if cfg.Cwd != "/tmp" {
		t.Errorf("cwd: got %q, want /tmp", cfg.Cwd)
	}
	if cfg.SystemPrompt != "be concise" {
		t.Errorf("systemPrompt: got %q, want be concise", cfg.SystemPrompt)
	}
}

func TestLoopRecordToWorkerConfig(t *testing.T) {
	model := "claude-3-opus"
	workerModel := "kimi-k2"

	tests := []struct {
		name string
		r    protocol.LoopRecord
		want protocol.AgentSessionConfig
	}{
		{
			name: "worker template wins",
			r: protocol.LoopRecord{
				Cwd: "/project",
				AgentTemplate: &protocol.AgentTemplate{
					Provider: "claude",
				},
				WorkerAgentTemplate: &protocol.AgentTemplate{
					Provider: "kimi",
					Model:    &workerModel,
				},
			},
			want: protocol.AgentSessionConfig{
				Provider: "kimi",
				Cwd:      "/project",
				Model:    &workerModel,
			},
		},
		{
			name: "base template used when worker template absent",
			r: protocol.LoopRecord{
				Cwd: "/project",
				AgentTemplate: &protocol.AgentTemplate{
					Provider:     "claude",
					Model:        &model,
					SystemPrompt: "base prompt",
				},
			},
			want: protocol.AgentSessionConfig{
				Provider:     "claude",
				Cwd:          "/project",
				Model:        &model,
				SystemPrompt: "base prompt",
			},
		},
		{
			name: "legacy worker provider/model override",
			r: protocol.LoopRecord{
				Cwd:      "/project",
				Provider: "claude",
				Model:    &model,
				WorkerProvider: func() *string {
					s := "kimi"
					return &s
				}(),
				WorkerModel: &workerModel,
			},
			want: protocol.AgentSessionConfig{
				Provider: "kimi",
				Cwd:      "/project",
				Model:    &workerModel,
			},
		},
		{
			name: "legacy provider/model fallback",
			r: protocol.LoopRecord{
				Cwd:      "/project",
				Provider: "claude",
				Model:    &model,
			},
			want: protocol.AgentSessionConfig{
				Provider: "claude",
				Cwd:      "/project",
				Model:    &model,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LoopRecordToWorkerConfig(tt.r)
			if got.Provider != tt.want.Provider {
				t.Errorf("provider: got %q, want %q", got.Provider, tt.want.Provider)
			}
			if got.Cwd != tt.want.Cwd {
				t.Errorf("cwd: got %q, want %q", got.Cwd, tt.want.Cwd)
			}
			if (got.Model == nil) != (tt.want.Model == nil) || (got.Model != nil && tt.want.Model != nil && *got.Model != *tt.want.Model) {
				t.Errorf("model: got %v, want %v", got.Model, tt.want.Model)
			}
			if got.SystemPrompt != tt.want.SystemPrompt {
				t.Errorf("systemPrompt: got %q, want %q", got.SystemPrompt, tt.want.SystemPrompt)
			}
		})
	}
}

func TestLoopRecordToVerifierConfig(t *testing.T) {
	verifierModel := "deepseek-chat"
	r := protocol.LoopRecord{
		Cwd:      "/project",
		Provider: "claude",
		VerifierAgentTemplate: &protocol.AgentTemplate{
			Provider:     "opencode",
			Model:        &verifierModel,
			SystemPrompt: "verify carefully",
		},
	}

	got := LoopRecordToVerifierConfig(r)
	if got.Provider != "opencode" {
		t.Errorf("provider: got %q, want opencode", got.Provider)
	}
	if got.Model == nil || *got.Model != verifierModel {
		t.Errorf("model: got %v, want %v", got.Model, &verifierModel)
	}
	if got.SystemPrompt != "verify carefully" {
		t.Errorf("systemPrompt: got %q, want verify carefully", got.SystemPrompt)
	}
}

func TestScheduleTargetToConfig(t *testing.T) {
	model := "claude-3-5-sonnet"

	tests := []struct {
		name    string
		target  protocol.ScheduleTarget
		cwd     string
		want    protocol.AgentSessionConfig
		wantErr bool
	}{
		{
			name: "new-agent with full template",
			target: protocol.ScheduleTarget{
				Type: "new-agent",
				Config: &protocol.AgentTemplate{
					Provider:     "claude",
					Cwd:          "/config-cwd",
					Model:        &model,
					SystemPrompt: "scheduled task",
				},
			},
			cwd: "",
			want: protocol.AgentSessionConfig{
				Provider:     "claude",
				Cwd:          "/config-cwd",
				Model:        &model,
				SystemPrompt: "scheduled task",
			},
		},
		{
			name: "new-agent uses schedule cwd when template cwd is empty",
			target: protocol.ScheduleTarget{
				Type: "new-agent",
				Config: &protocol.AgentTemplate{
					Provider: "claude",
				},
			},
			cwd: "/schedule-cwd",
			want: protocol.AgentSessionConfig{
				Provider: "claude",
				Cwd:      "/schedule-cwd",
			},
		},
		{
			name: "new-agent schedule cwd overrides template cwd",
			target: protocol.ScheduleTarget{
				Type: "new-agent",
				Config: &protocol.AgentTemplate{
					Provider: "claude",
					Cwd:      "/config-cwd",
				},
			},
			cwd: "/schedule-cwd",
			want: protocol.AgentSessionConfig{
				Provider: "claude",
				Cwd:      "/schedule-cwd",
			},
		},
		{
			name: "provider target",
			target: protocol.ScheduleTarget{
				Type:       "provider",
				ProviderID: "kimi",
			},
			cwd: "/schedule-cwd",
			want: protocol.AgentSessionConfig{
				Provider: "kimi",
				Cwd:      "/schedule-cwd",
			},
		},
		{
			name: "agent target returns error",
			target: protocol.ScheduleTarget{
				Type:    "agent",
				AgentID: "existing",
			},
			cwd:     "/schedule-cwd",
			wantErr: true,
		},
		{
			name: "new-agent without config returns error",
			target: protocol.ScheduleTarget{
				Type: "new-agent",
			},
			cwd:     "/schedule-cwd",
			wantErr: true,
		},
		{
			name: "provider without providerId returns error",
			target: protocol.ScheduleTarget{
				Type: "provider",
			},
			cwd:     "/schedule-cwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScheduleTargetToConfig(tt.target, tt.cwd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error: got %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Provider != tt.want.Provider {
				t.Errorf("provider: got %q, want %q", got.Provider, tt.want.Provider)
			}
			if got.Cwd != tt.want.Cwd {
				t.Errorf("cwd: got %q, want %q", got.Cwd, tt.want.Cwd)
			}
			if (got.Model == nil) != (tt.want.Model == nil) || (got.Model != nil && tt.want.Model != nil && *got.Model != *tt.want.Model) {
				t.Errorf("model: got %v, want %v", got.Model, tt.want.Model)
			}
			if got.SystemPrompt != tt.want.SystemPrompt {
				t.Errorf("systemPrompt: got %q, want %q", got.SystemPrompt, tt.want.SystemPrompt)
			}
		})
	}
}

func TestLoopRecordToWorkerConfigDefaultsPermissionMode(t *testing.T) {
	plan := "plan"

	// want == "" means ModeID should be left unset (nil).
	tests := []struct {
		name string
		r    protocol.LoopRecord
		want string
	}{
		{
			name: "claude defaults to its most autonomous mode (bypassPermissions)",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "claude"},
			},
			want: "bypassPermissions",
		},
		{
			name: "claude legacy provider path also gets bypassPermissions",
			r: protocol.LoopRecord{
				Cwd:      "/project",
				Provider: "claude",
			},
			want: "bypassPermissions",
		},
		{
			name: "kimi defaults to bypassPermissions (maps to --yolo)",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "kimi"},
			},
			want: "bypassPermissions",
		},
		{
			name: "codex defaults to full-access",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "codex"},
			},
			want: "full-access",
		},
		{
			name: "opencode defaults to build",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "opencode"},
			},
			want: "build",
		},
		{
			name: "pi defaults to default (its autonomous mode)",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "pi"},
			},
			want: "default",
		},
		{
			name: "preserves an explicit mode from the template",
			r: protocol.LoopRecord{
				Cwd: "/project",
				AgentTemplate: &protocol.AgentTemplate{
					Provider: "claude",
					ModeID:   &plan,
				},
			},
			want: "plan",
		},
		{
			name: "unknown provider leaves mode unset",
			r: protocol.LoopRecord{
				Cwd:           "/project",
				AgentTemplate: &protocol.AgentTemplate{Provider: "totally-unknown"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LoopRecordToWorkerConfig(tt.r)
			if tt.want == "" {
				if got.ModeID != nil {
					t.Fatalf("ModeID: got %q, want unset", *got.ModeID)
				}
				return
			}
			if got.ModeID == nil {
				t.Fatalf("ModeID is nil, want %q", tt.want)
			}
			if *got.ModeID != tt.want {
				t.Errorf("ModeID: got %q, want %q", *got.ModeID, tt.want)
			}
		})
	}
}

func TestAutonomousModeForProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"claude", "bypassPermissions"},
		{"kimi", "bypassPermissions"},
		{"codex", "full-access"},
		{"opencode", "build"},
		{"pi", "default"},
		{"mock", ""},
		{"", ""},
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := autonomousModeForProvider(tt.provider); got != tt.want {
				t.Errorf("autonomousModeForProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
