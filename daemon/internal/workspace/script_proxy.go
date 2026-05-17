package workspace

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// ScriptProxy routes HTTP requests to running service scripts based on hostname.
type ScriptProxy struct {
	logger    *slog.Logger
	scriptMgr *ScriptManager
	mu        sync.RWMutex
	proxies   map[string]*httputil.ReverseProxy // key: hostname
}

// NewScriptProxy creates a new ScriptProxy.
func NewScriptProxy(logger *slog.Logger, scriptMgr *ScriptManager) *ScriptProxy {
	return &ScriptProxy{
		logger:    logger,
		scriptMgr: scriptMgr,
		proxies:   make(map[string]*httputil.ReverseProxy),
	}
}

// RegisterRoute registers a reverse proxy route for a hostname.
func (p *ScriptProxy) RegisterRoute(hostname string, port int) {
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(target)

	p.mu.Lock()
	p.proxies[hostname] = proxy
	p.mu.Unlock()

	p.logger.Info("registered script proxy route", "hostname", hostname, "port", port)
}

// UnregisterRoute removes a reverse proxy route.
func (p *ScriptProxy) UnregisterRoute(hostname string) {
	p.mu.Lock()
	delete(p.proxies, hostname)
	p.mu.Unlock()

	p.logger.Info("unregistered script proxy route", "hostname", hostname)
}

// ServeHTTP implements http.Handler. It routes requests based on the Host header.
func (p *ScriptProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// Strip port from host if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	p.mu.RLock()
	proxy, ok := p.proxies[host]
	p.mu.RUnlock()

	if !ok {
		http.Error(w, "no route for host", http.StatusBadGateway)
		return
	}

	proxy.ServeHTTP(w, r)
}
