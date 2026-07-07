package loop

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestListTemplates_GroupsLegacyRecordsByName(t *testing.T) {
	store := NewStore()

	// Create 4 legacy records with same name/cwd (simulating re-runs of same loop)
	name := "Fix CI"
	cwd := "/Users/test/project"
	
	records := []*protocol.LoopRecord{
		{
			ID:        "record1",
			Name:      &name,
			Cwd:       cwd,
			Status:    "running",
			CreatedAt: time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:        "record2",
			Name:      &name,
			Cwd:       cwd,
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:        "record3",
			Name:      &name,
			Cwd:       cwd,
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:        "record4",
			Name:      &name,
			Cwd:       cwd,
			Status:    "failed",
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	for _, r := range records {
		store.records[r.ID] = r
	}

	templates := store.ListTemplates()

	// Should group into 1 template with 4 instances
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
		for i, tmpl := range templates {
			t.Logf("  template %d: id=%s, name=%s, instanceCount=%d", i, tmpl.ID, tmpl.Name, tmpl.InstanceCount)
		}
	}

	if len(templates) > 0 {
		if templates[0].InstanceCount != 4 {
			t.Errorf("expected 4 instances, got %d", templates[0].InstanceCount)
		}
		if templates[0].Name != name {
			t.Errorf("expected name %q, got %q", name, templates[0].Name)
		}
	}
}

func TestMigration_GroupsLegacyRecordsByName(t *testing.T) {
	store := NewStore()

	name := "Fix CI"
	cwd := "/Users/test/project"

	// Simulate legacy records without templateID
	store.records = map[string]*protocol.LoopRecord{
		"record1": {ID: "record1", Name: &name, Cwd: cwd, Status: "running"},
		"record2": {ID: "record2", Name: &name, Cwd: cwd, Status: "succeeded"},
		"record3": {ID: "record3", Name: &name, Cwd: cwd, Status: "failed"},
	}

	// Run migration logic directly
	templateGroups := make(map[string]string)
	for _, record := range store.records {
		if record.TemplateID == "" {
			name := ""
			if record.Name != nil {
				name = *record.Name
			}
			key := name + "|" + record.Cwd
			
			templateID, exists := templateGroups[key]
			if !exists {
				templateID = "test-" + key
				templateGroups[key] = templateID
			}
			
			record.TemplateID = templateID
		}
	}

	// Check that all records with same name+cwd share the same templateID
	templateIDs := make(map[string]bool)
	for _, r := range store.records {
		if r.TemplateID == "" {
			t.Errorf("record %s has empty templateID after migration", r.ID)
		}
		templateIDs[r.TemplateID] = true
	}

	if len(templateIDs) != 1 {
		t.Errorf("expected 1 distinct templateID, got %d", len(templateIDs))
		for tid := range templateIDs {
			t.Logf("  templateID: %s", tid)
		}
	}
}

func TestMigration_RegroupsRecordsWithDifferentTemplateIDs(t *testing.T) {
	store := NewStore()

	name := "Fix CI"
	cwd := "/Users/test/project"

	// Simulate records that were incorrectly assigned different templateIDs
	// (from a previous bad migration)
	store.records = map[string]*protocol.LoopRecord{
		"record1": {ID: "record1", Name: &name, Cwd: cwd, TemplateID: "template-a", Status: "running"},
		"record2": {ID: "record2", Name: &name, Cwd: cwd, TemplateID: "template-b", Status: "succeeded"},
		"record3": {ID: "record3", Name: &name, Cwd: cwd, TemplateID: "template-c", Status: "failed"},
	}

	// Run re-migration logic (same as in load())
	templateGroups := make(map[string]string)
	for _, record := range store.records {
		key := name + "|" + cwd
		if record.TemplateID == "" {
			templateID := templateGroups[key]
			if templateID == "" {
				templateID = "new-template"
				templateGroups[key] = templateID
			}
			record.TemplateID = templateID
		} else {
			expectedTID, exists := templateGroups[key]
			if !exists {
				templateGroups[key] = record.TemplateID
			} else if record.TemplateID != expectedTID {
				record.TemplateID = expectedTID
			}
		}
	}

	// Check that all records now share the same templateID
	templateIDs := make(map[string]bool)
	for _, r := range store.records {
		templateIDs[r.TemplateID] = true
	}

	if len(templateIDs) != 1 {
		t.Errorf("expected 1 distinct templateID after regrouping, got %d", len(templateIDs))
		for tid := range templateIDs {
			t.Logf("  templateID: %s", tid)
		}
	}

	// Verify ListTemplates returns 1 template
	templates := store.ListTemplates()
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}
	if len(templates) > 0 && templates[0].InstanceCount != 3 {
		t.Errorf("expected 3 instances, got %d", templates[0].InstanceCount)
	}
}
