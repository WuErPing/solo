package server

import (
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/WuErPing/solo/protocol"
)

func waitLoopNotRunning(t *testing.T, conn *websocket.Conn, loopID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn.WriteJSON(protocol.WSInboundMessage{
			Type: "session",
			Message: mustMarshal(map[string]interface{}{
				"type":      "loop/inspect",
				"requestId": "req-wait-" + strconv.Itoa(int(time.Now().UnixNano())),
				"id":        loopID,
			}),
		})
		resp := readUntilType(t, conn, "loop/inspect/response")
		payload := decodeSessionPayload[protocol.LoopInspectResponsePayload](t, resp)
		if payload.Error != nil {
			t.Fatalf("wait inspect error: %s", *payload.Error)
		}
		if payload.Loop != nil && payload.Loop.Status != "running" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("loop did not finish")
}

func TestHandleLoopRun_Success(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-run")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "loop/run",
			"requestId":     "req-loop-run-1",
			"prompt":        "say hello",
			"cwd":           t.TempDir(),
			"maxIterations": 0,
		}),
	})

	resp := readUntilType(t, conn, "loop/run/response")
	payload := decodeSessionPayload[protocol.LoopRunResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Loop == nil {
		t.Fatal("expected loop in response")
	}
	if payload.Loop.Prompt != "say hello" {
		t.Errorf("prompt mismatch: got %q", payload.Loop.Prompt)
	}
	if payload.Loop.Status != "running" {
		t.Errorf("expected running status, got %q", payload.Loop.Status)
	}
	waitLoopNotRunning(t, conn, payload.Loop.ID)
}

func TestHandleLoopRun_ValidationError(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-run-err")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/run",
			"requestId": "req-loop-run-err",
			"prompt":    "",
			"cwd":       t.TempDir(),
		}),
	})

	resp := readUntilType(t, conn, "loop/run/response")
	payload := decodeSessionPayload[protocol.LoopRunResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error")
	}
	if payload.Loop != nil {
		t.Error("expected nil loop on error")
	}
}

func TestHandleLoopList(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-list")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "loop/run",
			"requestId":     "req-loop-run-1",
			"prompt":        "prompt one",
			"cwd":           t.TempDir(),
			"maxIterations": 0,
		}),
	})
	createResp := readUntilType(t, conn, "loop/run/response")
	createPayload := decodeSessionPayload[protocol.LoopRunResponsePayload](t, createResp)
	if createPayload.Error != nil {
		t.Fatalf("create error: %s", *createPayload.Error)
	}
	waitLoopNotRunning(t, conn, createPayload.Loop.ID)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/list",
			"requestId": "req-loop-list-1",
		}),
	})
	resp := readUntilType(t, conn, "loop/list/response")
	payload := decodeSessionPayload[protocol.LoopListResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if len(payload.Loops) != 1 {
		t.Fatalf("expected 1 loop, got %d", len(payload.Loops))
	}
}

func TestHandleLoopInspect_Found(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-inspect")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "loop/run",
			"requestId":     "req-loop-run-1",
			"prompt":        "inspect me",
			"cwd":           t.TempDir(),
			"maxIterations": 0,
		}),
	})
	createResp := readUntilType(t, conn, "loop/run/response")
	createPayload := decodeSessionPayload[protocol.LoopRunResponsePayload](t, createResp)
	if createPayload.Error != nil {
		t.Fatalf("create error: %s", *createPayload.Error)
	}
	loopID := createPayload.Loop.ID
	waitLoopNotRunning(t, conn, loopID)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/inspect",
			"requestId": "req-loop-inspect-1",
			"id":        loopID,
		}),
	})
	resp := readUntilType(t, conn, "loop/inspect/response")
	payload := decodeSessionPayload[protocol.LoopInspectResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.Loop == nil || payload.Loop.ID != loopID {
		t.Fatal("expected loop with matching ID")
	}
}

func TestHandleLoopInspect_NotFound(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-inspect-nf")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/inspect",
			"requestId": "req-loop-inspect-nf",
			"id":        "nonexistent",
		}),
	})
	resp := readUntilType(t, conn, "loop/inspect/response")
	payload := decodeSessionPayload[protocol.LoopInspectResponsePayload](t, resp)
	if payload.Error == nil {
		t.Fatal("expected error")
	}
	if *payload.Error != "loop not found" {
		t.Errorf("got %q, want %q", *payload.Error, "loop not found")
	}
}

func TestHandleLoopUpdateDelete(t *testing.T) {
	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-loop-lifecycle")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":          "loop/run",
			"requestId":     "req-loop-run-1",
			"prompt":        "lifecycle",
			"cwd":           t.TempDir(),
			"maxIterations": 0,
		}),
	})
	createResp := readUntilType(t, conn, "loop/run/response")
	createPayload := decodeSessionPayload[protocol.LoopRunResponsePayload](t, createResp)
	if createPayload.Error != nil {
		t.Fatalf("create error: %s", *createPayload.Error)
	}
	loopID := createPayload.Loop.ID
	waitLoopNotRunning(t, conn, loopID)

	// Update name
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/update",
			"requestId": "req-loop-update-1",
			"id":        loopID,
			"name":      "renamed loop",
		}),
	})
	updateResp := readUntilType(t, conn, "loop/update/response")
	updatePayload := decodeSessionPayload[protocol.LoopUpdateResponsePayload](t, updateResp)
	if updatePayload.Error != nil {
		t.Fatalf("update error: %s", *updatePayload.Error)
	}
	if updatePayload.Loop == nil || updatePayload.Loop.Name == nil || *updatePayload.Loop.Name != "renamed loop" {
		t.Error("expected name to be updated")
	}

	// Delete
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/delete",
			"requestId": "req-loop-delete-1",
			"id":        loopID,
		}),
	})
	deleteResp := readUntilType(t, conn, "loop/delete/response")
	deletePayload := decodeSessionPayload[protocol.LoopDeleteResponsePayload](t, deleteResp)
	if deletePayload.Error != nil {
		t.Fatalf("delete error: %s", *deletePayload.Error)
	}
	if deletePayload.ID != loopID {
		t.Errorf("ID mismatch")
	}

	// Verify deleted
	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "loop/inspect",
			"requestId": "req-loop-inspect-after-delete",
			"id":        loopID,
		}),
	})
	inspectResp := readUntilType(t, conn, "loop/inspect/response")
	inspectPayload := decodeSessionPayload[protocol.LoopInspectResponsePayload](t, inspectResp)
	if inspectPayload.Error == nil {
		t.Fatal("expected not found after delete")
	}
}
