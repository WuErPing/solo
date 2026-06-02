package server

import (
	"fmt"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestHandleScheduleCreate_Success(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-create")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "Daily standup summary",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})

	resp := readUntilType(t, conn, "schedule/create/response")
	payload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Schedule == nil {
		t.Fatal("expected schedule in response")
	}
	if payload.Schedule.Prompt != "Daily standup summary" {
		t.Errorf("prompt mismatch: got %q", payload.Schedule.Prompt)
	}
	if payload.Schedule.Status != "active" {
		t.Errorf("expected active status, got %q", payload.Schedule.Status)
	}
}

func TestHandleScheduleCreate_ValidationError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-create-err")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-err",
			"prompt":    "",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})

	resp := readUntilType(t, conn, "schedule/create/response")
	payload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error")
	}
	if *payload.Error != "prompt is required" {
		t.Errorf("got %q, want %q", *payload.Error, "prompt is required")
	}
	if payload.Schedule != nil {
		t.Error("expected nil schedule on error")
	}
}

func TestHandleScheduleList(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-list")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create two schedules
	for i := 0; i < 2; i++ {
		conn.WriteJSON(protocol.WSInboundMessage{
			Type: "session",
			Message: mustMarshal(map[string]interface{}{
				"type":      "schedule/create",
				"requestId": fmt.Sprintf("req-create-%d", i),
				"prompt":    fmt.Sprintf("prompt %d", i),
				"cadence": map[string]interface{}{
					"type":    "every",
					"everyMs": 3600000,
				},
				"target": map[string]interface{}{
					"type":    "agent",
					"agentId": "agent-123",
				},
			}),
		})
		readUntilType(t, conn, "schedule/create/response")
	}

	// List
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/list",
			"requestId": "req-list-1",
		}),
	})

	resp := readUntilType(t, conn, "schedule/list/response")
	payload := decodeSessionPayload[protocol.ScheduleListResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if len(payload.Schedules) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(payload.Schedules))
	}
}

func TestHandleScheduleInspect_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-inspect")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "test prompt",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})
	createResp := readUntilType(t, conn, "schedule/create/response")
	createPayload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, createResp)
	scheduleID := createPayload.Schedule.ID

	// Inspect
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/inspect",
			"requestId":  "req-inspect-1",
			"scheduleId": scheduleID,
		}),
	})

	resp := readUntilType(t, conn, "schedule/inspect/response")
	payload := decodeSessionPayload[protocol.ScheduleInspectResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Schedule == nil {
		t.Fatal("expected schedule")
	}
	if payload.Schedule.ID != scheduleID {
		t.Errorf("ID mismatch")
	}
}

func TestHandleScheduleInspect_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-inspect-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/inspect",
			"requestId":  "req-inspect-nf",
			"scheduleId": "nonexistent",
		}),
	})

	resp := readUntilType(t, conn, "schedule/inspect/response")
	payload := decodeSessionPayload[protocol.ScheduleInspectResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error")
	}
	if *payload.Error != "schedule not found" {
		t.Errorf("got %q, want %q", *payload.Error, "schedule not found")
	}
}

