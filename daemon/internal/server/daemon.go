package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/metrics"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/relayclient"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// Daemon is the core server managing WebSocket connections and agent lifecycle.
type Daemon struct {
	cfg            *config.Config
	logger         *slog.Logger
	http           *http.Server
	ws             *WSServer
	agentMgr       *agent.AgentManager
	agentStorage   *agent.AgentStorage
	registry       *agent.ProviderRegistry
	workspaceStore *WorkspaceStore
	terminalMgr    *terminal.TerminalManager
	projectReg     *workspace.ProjectRegistry
	workspaceReg   *workspace.WorkspaceRegistry
	gitSvc         workspace.WorkspaceGitService
	scriptMgr      *workspace.ScriptManager
	scriptProxy    *workspace.ScriptProxy
	relayClient    *relayclient.Client
	ln             net.Listener
}

// NewDaemon creates and wires up all daemon services.
func NewDaemon(cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	// Initialize agent storage
	agentsDir := filepath.Join(cfg.SoloHome, "agents")
	agentStorage := agent.NewAgentStorage(agentsDir, logger)
	if err := agentStorage.Initialize(); err != nil {
		return nil, fmt.Errorf("agent storage init: %w", err)
	}

	// Initialize provider registry
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewClaudeAgentClient("", logger))
	registry.Register(agent.NewOpenCodeAgentClient("", logger))
	registry.Register(agent.NewKimiAgentClient("", logger))
	if os.Getenv("SOLO_ENABLE_MOCK_PROVIDER") == "1" {
		registry.Register(agent.NewMockAgentClient())
	}

	// Apply custom models from config
	if len(cfg.CustomModels) > 0 {
		customModels := make(map[string][]protocol.AgentModelDefinition)
		for providerID, models := range cfg.CustomModels {
			for _, m := range models {
				def := protocol.AgentModelDefinition{
					Provider:    providerID,
					ID:          m.ID,
					Label:       m.Label,
					Description: m.Description,
				}
				if def.Label == "" {
					def.Label = m.ID
				}
				if m.IsDefault != nil {
					def.IsDefault = *m.IsDefault
				}
				if m.DefaultThinkingOptionID != nil {
					def.DefaultThinkingOptionID = *m.DefaultThinkingOptionID
				}
				for _, opt := range m.ThinkingOptions {
					so := protocol.AgentSelectOption{ID: opt.ID}
					if opt.Label != "" {
						so.Label = opt.Label
					} else {
						so.Label = opt.ID
					}
					if opt.IsDefault != nil {
						so.IsDefault = *opt.IsDefault
					}
					def.ThinkingOptions = append(def.ThinkingOptions, so)
				}
				customModels[providerID] = append(customModels[providerID], def)
			}
		}
		registry.SetCustomModels(customModels)
	}

	// Initialize agent manager
	agentMgr := agent.NewAgentManager(agentStorage, registry, logger)
	if err := agentMgr.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("agent manager init: %w", err)
	}

	// Initialize timeline store
	timelineStore := agent.NewInMemoryTimelineStore()

	// Initialize workspace store
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	if err := workspaceStore.Initialize(); err != nil {
		return nil, fmt.Errorf("workspace store init: %w", err)
	}

	// Initialize terminal manager
	terminalMgr := terminal.NewTerminalManager(logger)

	// Initialize workspace registries
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	if err := projectReg.Initialize(); err != nil {
		return nil, fmt.Errorf("project registry init: %w", err)
	}
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	if err := workspaceReg.Initialize(); err != nil {
		return nil, fmt.Errorf("workspace registry init: %w", err)
	}
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)

	// Initialize push infrastructure
	pushTokenStore := push.NewPersistedTokenStore(filepath.Join(cfg.SoloHome, "push-tokens.json"), logger)
	pusher := push.NewExpoPushService("", pushTokenStore, logger)
	activityTracker := NewClientActivityTracker()

	// Create WS server with dependencies
	ws := NewWSServerWithConfig(DaemonConfig{
		Config:          cfg,
		Logger:          logger,
		AgentMgr:        agentMgr,
		TimelineStore:   timelineStore,
		Registry:        registry,
		WorkspaceStore:  workspaceStore,
		TerminalMgr:     terminalMgr,
		ProjectReg:      projectReg,
		WorkspaceReg:    workspaceReg,
		GitSvc:          gitSvc,
		ScriptMgr:       scriptMgr,
		ScriptProxy:     scriptProxy,
		PushTokenStore:  pushTokenStore,
		Pusher:          pusher,
		ActivityTracker: activityTracker,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.HandleWebSocket)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/status", handleStatus(cfg))
	mux.Handle("/metrics", promhttp.Handler())
	// Script proxy routes (matched by hostname)
	mux.Handle("/", scriptProxy)

	d := &Daemon{
		cfg:            cfg,
		logger:         logger,
		ws:             ws,
		agentMgr:       agentMgr,
		agentStorage:   agentStorage,
		registry:       registry,
		workspaceStore: workspaceStore,
		terminalMgr:    terminalMgr,
		projectReg:     projectReg,
		workspaceReg:   workspaceReg,
		gitSvc:         gitSvc,
		scriptMgr:      scriptMgr,
		scriptProxy:    scriptProxy,
		http: &http.Server{
			Handler: mux,
		},
	}

	if cfg.RelayEnabled && cfg.RelayEndpoint != "" {
		keyPair, err := relayclient.LoadDaemonKeyPair(cfg.SoloHome)
		if err != nil {
			logger.Warn("daemon keypair not found, E2EE disabled", "error", err)
		}
		d.relayClient = relayclient.NewClient(cfg.ServerID, cfg.RelayEndpoint, ws, logger, keyPair, cfg.RelayDisableControlKeepalive)
	}

	return d, nil
}

