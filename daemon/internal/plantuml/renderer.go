// Package plantuml renders PlantUML diagrams on the daemon host.
//
// PlantUML has no pure-JS renderer, and sending diagram source to a public
// service would leak code structure, so rendering stays local: either an
// installed plantuml executable (stdin -> SVG on stdout) or a PlantUML-
// compatible server the user runs themselves (source POSTed to <url>/svg).
package plantuml

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/httpx"
)

const (
	// maxSourceBytes keeps a single fence from monopolising the renderer.
	maxSourceBytes = 256 << 10
	// maxSVGBytes bounds both the exec stdout and the HTTP response body.
	maxSVGBytes = 8 << 20
	// maxCacheEntries and maxCacheBytes bound the process-wide result cache.
	maxCacheEntries = 64
	maxCacheBytes   = 8 << 20
	// maxStderrBytes keeps failure messages actionable without huge payloads.
	maxStderrBytes = 4 << 10
)

const commandEnvVar = "SOLO_PLANTUML_BIN"

const unavailableMessage = "PlantUML renderer unavailable: install PlantUML (e.g. brew install plantuml) or set plantuml.serverUrl in ~/.solo/config.json"

type backendKind int

const (
	backendUnavailable backendKind = iota
	backendCommand
	backendServer
)

type cacheEntry struct {
	key string
	svg string
}

// Renderer converts PlantUML source to SVG using a local backend. It is safe
// for concurrent use; identical in-flight renders are coalesced.
type Renderer struct {
	cfg         config.PlantUMLConfig
	logger      *slog.Logger
	backend     backendKind
	command     string
	serverURL   string
	unavailable string
	httpClient  *http.Client
	group       singleflight.Group

	mu         sync.Mutex
	lru        *list.List
	entries    map[string]*list.Element
	cacheBytes int
}

// NewRenderer resolves the configured backend once. Resolution never fails
// the daemon: when no renderer is available, Render returns an actionable
// error and the Markdown preview shows the raw diagram source instead.
func NewRenderer(cfg config.PlantUMLConfig, logger *slog.Logger) *Renderer {
	cfg.ApplyDefaults()
	if logger == nil {
		logger = slog.Default()
	}
	r := &Renderer{
		cfg:        cfg,
		logger:     logger,
		httpClient: httpx.Standard(),
		lru:        list.New(),
		entries:    make(map[string]*list.Element),
	}
	r.resolveBackend()
	return r
}

func (r *Renderer) resolveBackend() {
	if command := strings.TrimSpace(r.cfg.Command); command != "" {
		if _, err := os.Stat(command); err != nil {
			r.unavailable = fmt.Sprintf("PlantUML renderer unavailable: configured command %q is not accessible", command)
			r.logger.Warn("plantuml command unavailable", "command", command, "error", err)
			return
		}
		r.backend = backendCommand
		r.command = command
		r.logger.Info("plantuml renderer ready", "backend", "command", "command", command)
		return
	}

	if serverURL := strings.TrimSpace(r.cfg.ServerURL); serverURL != "" {
		parsed, err := url.Parse(serverURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			r.unavailable = fmt.Sprintf("PlantUML renderer unavailable: plantuml.serverUrl %q must be an http(s) URL", serverURL)
			r.logger.Warn("plantuml server url invalid", "serverUrl", serverURL)
			return
		}
		r.backend = backendServer
		r.serverURL = strings.TrimRight(serverURL, "/")
		r.logger.Info("plantuml renderer ready", "backend", "server", "serverUrl", r.serverURL)
		return
	}

	if envCommand := strings.TrimSpace(os.Getenv(commandEnvVar)); envCommand != "" {
		if _, err := os.Stat(envCommand); err != nil {
			r.unavailable = fmt.Sprintf("PlantUML renderer unavailable: %s=%q is not accessible", commandEnvVar, envCommand)
			r.logger.Warn("plantuml command from env unavailable", "command", envCommand, "error", err)
			return
		}
		r.backend = backendCommand
		r.command = envCommand
		r.logger.Info("plantuml renderer ready", "backend", "command", "command", envCommand, "source", commandEnvVar)
		return
	}

	path, err := exec.LookPath("plantuml")
	if err != nil {
		r.unavailable = unavailableMessage
		r.logger.Info("plantuml renderer unavailable", "reason", "plantuml not found in PATH")
		return
	}
	r.backend = backendCommand
	r.command = path
	r.logger.Info("plantuml renderer ready", "backend", "command", "command", path, "source", "PATH")
}