func TestHandleSchedulePauseResumeDelete(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-lifecycle")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "test prompt",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})
	createResp := readUntilType(t, conn, "schedule/create/response")
	createPayload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, createResp)
	scheduleID := createPayload.Schedule.ID

	// Pause
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/pause",
			"requestId":  "req-pause-1",
			"scheduleId": scheduleID,
		}),
	})
	pauseResp := readUntilType(t, conn, "schedule/pause/response")
	pausePayload := decodeSessionPayload[protocol.SchedulePauseResponsePayload](t, pauseResp)
	if pausePayload.Error != nil {
		t.Fatalf("pause error: %s", *pausePayload.Error)
	}
	if pausePayload.Schedule.Status != "paused" {
		t.Errorf("expected paused, got %q", pausePayload.Schedule.Status)
	}

	// Resume
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/resume",
			"requestId":  "req-resume-1",
			"scheduleId": scheduleID,
		}),
	})
	resumeResp := readUntilType(t, conn, "schedule/resume/response")
	resumePayload := decodeSessionPayload[protocol.ScheduleResumeResponsePayload](t, resumeResp)
	if resumePayload.Error != nil {
		t.Fatalf("resume error: %s", *resumePayload.Error)
	}
	if resumePayload.Schedule.Status != "active" {
		t.Errorf("expected active, got %q", resumePayload.Schedule.Status)
	}

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/delete",
			"requestId":  "req-delete-1",
			"scheduleId": scheduleID,
		}),
	})
	deleteResp := readUntilType(t, conn, "schedule/delete/response")
	deletePayload := decodeSessionPayload[protocol.ScheduleDeleteResponsePayload](t, deleteResp)
	if deletePayload.Error != nil {
		t.Fatalf("delete error: %s", *deletePayload.Error)
	}
	if deletePayload.ScheduleID != scheduleID {
		t.Errorf("ID mismatch")
	}

	// Verify deleted
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/inspect",
			"requestId":  "req-inspect-after-delete",
			"scheduleId": scheduleID,
		}),
	})
	inspectResp := readUntilType(t, conn, "schedule/inspect/response")
	inspectPayload := decodeSessionPayload[protocol.ScheduleInspectResponsePayload](t, inspectResp)
	if inspectPayload.Error == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestHandleSchedulePause_AlreadyPaused(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-pause-twice")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "test",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})
	createResp := readUntilType(t, conn, "schedule/create/response")
	createPayload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, createResp)
	scheduleID := createPayload.Schedule.ID

	// Pause once
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/pause",
			"requestId":  "req-pause-1",
			"scheduleId": scheduleID,
		}),
	})
	readUntilType(t, conn, "schedule/pause/response")

	// Pause again
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/pause",
			"requestId":  "req-pause-2",
			"scheduleId": scheduleID,
		}),
	})
	resp := readUntilType(t, conn, "schedule/pause/response")
	payload := decodeSessionPayload[protocol.SchedulePauseResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error")
	}
	if *payload.Error != "schedule already paused" {
		t.Errorf("got %q, want %q", *payload.Error, "schedule already paused")
	}
}

func TestHandleScheduleLogs(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-logs")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "test",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})
	createResp := readUntilType(t, conn, "schedule/create/response")
	createPayload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, createResp)
	scheduleID := createPayload.Schedule.ID

	// Logs
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/logs",
			"requestId":  "req-logs-1",
			"scheduleId": scheduleID,
		}),
	})
	resp := readUntilType(t, conn, "schedule/logs/response")
	payload := decodeSessionPayload[protocol.ScheduleLogsResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Runs == nil {
		t.Fatal("expected runs")
	}
	if len(payload.Runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(payload.Runs))
	}
}

func TestHandleSchedulePersistence(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-schedule-persist")
	defer conn.Close()
	readInitialMessages(t, conn)

	// Create
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "schedule/create",
			"requestId": "req-create-1",
			"prompt":    "persist test",
			"cadence": map[string]interface{}{
				"type":    "every",
				"everyMs": 3600000,
			},
			"target": map[string]interface{}{
				"type":    "agent",
				"agentId": "agent-123",
			},
		}),
	})
	createResp := readUntilType(t, conn, "schedule/create/response")
	createPayload := decodeSessionPayload[protocol.ScheduleCreateResponsePayload](t, createResp)
	scheduleID := createPayload.Schedule.ID

	// Pause
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/pause",
			"requestId":  "req-pause-1",
			"scheduleId": scheduleID,
		}),
	})
	readUntilType(t, conn, "schedule/pause/response")

	// Reconnect and verify state persisted
	conn2 := dialAndHello(t, ts.URL, "client-schedule-persist-2")
	defer conn2.Close()
	readInitialMessages(t, conn2)

	conn2.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":       "schedule/inspect",
			"requestId":  "req-inspect-1",
			"scheduleId": scheduleID,
		}),
	})
	resp := readUntilType(t, conn2, "schedule/inspect/response")
	payload := decodeSessionPayload[protocol.ScheduleInspectResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Schedule.Status != "paused" {
		t.Errorf("expected paused after reconnect, got %q", payload.Schedule.Status)
	}
}
