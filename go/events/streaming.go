// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"right-sizer/logger"

	"github.com/gorilla/websocket"
)

// StreamingAPI provides event streaming capabilities for dashboard integration
type StreamingAPI struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	eventBus    *EventBus
	upgrader    websocket.Upgrader
	config      StreamingConfig
}

// StreamingConfig configures the streaming API
type StreamingConfig struct {
	MaxConnections    int           `json:"maxConnections"`
	ConnectionTimeout time.Duration `json:"connectionTimeout"`
	BufferSize        int           `json:"bufferSize"`
	EnableAuth        bool          `json:"enableAuth"`
	AuthToken         string        `json:"authToken,omitempty"`
	CorsOrigins       []string      `json:"corsOrigins"`
}

// Connection represents a WebSocket connection to dashboard
type Connection struct {
	ID          string
	DashboardID string
	Conn        *websocket.Conn
	Send        chan *Event
	Filter      *EventFilter
	LastPing    time.Time
	Metadata    map[string]string
}

// EventFilter defines filtering criteria for events
type EventFilter struct {
	EventTypes []EventType `json:"eventTypes,omitempty"`
	Types      []EventType `json:"types,omitempty"` // Legacy field, use EventTypes
	Namespaces []string    `json:"namespaces,omitempty"`
	PodNames   []string    `json:"podNames,omitempty"`
	Resources  []string    `json:"resources,omitempty"`
	Severities []Severity  `json:"severities,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Since      *time.Time  `json:"since,omitempty"`
}

// NewStreamingAPI creates a new streaming API instance
func NewStreamingAPI(eventBus *EventBus, config StreamingConfig) *StreamingAPI {
	return &StreamingAPI{
		connections: make(map[string]*Connection),
		eventBus:    eventBus,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if len(config.CorsOrigins) == 0 {
					return true // Allow all origins if none specified
				}
				origin := r.Header.Get("Origin")
				for _, allowed := range config.CorsOrigins {
					if origin == allowed {
						return true
					}
				}
				return false
			},
			WriteBufferSize: 1024,
			ReadBufferSize:  1024,
		},
		config: config,
	}
}

// Start starts the streaming API server
func (s *StreamingAPI) Start(ctx context.Context, port int) error {
	// Subscribe to event bus
	s.eventBus.Subscribe("streaming-api", s.handleEvent)

	// Setup HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/connections", s.handleConnections)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start connection cleanup routine
	go s.cleanupConnections(ctx)

	logger.Info("🌊 Event streaming API started on port %d", port)

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Streaming API server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// handleWebSocket handles WebSocket upgrade and connection management
func (s *StreamingAPI) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate if enabled
	if s.config.EnableAuth {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+s.config.AuthToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Check connection limit
	s.mu.RLock()
	connCount := len(s.connections)
	s.mu.RUnlock()

	if connCount >= s.config.MaxConnections {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection: %v", err)
		return
	}

	// Create connection
	connection := &Connection{
		ID:          generateConnectionID(),
		DashboardID: r.Header.Get("X-Dashboard-ID"),
		Conn:        conn,
		Send:        make(chan *Event, s.config.BufferSize),
		Filter:      &EventFilter{},
		LastPing:    time.Now(),
		Metadata:    make(map[string]string),
	}

	// Parse query parameters for filtering
	s.parseFilterFromRequest(r, connection.Filter)

	// Register connection
	s.mu.Lock()
	s.connections[connection.ID] = connection
	s.mu.Unlock()

	logger.Info("📡 Dashboard connected: %s (total: %d)", connection.DashboardID, len(s.connections))

	// Start connection handlers
	go s.handleConnection(connection)
	go s.writeConnection(connection)
}

// handleConnection handles incoming messages from dashboard
func (s *StreamingAPI) handleConnection(conn *Connection) {
	defer func() {
		s.removeConnection(conn.ID)
		if err := conn.Conn.Close(); err != nil {
			logger.Debug("Failed to close WebSocket connection: %v", err)
		}
	}()

	conn.Conn.SetReadLimit(512)
	if err := conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		logger.Debug("Failed to set read deadline: %v", err)
	}
	conn.Conn.SetPongHandler(func(string) error {
		conn.LastPing = time.Now()
		return conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		messageType, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket error: %v", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			s.handleMessage(conn, data)
		}
	}
}

// writeConnection handles outgoing messages to dashboard
func (s *StreamingAPI) writeConnection(conn *Connection) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		if err := conn.Conn.Close(); err != nil {
			logger.Debug("Failed to close WebSocket connection in write handler: %v", err)
		}
	}()

	for {
		select {
		case event, ok := <-conn.Send:
			if err := conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logger.Debug("Failed to set write deadline: %v", err)
			}
			if !ok {
				if err := conn.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					logger.Debug("Failed to write close message: %v", err)
				}
				return
			}

			data, err := event.ToJSON()
			if err != nil {
				logger.Error("Failed to serialize event: %v", err)
				continue
			}

			if err := conn.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logger.Error("Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			if err := conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logger.Debug("Failed to set write deadline for ping: %v", err)
			}
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from dashboard
func (s *StreamingAPI) handleMessage(conn *Connection, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("Invalid message format: %v", err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "filter":
		s.updateFilter(conn, msg)
	case "ping":
		conn.LastPing = time.Now()
	case "metadata":
		s.updateMetadata(conn, msg)
	}
}

// handleEvent processes events from the event bus
func (s *StreamingAPI) handleEvent(event *Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if s.shouldSendEvent(conn, event) {
			select {
			case conn.Send <- event:
			default:
				// Channel full, remove connection
				go s.removeConnection(conn.ID)
			}
		}
	}
}

// shouldSendEvent checks if event matches connection filter
func (s *StreamingAPI) shouldSendEvent(conn *Connection, event *Event) bool {
	filter := conn.Filter

	// Check event types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespaces
	if len(filter.Namespaces) > 0 && event.Namespace != "" {
		found := false
		for _, ns := range filter.Namespaces {
			if ns == event.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check severities
	if len(filter.Severities) > 0 {
		found := false
		for _, sev := range filter.Severities {
			if sev == event.Severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check timestamp
	if filter.Since != nil && event.Timestamp.Before(*filter.Since) {
		return false
	}

	return true
}

// removeConnection removes a connection from the registry
func (s *StreamingAPI) removeConnection(connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.connections[connectionID]; exists {
		close(conn.Send)
		delete(s.connections, connectionID)
		logger.Info("📡 Dashboard disconnected: %s (remaining: %d)", conn.DashboardID, len(s.connections))
	}
}

// cleanupConnections removes stale connections
func (s *StreamingAPI) cleanupConnections(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, conn := range s.connections {
				if time.Since(conn.LastPing) > s.config.ConnectionTimeout {
					_ = conn.Conn.Close()
					close(conn.Send)
					delete(s.connections, id)
					logger.Info("🧹 Cleaned up stale connection: %s", conn.DashboardID)
				}
			}
			s.mu.Unlock()
		}
	}
}

// updateFilter updates connection event filter
func (s *StreamingAPI) updateFilter(conn *Connection, msg map[string]interface{}) {
	if filterData, ok := msg["filter"]; ok {
		if filterBytes, err := json.Marshal(filterData); err == nil {
			var newFilter EventFilter
			if json.Unmarshal(filterBytes, &newFilter) == nil {
				conn.Filter = &newFilter
				logger.Info("📋 Updated filter for connection: %s", conn.DashboardID)
			}
		}
	}
}

// updateMetadata updates connection metadata
func (s *StreamingAPI) updateMetadata(conn *Connection, msg map[string]interface{}) {
	if metadata, ok := msg["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				conn.Metadata[k] = str
			}
		}
	}
}

// parseFilterFromRequest parses filter parameters from HTTP request
func (s *StreamingAPI) parseFilterFromRequest(r *http.Request, filter *EventFilter) {
	// Parse query parameters and populate filter
	// Implementation details for parsing URL parameters
}

// handleHealth returns streaming API health status
func (s *StreamingAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	connCount := len(s.connections)
	s.mu.RUnlock()

	status := map[string]interface{}{
		"status":         "healthy",
		"connections":    connCount,
		"maxConnections": s.config.MaxConnections,
		"timestamp":      time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// handleConnections returns active connections info
func (s *StreamingAPI) handleConnections(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	connections := make([]map[string]interface{}, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, map[string]interface{}{
			"id":          conn.ID,
			"dashboardId": conn.DashboardID,
			"lastPing":    conn.LastPing,
			"metadata":    conn.Metadata,
		})
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"connections": connections,
		"total":       len(connections),
	})
}

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	return fmt.Sprintf("conn-%d-%s", time.Now().UnixNano(), randomString(6))
}
