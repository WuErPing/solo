package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func opencodeGet(ctx context.Context, baseURL, path, cwd string, result interface{}) error {
	httpCtx, cancel := context.WithTimeout(ctx, opencodeHTTPRequestTimeout)
	defer cancel()
	return opencodeGetContext(httpCtx, baseURL, path, cwd, result)
}

func opencodeGetContext(ctx context.Context, baseURL, path, cwd string, result interface{}) error {
	url := baseURL + path
	if cwd != "" {
		url += "?directory=" + cwd
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opencode API %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if result != nil {
		if err := decodeOpencodeResponse(resp.Body, result); err != nil {
			return err
		}
	}
	return nil
}

func opencodePost(ctx context.Context, baseURL, path, cwd string, body interface{}, result interface{}) error {
	httpCtx, cancel := context.WithTimeout(ctx, opencodeHTTPRequestTimeout)
	defer cancel()
	return opencodePostJSON(httpCtx, baseURL, path, cwd, body, result)
}

func opencodePostJSON(ctx context.Context, baseURL, path, cwd string, body interface{}, result interface{}) error {
	url := baseURL + path
	if cwd != "" {
		url += "?directory=" + cwd
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opencode API %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := decodeOpencodeResponse(resp.Body, result); err != nil {
			return err
		}
	}
	return nil
}

func decodeOpencodeResponse(respBody io.Reader, result interface{}) error {
	body, err := io.ReadAll(respBody)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Try to decode as object first (to check for data wrapper)
	var check map[string]json.RawMessage
	if err := json.Unmarshal(body, &check); err != nil {
		// Not an object (e.g., array), decode directly into result
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}

	if _, hasData := check["data"]; hasData {
		var wrapper struct {
			Data  json.RawMessage `json:"data"`
			Error interface{}     `json:"error"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if wrapper.Error != nil {
			return fmt.Errorf("opencode API error: %v", wrapper.Error)
		}
		if err := json.Unmarshal(wrapper.Data, result); err != nil {
			return fmt.Errorf("unmarshal data: %w", err)
		}
		return nil
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func findAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