// Start begins listening for connections.
func (d *Daemon) Start() error {
	target, err := d.cfg.ResolveListenTarget()
	if err != nil {
		return fmt.Errorf("cannot resolve listen target: %w", err)
	}

	switch target.Type {
	case "tcp":
		d.ln, err = net.Listen("tcp", fmt.Sprintf("%s:%d", target.Host, target.Port))
	case "socket":
		d.ln, err = net.Listen("unix", target.Path)
	case "pipe":
		d.ln, err = net.Listen("unix", target.Path)
	default:
		return fmt.Errorf("unknown listen type: %s", target.Type)
	}
	if err != nil {
		return fmt.Errorf("cannot listen: %w", err)
	}

	go func() {
		if err := d.http.Serve(d.ln); err != nil && err != http.ErrServerClosed {
			d.logger.Error("HTTP server error", "error", err)
		}
	}()

	d.logger.Info("daemon started",
		"serverId", d.cfg.ServerID,
		"listen", d.ln.Addr().String(),
	)

	if d.relayClient != nil {
		if err := d.relayClient.Start(); err != nil {
			d.logger.Warn("failed to start relay client", "error", err)
		}
	}

	// Start background git metadata refresh so handlers always read from cache.
	d.gitSvc.StartBackgroundRefresh(context.Background(), 15*time.Second)

	// Pre-warm git cache for all persisted workspaces so that the first client
	// connection sees valid GetMetadataCached results rather than nil.
	// This runs in the background; by the time a human opens the iOS App the
	// cache will usually be populated.
	go d.prewarmGitCache()

	// Pre-warm OpenCode server in the background so first list_commands doesn't
	// block the session handler waiting for the server to cold-start.
	go d.prewarmOpenCodeServer()

	return nil
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop(ctx context.Context) error {
	if d.relayClient != nil {
		d.relayClient.Stop()
	}
	// Stop background git refresh before shutting down other services.
	d.gitSvc.StopBackgroundRefresh()
	d.terminalMgr.KillAll()
	d.ws.Close()
	// Shutdown OpenCode server manager (terminates background server processes)
	agent.ShutdownOpenCodeServerManager()
	if err := d.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP shutdown error: %w", err)
	}
	return nil
}

func (d *Daemon) prewarmGitCache() {
	if d.workspaceStore == nil {
		return
	}
	workspaces := d.workspaceStore.GetAll()
	if len(workspaces) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, ws := range workspaces {
		if ws == nil || ws.WorkspaceDirectory == "" {
			continue
		}
		wg.Add(1)
		dir := ws.WorkspaceDirectory
		go func() {
			defer wg.Done()
			_, _ = d.gitSvc.GetMetadata(dir)
		}()
	}
	wg.Wait()
	d.logger.Info("git cache prewarmed", "count", len(workspaces))
}

