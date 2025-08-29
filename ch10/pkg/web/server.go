package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/agent"
	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/types"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// WebSocket connection wrapper
type wsConnection struct {
	conn     *websocket.Conn
	lastPong time.Time
}

// WebServer handles HTTP requests for the AI agent UI
type WebServer struct {
	router    *mux.Router
	templates *template.Template
	aiAgent   *agent.StateAwareAgent
	upgrader  websocket.Upgrader

	// WebSocket connection management
	connections map[string]*wsConnection
	connMutex   sync.RWMutex

	// Decision storage (in-memory for now)
	decisions      map[string]*StoredDecision
	decisionsMutex sync.RWMutex
}

// StoredDecision stores a decision along with its execution parameters
type StoredDecision struct {
	Decision *types.AgentDecision `json:"decision"`
	DryRun   bool                 `json:"dry_run"`
}

// NewWebServer creates a new web server instance
func NewWebServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *WebServer {
	ws := &WebServer{
		router:      mux.NewRouter(),
		connections: make(map[string]*wsConnection),
		decisions:   make(map[string]*StoredDecision),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	// Initialize AI agent with all infrastructure components
	ws.initializeAIAgent(cfg, awsClient, logger)

	ws.setupRoutes()
	ws.loadTemplates()

	return ws
}

// initializeAIAgent initializes the AI agent if possible
func (ws *WebServer) initializeAIAgent(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) {
	// Check if the required API key is available based on provider
	provider := strings.ToLower(cfg.Agent.Provider)
	var hasAPIKey bool

	switch provider {
	case "openai":
		hasAPIKey = cfg.Agent.OpenAIAPIKey != ""
	case "gemini", "googleai":
		hasAPIKey = cfg.Agent.GeminiAPIKey != ""
	case "anthropic":
		hasAPIKey = cfg.Agent.AnthropicAPIKey != ""
	default:
		logger.WithField("provider", provider).Warn("Unknown AI provider - AI agent will run in demo mode")
		return
	}

	if !hasAPIKey {
		logger.WithField("provider", provider).Warn("API key not set for provider - AI agent will run in demo mode")
		return
	}

	// Create AI agent using centralized config
	aiAgent, err := agent.NewStateAwareAgent(
		&cfg.Agent,
		awsClient,
		cfg.GetStateFilePath(),
		cfg.AWS.Region,
		logger,
		&cfg.AWS,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to create AI agent - running in demo mode")
		return
	}

	// Initialize the agent
	if err := aiAgent.Initialize(context.Background()); err != nil {
		logger.WithError(err).Error("Failed to initialize AI agent - running in demo mode")
		return
	}

	ws.aiAgent = aiAgent
	logger.Info("AI agent initialized successfully")
}

// setupRoutes configures HTTP routes
func (ws *WebServer) setupRoutes() {
	// Static files
	ws.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// UI routes
	ws.router.HandleFunc("/", ws.indexHandler).Methods("GET")
	ws.router.HandleFunc("/dashboard", ws.dashboardHandler).Methods("GET")

	// API routes
	api := ws.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/state", ws.getStateHandler).Methods("GET")
	api.HandleFunc("/discover", ws.discoverInfrastructureHandler).Methods("POST")
	api.HandleFunc("/graph", ws.getGraphHandler).Methods("GET")
	api.HandleFunc("/conflicts", ws.getConflictsHandler).Methods("GET")
	api.HandleFunc("/plan", ws.getPlanHandler).Methods("POST")
	api.HandleFunc("/agent/process", ws.processRequestHandler).Methods("POST")
	api.HandleFunc("/agent/execute", ws.executeConfirmedPlanHandler).Methods("POST")
	api.HandleFunc("/export", ws.exportStateHandler).Methods("GET")

	// WebSocket for real-time updates
	ws.router.HandleFunc("/ws", ws.websocketHandler)
}

// loadTemplates loads HTML templates
func (ws *WebServer) loadTemplates() {
	templatePath := filepath.Join("web", "templates", "*.html")
	templates, err := template.ParseGlob(templatePath)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to load templates")
		return
	}
	ws.templates = templates
}

