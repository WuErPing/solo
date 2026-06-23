package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpencodeGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if r.URL.Query().Get("directory") != "/cwd" {
			t.Error("expected directory query param")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var result map[string]string
	if err := opencodeGet(ctx, srv.URL, "/test", "/cwd", &result); err != nil {
		t.Fatalf("opencodeGet: %v", err)
	}
	if result["result"] != "ok" {
		t.Errorf("result: got %v", result)
	}
}

func TestOpencodeGet_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	defer cancel()

	err := opencodeGet(ctx, srv.URL, "/test", "", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestOpencodePost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "value" {
			t.Errorf("body: got %v", body)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "posted"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var result map[string]string
	if err := opencodePost(ctx, srv.URL, "/test", "", map[string]string{"key": "value"}, &result); err != nil {
		t.Fatalf("opencodePost: %v", err)
	}
	if result["result"] != "posted" {
		t.Errorf("result: got %v", result)
	}
}

func TestOpencodePost_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := opencodePost(ctx, srv.URL, "/test", "", nil, nil); err != nil {
		t.Fatalf("opencodePost: %v", err)
	}
}

func TestDecodeOpencodeResponse_DirectArray(t *testing.T) {
	body := strings.NewReader(`[{"id":"1"},{"id":"2"}]`)
	var result []map[string]string
	if err := decodeOpencodeResponse(body, &result); err != nil {
		t.Fatalf("decodeOpencodeResponse: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
}

func TestDecodeOpencodeResponse_DataWrapper(t *testing.T) {
	body := strings.NewReader(`{"data":{"name":"test"},"error":null}`)
	var result map[string]string
	if err := decodeOpencodeResponse(body, &result); err != nil {
		t.Fatalf("decodeOpencodeResponse: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("result: got %v", result)
	}
}

func TestDecodeOpencodeResponse_ErrorWrapper(t *testing.T) {
	body := strings.NewReader(`{"data":null,"error":"something went wrong"}`)
	var result map[string]string
	err := decodeOpencodeResponse(body, &result)
	if err == nil {
		t.Fatal("expected error for wrapper error")
	}
}

func TestDecodeOpencodeResponse_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not json`)
	var result map[string]string
	err := decodeOpencodeResponse(body, &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