func (d *Daemon) prewarmOpenCodeServer() {
	client, err := d.registry.Get("opencode")
	if err != nil {
		return
	}
	opencodeClient, ok := client.(*agent.OpenCodeAgentClient)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := opencodeClient.EnsureRunning(ctx); err != nil {
		d.logger.Warn("opencode server prewarm failed", "error", err)
	} else {
		d.logger.Info("opencode server prewarmed")
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func handleStatus(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"serverId": cfg.ServerID,
			"version":  cfg.Version,
			"listen":   cfg.Listen,
		})
	}
}

// --- WebSocket Server ---

// WSServer manages WebSocket connections.
type WSServer struct {
	cfg             *config.Config
	logger          *slog.Logger
	sessions        map[string]*Session
	agentMgr        *agent.AgentManager
	timelineStore   *agent.InMemoryTimelineStore
	registry        *agent.ProviderRegistry
	workspaceStore  *WorkspaceStore
	terminalMgr     *terminal.TerminalManager
	projectReg      *workspace.ProjectRegistry
	workspaceReg    *workspace.WorkspaceRegistry
	gitSvc          workspace.WorkspaceGitService
	scriptMgr       *workspace.ScriptManager
	scriptProxy     *workspace.ScriptProxy
	pushTokenStore  push.TokenStore
	pusher          push.Pusher
	activityTracker ActivityTracker
	done            chan struct{}
	mu              sync.RWMutex
	gracePeriod     time.Duration // override for SessionDisconnectGraceMs; 0 = use default

	onHelloMu       sync.Mutex
	onHelloCallback func()
}

// NewWSServer creates a new WebSocket server with agent dependencies.
// Deprecated: use NewWSServerWithConfig
func NewWSServer(cfg *config.Config, logger *slog.Logger, agentMgr *agent.AgentManager, timelineStore *agent.InMemoryTimelineStore, registry *agent.ProviderRegistry, workspaceStore *WorkspaceStore, terminalMgr *terminal.TerminalManager, projectReg *workspace.ProjectRegistry, workspaceReg *workspace.WorkspaceRegistry, gitSvc workspace.WorkspaceGitService, scriptMgr *workspace.ScriptManager, scriptProxy *workspace.ScriptProxy, pushTokenStore push.TokenStore, pusher push.Pusher, activityTracker ActivityTracker) *WSServer {
	return &WSServer{
		cfg:             cfg,
		logger:          logger,
		sessions:        make(map[string]*Session),
		agentMgr:        agentMgr,
		timelineStore:   timelineStore,
		registry:        registry,
		workspaceStore:  workspaceStore,
		terminalMgr:     terminalMgr,
		projectReg:      projectReg,
		workspaceReg:    workspaceReg,
		gitSvc:          gitSvc,
		scriptMgr:       scriptMgr,
		scriptProxy:     scriptProxy,
		pushTokenStore:  pushTokenStore,
		pusher:          pusher,
		activityTracker: activityTracker,
		done:            make(chan struct{}),
	}
}

// DaemonConfig aggregates all dependencies needed by NewWSServer.
// This reduces the parameter count from 15 to 1.
type DaemonConfig struct {
	Config          *config.Config
	Logger          *slog.Logger
	AgentMgr        *agent.AgentManager
	TimelineStore   *agent.InMemoryTimelineStore
	Registry        *agent.ProviderRegistry
	WorkspaceStore  *WorkspaceStore
	TerminalMgr     *terminal.TerminalManager
	ProjectReg      *workspace.ProjectRegistry
	WorkspaceReg    *workspace.WorkspaceRegistry
	GitSvc          workspace.WorkspaceGitService
	ScriptMgr       *workspace.ScriptManager
	ScriptProxy     *workspace.ScriptProxy
	PushTokenStore  push.TokenStore
	Pusher          push.Pusher
	ActivityTracker ActivityTracker
}

// NewWSServerWithConfig creates a new WebSocket server using a DaemonConfig.
func NewWSServerWithConfig(cfg DaemonConfig) *WSServer {
	return NewWSServer(
		cfg.Config,
		cfg.Logger,
		cfg.AgentMgr,
		cfg.TimelineStore,
		cfg.Registry,
		cfg.WorkspaceStore,
		cfg.TerminalMgr,
		cfg.ProjectReg,
		cfg.WorkspaceReg,
		cfg.GitSvc,
		cfg.ScriptMgr,
		cfg.ScriptProxy,
		cfg.PushTokenStore,
		cfg.Pusher,
		cfg.ActivityTracker,
	)
}