// Render converts one diagram to SVG. Results (including failures for the
// same source) are cached process-wide.
func (r *Renderer) Render(ctx context.Context, source string, dark bool) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", errors.New("empty PlantUML source")
	}
	if len(source) > maxSourceBytes {
		return "", fmt.Errorf("PlantUML source exceeds %d bytes", maxSourceBytes)
	}
	if r.backend == backendUnavailable {
		return "", errors.New(r.unavailable)
	}

	prepared := source
	if dark {
		prepared = applyDarkSkinparams(source)
	}

	key := r.cacheKey(prepared)
	if cached, ok := r.cacheGet(key); ok {
		return cached, nil
	}

	result, err, _ := r.group.Do(key, func() (any, error) {
		if cached, ok := r.cacheGet(key); ok {
			return cached, nil
		}
		renderCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout())
		defer cancel()

		var svg string
		var renderErr error
		switch r.backend {
		case backendCommand:
			svg, renderErr = r.renderCommand(renderCtx, prepared)
		case backendServer:
			svg, renderErr = r.renderServer(renderCtx, prepared)
		default:
			renderErr = errors.New(unavailableMessage)
		}
		if renderErr != nil {
			r.logger.Warn("plantuml render failed", "backend", r.backendName(), "error", renderErr, "sourceBytes", len(source))
			return "", renderErr
		}
		r.cacheSet(key, svg)
		return svg, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (r *Renderer) backendName() string {
	switch r.backend {
	case backendCommand:
		return "command"
	case backendServer:
		return "server"
	default:
		return "unavailable"
	}
}

func (r *Renderer) cacheKey(preparedSource string) string {
	sum := sha256.Sum256([]byte(r.backendName() + "\x00" + r.command + r.serverURL + "\x00" + preparedSource))
	return hex.EncodeToString(sum[:])
}

func (r *Renderer) renderCommand(ctx context.Context, source string) (string, error) {
	cmd := exec.CommandContext(ctx, r.command, "-tsvg", "-pipe")
	cmd.Stdin = strings.NewReader(source)
	stdout := &cappedBuffer{limit: maxSVGBytes}
	stderr := &cappedBuffer{limit: maxStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("plantuml render timed out after %s", r.cfg.Timeout())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("plantuml failed: %s", detail)
	}
	if stdout.overflow {
		return "", fmt.Errorf("plantuml output exceeds %d bytes", maxSVGBytes)
	}
	svg := strings.TrimSpace(stdout.String())
	if !strings.Contains(svg, "<svg") {
		return "", errors.New("plantuml produced no SVG output")
	}
	return svg, nil
}

func (r *Renderer) renderServer(ctx context.Context, source string) (string, error) {
	endpoint := r.serverURL + "/svg"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("plantuml request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Accept", "image/svg+xml")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("plantuml server timed out after %s", r.cfg.Timeout())
		}
		return "", fmt.Errorf("plantuml server request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSVGBytes+1))
	if err != nil {
		return "", fmt.Errorf("plantuml server response: %w", err)
	}
	if len(body) > maxSVGBytes {
		return "", fmt.Errorf("plantuml output exceeds %d bytes", maxSVGBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(body))
		if len(detail) > 500 {
			detail = detail[:500]
		}
		return "", fmt.Errorf("plantuml server returned %s: %s", resp.Status, detail)
	}
	svg := strings.TrimSpace(string(body))
	if !strings.Contains(svg, "<svg") {
		return "", errors.New("plantuml server produced no SVG output")
	}
	return svg, nil
}

func (r *Renderer) cacheGet(key string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	element, ok := r.entries[key]
	if !ok {
		return "", false
	}
	r.lru.MoveToFront(element)
	return element.Value.(*cacheEntry).svg, true
}

func (r *Renderer) cacheSet(key, svg string) {
	size := len(svg)
	if size > maxCacheBytes {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if element, ok := r.entries[key]; ok {
		entry := element.Value.(*cacheEntry)
		r.cacheBytes += size - len(entry.svg)
		entry.svg = svg
		r.lru.MoveToFront(element)
	} else {
		element := r.lru.PushFront(&cacheEntry{key: key, svg: svg})
		r.entries[key] = element
		r.cacheBytes += size
	}
	for r.lru.Len() > maxCacheEntries || r.cacheBytes > maxCacheBytes {
		oldest := r.lru.Back()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*cacheEntry)
		r.lru.Remove(oldest)
		delete(r.entries, entry.key)
		r.cacheBytes -= len(entry.svg)
	}
}

// cappedBuffer stores at most limit bytes and drops the excess instead of
// failing the write, so a runaway process keeps running until its context
// deadline and we can report an overflow afterwards.
type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return written, nil
	}
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.buf.Write(p)
	return written, nil
}

func (b *cappedBuffer) String() string { return b.buf.String() }