// Start starts the web server
func (ws *WebServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	ws.aiAgent.Logger.WithField("port", port).Info("Starting web server")

	return http.ListenAndServe(addr, ws.router)
}

// Handlers

func (ws *WebServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (ws *WebServer) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if ws.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title": "AI Infrastructure Agent Dashboard",
		"Time":  time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := ws.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to execute template")
		http.Error(w, "Template execution failed", http.StatusInternalServerError)
		return
	}
}

// API Handlers

func (ws *WebServer) getStateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if AI agent is available
	if ws.aiAgent == nil {
		// Return empty state if AI agent is not available
		emptyState := `{"resources": [], "metadata": {"timestamp": "` + time.Now().Format(time.RFC3339) + `", "source": "demo_mode", "message": "AI agent not available"}}`
		w.Write([]byte(emptyState))
		return
	}

	// Use MCP server to get state
	stateJSON, err := ws.aiAgent.ExportInfrastructureState(r.Context(), false)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to get state from MCP server")
		http.Error(w, "Failed to get state", http.StatusInternalServerError)
		return
	}

	// Write the JSON response directly
	w.Write([]byte(stateJSON))
}

func (ws *WebServer) discoverInfrastructureHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()
	// Use MCP server to analyze infrastructure state
	_, discoveredResources, _, err := ws.aiAgent.AnalyzeInfrastructureState(ctx, true)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to discover infrastructure")
		http.Error(w, "Discovery failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"resources": discoveredResources,
		"count":     len(discoveredResources),
		"timestamp": time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode discovery response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) getGraphHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "text"
	}

	ctx := r.Context()
	// Use MCP server to visualize dependency graph
	visualization, bottlenecks, err := ws.aiAgent.VisualizeDependencyGraph(ctx, format, true)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to visualize dependency graph")
		http.Error(w, "Graph visualization failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"visualization": visualization,
		"format":        format,
		"bottlenecks":   bottlenecks,
		"timestamp":     time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode graph response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) getConflictsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	autoResolve := r.URL.Query().Get("auto_resolve") == "true"

	ctx := r.Context()
	// Use MCP server to detect conflicts
	conflicts, err := ws.aiAgent.DetectInfrastructureConflicts(ctx, autoResolve)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to detect conflicts")
		http.Error(w, "Conflict detection failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"conflicts":    conflicts,
		"count":        len(conflicts),
		"auto_resolve": autoResolve,
		"timestamp":    time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode conflicts response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) getPlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	includeLevels := true
	if val, ok := requestBody["include_levels"].(bool); ok {
		includeLevels = val
	}

	ctx := r.Context()
	// Use MCP server to plan deployment
	deploymentOrder, deploymentLevels, err := ws.aiAgent.PlanInfrastructureDeployment(ctx, nil, includeLevels)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to plan deployment")
		http.Error(w, "Deployment planning failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"deployment_order": deploymentOrder,
		"resource_count":   len(deploymentOrder),
		"timestamp":        time.Now(),
	}

	if includeLevels {
		response["deployment_levels"] = deploymentLevels
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode plan response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) processRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	request, ok := requestBody["request"].(string)
	if !ok {
		http.Error(w, "Request field is required", http.StatusBadRequest)
		return
	}

	dryRun := true
	if val, ok := requestBody["dry_run"].(bool); ok {
		dryRun = val
	}

	ctx := r.Context()

	// Check if AI agent is available
	if ws.aiAgent == nil {
		// Return demo response if AI agent is not available
		response := map[string]interface{}{
			"request":    request,
			"dry_run":    dryRun,
			"response":   "AI Agent not available. Please set OPENAI_API_KEY environment variable to enable real AI processing.",
			"confidence": 0.0,
			"mode":       "demo",
			"timestamp":  time.Now(),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to encode demo response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Use real AI agent
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"request": request,
		"dry_run": dryRun,
	}).Info("Processing request with AI agent")

	// Notify WebSocket clients that processing has started
	ws.broadcastUpdate(map[string]interface{}{
		"type":      "processing_started",
		"request":   request,
		"dry_run":   dryRun,
		"timestamp": time.Now(),
	})

	// Process the request
	decision, err := ws.aiAgent.ProcessRequest(ctx, request)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("AI agent request processing failed")
		http.Error(w, fmt.Sprintf("AI processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Store the decision for later execution
	ws.storeDecisionWithDryRun(decision, dryRun)

	// Build response with execution plan (without executing yet)
	response := map[string]interface{}{
		"request":              request,
		"dry_run":              dryRun,
		"mode":                 "live",
		"decision":             decision,
		"executionPlan":        decision.ExecutionPlan,
		"confidence":           decision.Confidence,
		"action":               decision.Action,
		"reasoning":            decision.Reasoning,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode AI response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Notify WebSocket clients that processing has completed
	ws.broadcastUpdate(map[string]interface{}{
		"type":                 "processing_completed",
		"request":              request,
		"decisionId":           decision.ID,
		"success":              true,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	})
}

func (ws *WebServer) executeConfirmedPlanHandler(w http.ResponseWriter, r *http.Request) {
	var executeRequest struct {
		DecisionID string `json:"decisionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to decode execute request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if executeRequest.DecisionID == "" {
		http.Error(w, "Decision ID is required", http.StatusBadRequest)
		return
	}

	ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Info("Executing confirmed plan")

	// Retrieve the stored decision with dry run flag
	decision, dryRun, exists := ws.getStoredDecisionWithDryRun(executeRequest.DecisionID)
	if !exists {
		ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Error("Decision not found")
		http.Error(w, "Decision not found", http.StatusNotFound)
		return
	}

	// Create a buffered progress channel to avoid blocking
	progressChan := make(chan *types.ExecutionUpdate, 100)

	// Start execution in a goroutine
	go func() {
		defer close(progressChan)
		defer ws.removeStoredDecision(executeRequest.DecisionID) // Cleanup after execution

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
		defer cancel()

		ws.aiAgent.Logger.WithFields(map[string]interface{}{
			"decision_id": executeRequest.DecisionID,
			"dry_run":     dryRun,
		}).Debug("Starting agent execution")

		execution, err := ws.aiAgent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, dryRun)
		if err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Plan execution failed")
			// Send error update
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "execution_failed",
				ExecutionID: "failed",
				Message:     fmt.Sprintf("Execution failed: %v", err),
				Error:       err.Error(),
				Timestamp:   time.Now(),
			}:
			default:
			}
		} else {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"execution_id": execution.ID,
				"status":       execution.Status,
			}).Info("Plan execution completed")
		}
	}()

	// Start progress streaming in another goroutine
	go func() {
		for update := range progressChan {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"type":    update.Type,
				"message": update.Message,
			}).Debug("Broadcasting execution update")

			// Broadcast update via WebSocket
			ws.broadcastUpdate(map[string]interface{}{
				"type":        update.Type,
				"executionId": update.ExecutionID,
				"stepId":      update.StepID,
				"message":     update.Message,
				"error":       update.Error,
				"timestamp":   update.Timestamp,
			})
		}
	}()

	// Return immediate response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":     true,
		"message":     "Plan execution started",
		"executionId": "exec-" + executeRequest.DecisionID,
		"timestamp":   time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode execute response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) exportStateHandler(w http.ResponseWriter, r *http.Request) {
	includeDiscovered := r.URL.Query().Get("include_discovered") == "true"

	ctx := r.Context()
	// Use MCP server to export infrastructure state
	stateJSON, err := ws.aiAgent.ExportInfrastructureState(ctx, includeDiscovered)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to export infrastructure state")
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=infrastructure-state.json")

	// Write the JSON response directly
	w.Write([]byte(stateJSON))
}

// WebSocket handler for real-time updates
func (ws *WebServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to upgrade WebSocket")
		return
	}

	// Generate unique connection ID
	connID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

	// Store connection
	ws.connMutex.Lock()
	ws.connections[connID] = &wsConnection{
		conn:     conn,
		lastPong: time.Now(),
	}
	ws.connMutex.Unlock()

	ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection established")

	// Setup connection cleanup
	defer func() {
		ws.connMutex.Lock()
		delete(ws.connections, connID)
		ws.connMutex.Unlock()
		conn.Close()
		ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection closed")
	}()

	// Configure connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Setup ping/pong for connection health
	conn.SetPongHandler(func(string) error {
		ws.connMutex.Lock()
		if wsConn, exists := ws.connections[connID]; exists {
			wsConn.lastPong = time.Now()
		}
		ws.connMutex.Unlock()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send periodic updates and ping
	ticker := time.NewTicker(30 * time.Second)
	pingTicker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	defer pingTicker.Stop()

	// Send initial state
	ws.sendStateUpdate(connID)

	// Handle connection lifecycle
	done := make(chan struct{})

	// Handle incoming messages (if any)
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("WebSocket read error")
				}
				return
			}
		}
	}()

	// Main event loop
	for {
		select {
		case <-ticker.C:
			// Send state update
			ws.sendStateUpdate(connID)

		case <-pingTicker.C:
			// Send ping to check connection health
			ws.connMutex.Lock()
			wsConn, exists := ws.connections[connID]
			ws.connMutex.Unlock()

			if !exists {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("Failed to send ping")
				return
			}

			// Check if we've received a recent pong
			if time.Since(wsConn.lastPong) > 90*time.Second {
				ws.aiAgent.Logger.WithField("conn_id", connID).Warn("Connection seems stale, closing")
				return
			}

		case <-done:
			return

		case <-r.Context().Done():
			return
		}
	}
}

// sendStateUpdate sends a state update to a specific connection
func (ws *WebServer) sendStateUpdate(connID string) {
	ws.connMutex.RLock()
	wsConn, exists := ws.connections[connID]
	ws.connMutex.RUnlock()

	if !exists {
		return
	}

	// Use MCP server to get current state
	stateJSON, err := ws.aiAgent.ExportInfrastructureState(context.Background(), false)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to get state for WebSocket update")
		return
	}

	update := map[string]interface{}{
		"type":      "state_update",
		"data":      stateJSON,
		"timestamp": time.Now(),
	}

	wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := wsConn.conn.WriteJSON(update); err != nil {
		ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to send state update, connection likely closed")
		// Connection is broken, it will be cleaned up by the main handler
	}
}

// broadcastUpdate sends an update to all active WebSocket connections
func (ws *WebServer) broadcastUpdate(update map[string]interface{}) {
	ws.connMutex.RLock()
	connections := make(map[string]*wsConnection)
	for id, conn := range ws.connections {
		connections[id] = conn
	}
	ws.connMutex.RUnlock()

	for connID, wsConn := range connections {
		wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := wsConn.conn.WriteJSON(update); err != nil {
			ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to broadcast update, removing connection")
			// Remove broken connection
			ws.connMutex.Lock()
			delete(ws.connections, connID)
			ws.connMutex.Unlock()
			wsConn.conn.Close()
		}
	}
}

// storeDecisionWithDryRun stores a decision with dry run flag for later execution
func (ws *WebServer) storeDecisionWithDryRun(decision *types.AgentDecision, dryRun bool) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()
	ws.decisions[decision.ID] = &StoredDecision{
		Decision: decision,
		DryRun:   dryRun,
	}
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"dry_run":     dryRun,
	}).Debug("Stored decision for execution")
}

// getStoredDecisionWithDryRun retrieves a stored decision with its dry run flag
func (ws *WebServer) getStoredDecisionWithDryRun(decisionID string) (*types.AgentDecision, bool, bool) {
	ws.decisionsMutex.RLock()
	defer ws.decisionsMutex.RUnlock()
	storedDecision, exists := ws.decisions[decisionID]
	if !exists {
		return nil, false, false
	}
	return storedDecision.Decision, storedDecision.DryRun, true
}

// removeStoredDecision removes a stored decision after execution
func (ws *WebServer) removeStoredDecision(decisionID string) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()
	delete(ws.decisions, decisionID)
	ws.aiAgent.Logger.WithField("decision_id", decisionID).Debug("Removed stored decision")
}