func (s *WSServer) broadcast(msg protocol.WSOutboundMessage) {
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.RUnlock()

	for _, sess := range sessions {
		sess.sendMessage(msg)
	}
}

// HandleWebSocket handles the /ws endpoint.
func (s *WSServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: s.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	s.handleNewConnection(conn)
}

func (s *WSServer) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if len(s.cfg.CORSOrigins) == 0 {
		return true
	}
	for _, allowed := range s.cfg.CORSOrigins {
		if origin == allowed {
			return true
		}
	}
	s.logger.Warn("rejected WebSocket connection: origin not allowed", "origin", origin)
	return false
}

// AttachExternalConnection feeds an externally-created WebSocket connection
// (e.g. from a relay data socket) into the same session lifecycle as local
// inbound connections.
func (s *WSServer) AttachExternalConnection(conn WSConn) {
	s.handleNewConnection(conn)
}

// OnHelloProcessed registers a callback that fires when a hello handshake
// completes successfully (right after server_info is sent). This is used by
// the relay client to cancel the openTimer guard, mirroring Solo's
// clearTimeout(openTimeout) on socket "open".
func (s *WSServer) OnHelloProcessed(fn func()) {
	s.onHelloMu.Lock()
	s.onHelloCallback = fn
	s.onHelloMu.Unlock()
}

// fireHelloProcessed invokes the registered hello-processed callback.
func (s *WSServer) fireHelloProcessed() {
	s.onHelloMu.Lock()
	fn := s.onHelloCallback
	s.onHelloCallback = nil
	s.onHelloMu.Unlock()
	if fn != nil {
		fn()
	}
}

// Close shuts down all sessions, including any in the grace period.
func (s *WSServer) Close() {
	close(s.done)
	s.mu.Lock()
	for _, sess := range s.sessions {
		if sess.IsInGrace() {
			sess.expireGrace()
		}
	}
	s.mu.Unlock()
}

// handleNewConnection manages a single WebSocket connection lifecycle.
func (s *WSServer) handleNewConnection(conn WSConn) {
	helloTimer := time.NewTimer(protocol.HelloTimeoutMs * time.Millisecond)
	defer helloTimer.Stop()

	helloCh := make(chan *protocol.WSInboundMessage, 1)
	errCh := make(chan error, 1)

	go func() {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		var wsMsg protocol.WSInboundMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			errCh <- fmt.Errorf("invalid JSON: %w", err)
			return
		}
		helloCh <- &wsMsg
	}()

	var hello *protocol.WSInboundMessage
	select {
	case hello = <-helloCh:
		s.logger.Info("hello received", "clientId", hello.ClientID, "clientType", hello.ClientType, "protocolVersion", hello.ProtocolVersion)
	case err := <-errCh:
		s.logger.Info("connection closed before hello", "error", err)
		conn.Close()
		return
	case <-helloTimer.C:
		s.logger.Info("hello timeout, closing connection")
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(protocol.WSCloseHelloTimeout, "hello timeout"))
		conn.Close()
		return
	case <-s.done:
		s.logger.Info("server shutting down, closing connection")
		conn.Close()
		return
	}

	if hello.Type != "hello" {
		s.logger.Info("invalid hello type", "type", hello.Type)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(protocol.WSCloseInvalidHello, "expected hello message"))
		conn.Close()
		return
	}
	if hello.ProtocolVersion != protocol.WSProtocolVersion {
		s.logger.Info("incompatible protocol version", "version", hello.ProtocolVersion, "expected", protocol.WSProtocolVersion)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(protocol.WSCloseIncompatibleProtocol, "incompatible protocol version"))
		conn.Close()
		return
	}

	clientID := hello.ClientID
	clientType := hello.ClientType

	serverInfo := protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.ServerInfoPayload{
			Status:   "server_info",
			ServerID: s.cfg.ServerID,
			Capabilities: &protocol.ServerCapabilities{
				Voice: &protocol.VoiceCapabilities{
					Dictation: protocol.VoiceFeatureStatus{Enabled: false, Reason: "not_implemented"},
					Voice:     protocol.VoiceFeatureStatus{Enabled: false, Reason: "not_implemented"},
				},
			},
			Features: &protocol.ServerFeatures{
				ProvidersSnapshot: ptrBool(true),
			},
		},
	})

	data, err := json.Marshal(serverInfo)
	if err != nil {
		s.logger.Error("cannot marshal server_info", "error", err)
		conn.Close()
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.logger.Error("cannot send server_info", "error", err)
		conn.Close()
		return
	}
	s.logger.Info("server_info sent", "clientId", clientID, "bytes", len(data), "json", string(data))

	// Signal that the hello handshake completed successfully.
	// This allows the relay client to cancel the openTimer guard,
	// preventing it from killing healthy, long-running sessions.
	s.fireHelloProcessed()

	// Check for an existing session in grace period for this clientID.
	// If found, attach to it instead of creating a new session.
	// AttachSocket cancels the grace timer and resumes the session.
	s.mu.RLock()
	existingSess, exists := s.sessions[clientID]
	s.mu.RUnlock()

	if exists && existingSess.IsInGrace() {
		s.logger.Info("resuming session in grace period via AttachSocket", "clientId", clientID)
		// AttachSocket blocks until the new socket disconnects.
		// It cancels the grace timer, pushes current state, and enters grace again on disconnect.
		existingSess.AttachSocket(conn)
		// After AttachSocket returns the client disconnected again.
		// If the session is still in grace, keep it in the map for another
		// potential reconnect; otherwise remove it.
		s.mu.Lock()
		if s.sessions[clientID] == existingSess && !existingSess.IsInGrace() {
			delete(s.sessions, clientID)
			metrics.SessionsActive.Dec()
		}
		s.mu.Unlock()
		s.logger.Info("client disconnected after session resume", "clientId", clientID)
		return
	}
	if exists && existingSess.IsAttaching() {
		// A concurrent AttachSocket is in progress on this session. If the
		// read-loop is stuck on a stale socket (e.g. iOS app reconnected via
		// relay with a new connectionId while the old data socket appears
		// alive), force-close the attaching socket and replace the session
		// so the client can reconnect. shutdownForReplacement closes the
		// attaching conn, which unblocks readLoopFor and lets the old
		// AttachSocket goroutine exit.
		s.logger.Warn("session is stuck attaching, replacing with new session", "clientId", clientID)
		existingSess.shutdownForReplacement()
		// Fall through to create a fresh session below.
	}
	if exists {
		s.logger.Warn("replacing existing non-grace session for client", "clientId", clientID)
		existingSess.shutdownForReplacement()
	}

	sess := NewSessionWithConfig(clientID, string(clientType), conn, SessionConfig{
		Config:         s.cfg,
		Logger:         s.logger,
		AgentMgr:       s.agentMgr,
		TimelineStore:  s.timelineStore,
		Registry:       s.registry,
		WorkspaceStore: s.workspaceStore,
		TerminalMgr:    s.terminalMgr,
		ProjectReg:     s.projectReg,
		WorkspaceReg:   s.workspaceReg,
		GitSvc:         s.gitSvc,
		ScriptMgr:      s.scriptMgr,
		ScriptProxy:    s.scriptProxy,
		Broadcast:      s.broadcast,
	})
	if _, isRelay := conn.(*relayclient.E2EEConn); isRelay {
		sess.SetIsRelay(true)
	}
	if s.gracePeriod > 0 {
		sess.gracePeriod = s.gracePeriod
	}
	sess.SetPushTokenStore(s.pushTokenStore)
	sess.SetPusher(s.pusher)
	sess.SetActivityTracker(s.activityTracker)

	// Set callback so the session can remove itself from the map when grace expires.
	sess.onGraceExpire = func() {
		s.mu.Lock()
		if s.sessions[clientID] == sess {
			delete(s.sessions, clientID)
			metrics.SessionsActive.Dec()
		}
		s.mu.Unlock()
	}

	s.logger.Info("client connected", "clientId", clientID, "clientType", clientType)
	metrics.ConnectionsTotal.Inc()

	s.mu.Lock()
	s.sessions[clientID] = sess
	s.mu.Unlock()
	metrics.SessionsActive.Inc()

	sess.Run()

	// After Run() returns the session may be in grace (waiting for reconnect)
	// or fully destroyed. Only remove from map if not in grace.
	s.mu.Lock()
	if s.sessions[clientID] == sess && !sess.IsInGrace() {
		delete(s.sessions, clientID)
		metrics.SessionsActive.Dec()
	}
	s.mu.Unlock()

	s.logger.Info("client disconnected", "clientId", clientID)
}

func ptrBool(b bool) *bool { return &b }
