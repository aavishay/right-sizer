// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "right-sizer/api/grpc/v1"
	"right-sizer/config"
	"right-sizer/events"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/remediation"
)

// Server implements the RightSizer gRPC API
type Server struct {
	pb.UnimplementedRightSizerServiceServer

	config            *config.Config
	eventBus          *events.EventBus
	remediationEngine *remediation.Engine
	metricsProvider   metrics.Provider

	// gRPC server
	server   *grpc.Server
	listener net.Listener

	// Authentication
	authTokens map[string]bool
}

// NewServer creates a new gRPC server
func NewServer(
	config *config.Config,
	eventBus *events.EventBus,
	remediationEngine *remediation.Engine,
	metricsProvider metrics.Provider,
) *Server {
	return &Server{
		config:            config,
		eventBus:          eventBus,
		remediationEngine: remediationEngine,
		metricsProvider:   metricsProvider,
		authTokens:        make(map[string]bool),
	}
}

// Start starts the gRPC server
func (s *Server) Start(address string, tlsConfig *tls.Config) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	s.listener = listener

	// Configure gRPC server options
	var opts []grpc.ServerOption

	// Add TLS if configured
	if tlsConfig != nil {
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
	}

	// Add authentication interceptor
	opts = append(opts,
		grpc.UnaryInterceptor(s.authUnaryInterceptor),
		grpc.StreamInterceptor(s.authStreamInterceptor),
	)

	s.server = grpc.NewServer(opts...)
	pb.RegisterRightSizerServiceServer(s.server, s)

	logger.Info("Starting gRPC server on %s", address)

	if err := s.server.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

// Stop stops the gRPC server gracefully
func (s *Server) Stop() {
	if s.server != nil {
		logger.Info("Stopping gRPC server")
		s.server.GracefulStop()
	}
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			logger.Error("Failed to close gRPC listener: %v", err)
		}
	}
}

// GetClusterInfo returns cluster information
func (s *Server) GetClusterInfo(ctx context.Context, req *emptypb.Empty) (*pb.ClusterInfo, error) {
	return &pb.ClusterInfo{
		ClusterId:   s.config.ClusterID,
		Name:        s.config.ClusterName,
		Environment: s.config.Environment,
		Version:     s.config.Version,
		Capabilities: []string{
			"resource-optimization",
			"event-streaming",
			"automated-remediation",
			"metrics-collection",
		},
		Status: pb.ClusterStatus_CLUSTER_STATUS_HEALTHY,
	}, nil
}

// GetMetrics returns cluster metrics
func (s *Server) GetMetrics(ctx context.Context, req *pb.MetricsRequest) (*pb.MetricsResponse, error) {
	// Convert timestamp
	var since time.Time
	if req.Since != nil {
		since = req.Since.AsTime()
	} else {
		since = time.Now().Add(-1 * time.Hour) // Default to last hour
	}

	// Get metrics based on type
	var metrics []*pb.Metric
	var err error

	switch req.Type {
	case pb.MetricType_METRIC_TYPE_RESOURCE:
		metrics, err = s.getResourceMetrics(since, req.Namespace, req.PodName)
	case pb.MetricType_METRIC_TYPE_PERFORMANCE:
		metrics, err = s.getPerformanceMetrics(since, req.Namespace, req.PodName)
	case pb.MetricType_METRIC_TYPE_USAGE:
		metrics, err = s.getUsageMetrics(since, req.Namespace, req.PodName)
	default:
		metrics, err = s.getAllMetrics(since, req.Namespace, req.PodName)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get metrics: %v", err)
	}

	return &pb.MetricsResponse{
		Metrics:   metrics,
		Timestamp: timestamppb.Now(),
	}, nil
}

// StreamEvents streams cluster events
func (s *Server) StreamEvents(req *pb.EventStreamRequest, stream pb.RightSizerService_StreamEventsServer) error {
	// Create event filter
	filter := &events.EventFilter{
		EventTypes: convertEventTypes(req.EventTypes),
		Namespaces: req.Namespaces,
		PodNames:   req.PodNames,
		Severities: convertSeverities(req.Severities),
		Tags:       req.Tags,
	}

	// Subscribe to events
	eventChan := make(chan *events.Event, 100)
	subscriptionID := s.eventBus.SubscribeChannel(filter, eventChan)
	defer s.eventBus.Unsubscribe(subscriptionID)

	logger.Info("Started event stream for client")

	// Stream events until context is cancelled
	for {
		select {
		case <-stream.Context().Done():
			logger.Info("Event stream cancelled by client")
			return nil
		case event := <-eventChan:
			pbEvent := s.convertEventToProto(event)
			if err := stream.Send(&pb.EventStreamResponse{
				Event: pbEvent,
			}); err != nil {
				logger.Error("Failed to send event to stream: %v", err)
				return err
			}
		}
	}
}

// ExecuteAction executes a remediation action
func (s *Server) ExecuteAction(ctx context.Context, req *pb.ActionRequest) (*pb.ActionResponse, error) {
	// Convert proto action to internal action
	action := s.convertProtoToAction(req.Action)

	// Execute the action
	if err := s.remediationEngine.ExecuteAction(ctx, action); err != nil {
		return &pb.ActionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to execute action: %v", err),
		}, nil
	}

	return &pb.ActionResponse{
		Success:  true,
		Message:  "Action executed successfully",
		ActionId: action.ID,
	}, nil
}

// GetActionStatus returns the status of an action
func (s *Server) GetActionStatus(ctx context.Context, req *pb.ActionStatusRequest) (*pb.ActionStatusResponse, error) {
	action, exists := s.remediationEngine.GetAction(req.ActionId)
	if !exists {
		return nil, status.Errorf(codes.NotFound, "action %s not found", req.ActionId)
	}

	return &pb.ActionStatusResponse{
		Status:    s.convertStatusToProto(action.Status),
		Result:    action.Result,
		Error:     action.Error,
		UpdatedAt: timestamppb.New(action.UpdatedAt),
	}, nil
}

// Health check
func (s *Server) HealthCheck(ctx context.Context, req *emptypb.Empty) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status: pb.HealthStatus_HEALTH_STATUS_SERVING,
		Checks: map[string]pb.HealthStatus{
			"event-bus":          pb.HealthStatus_HEALTH_STATUS_SERVING,
			"remediation-engine": pb.HealthStatus_HEALTH_STATUS_SERVING,
			"metrics-provider":   pb.HealthStatus_HEALTH_STATUS_SERVING,
		},
	}, nil
}

// Authentication interceptors
func (s *Server) authUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := s.authenticate(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (s *Server) authStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := s.authenticate(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

func (s *Server) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return status.Errorf(codes.Unauthenticated, "missing authorization token")
	}

	token := tokens[0]
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Validate JWT token
	if !s.isValidToken(token) {
		return status.Errorf(codes.Unauthenticated, "invalid token")
	}

	return nil
}

// isValidToken checks if a token is valid with signature and expiration validation
func (s *Server) isValidToken(token string) bool {
	// Validate JWT tokens with proper signature and expiration checks
	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC (prevent algorithm substitution attacks)
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))

	if err != nil {
		logger.Warn("Token validation failed: %v", err)
		return false
	}

	if !parsedToken.Valid {
		logger.Warn("Token validation failed: invalid token")
		return false
	}

	// Verify standard claims including expiration
	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		// Check expiration explicitly
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				logger.Warn("Token validation failed: token has expired")
				return false
			}
		}
		// Check issued-at time to prevent future tokens
		if iat, ok := claims["iat"].(float64); ok {
			if time.Now().Unix() < int64(iat) {
				logger.Warn("Token validation failed: token used before issued")
				return false
			}
		}
	}

	return true
}

// Helper methods for metrics
func (s *Server) getResourceMetrics(since time.Time, namespace, podName string) ([]*pb.Metric, error) {
	// Implementation would query the metrics provider
	// This is a simplified version
	return []*pb.Metric{
		{
			Name:      "cpu_usage",
			Value:     0.45,
			Unit:      "cores",
			Timestamp: timestamppb.Now(),
			Labels: map[string]string{
				"namespace": namespace,
				"pod":       podName,
			},
		},
		{
			Name:      "memory_usage",
			Value:     1024 * 1024 * 512, // 512MB
			Unit:      "bytes",
			Timestamp: timestamppb.Now(),
			Labels: map[string]string{
				"namespace": namespace,
				"pod":       podName,
			},
		},
	}, nil
}

func (s *Server) getPerformanceMetrics(since time.Time, namespace, podName string) ([]*pb.Metric, error) {
	return []*pb.Metric{
		{
			Name:      "response_time",
			Value:     120.5,
			Unit:      "ms",
			Timestamp: timestamppb.Now(),
		},
	}, nil
}

func (s *Server) getUsageMetrics(since time.Time, namespace, podName string) ([]*pb.Metric, error) {
	return []*pb.Metric{
		{
			Name:      "cpu_utilization",
			Value:     0.85,
			Unit:      "percent",
			Timestamp: timestamppb.Now(),
		},
	}, nil
}

func (s *Server) getAllMetrics(since time.Time, namespace, podName string) ([]*pb.Metric, error) {
	var allMetrics []*pb.Metric

	resourceMetrics, _ := s.getResourceMetrics(since, namespace, podName)
	allMetrics = append(allMetrics, resourceMetrics...)

	perfMetrics, _ := s.getPerformanceMetrics(since, namespace, podName)
	allMetrics = append(allMetrics, perfMetrics...)

	usageMetrics, _ := s.getUsageMetrics(since, namespace, podName)
	allMetrics = append(allMetrics, usageMetrics...)

	return allMetrics, nil
}

// Conversion helpers
func (s *Server) convertEventToProto(event *events.Event) *pb.Event {
	return &pb.Event{
		Id:            event.ID,
		Type:          string(event.Type),
		ClusterId:     event.ClusterID,
		Namespace:     event.Namespace,
		ResourceName:  event.Resource,
		Severity:      s.convertSeverityToProto(event.Severity),
		Message:       event.Message,
		Timestamp:     timestamppb.New(event.Timestamp),
		Details:       convertDetailsToProto(event.Details),
		Tags:          event.Tags,
		CorrelationId: event.CorrelationID,
	}
}

func (s *Server) convertSeverityToProto(severity events.Severity) pb.Severity {
	switch severity {
	case events.SeverityInfo:
		return pb.Severity_SEVERITY_INFO
	case events.SeverityWarning:
		return pb.Severity_SEVERITY_WARNING
	case events.SeverityError:
		return pb.Severity_SEVERITY_ERROR
	case events.SeverityCritical:
		return pb.Severity_SEVERITY_CRITICAL
	default:
		return pb.Severity_SEVERITY_INFO
	}
}

func (s *Server) convertProtoToAction(pbAction *pb.Action) *remediation.Action {
	return &remediation.Action{
		ID:   pbAction.Id,
		Type: remediation.ActionType(pbAction.Type),
		Target: remediation.ActionTarget{
			Namespace: pbAction.Target.Namespace,
			Name:      pbAction.Target.Name,
			Kind:      pbAction.Target.Kind,
			Container: pbAction.Target.Container,
		},
		Parameters: convertProtoDetailsToMap(pbAction.Parameters),
		Risk:       s.convertProtoToRisk(pbAction.Risk),
		Reason:     pbAction.Reason,
		Source:     "grpc-api",
		Priority:   s.convertProtoToPriority(pbAction.Priority),
		Timeout:    time.Duration(pbAction.TimeoutSeconds) * time.Second,
		CreatedAt:  time.Now(),
		Status:     remediation.StatusPending,
	}
}

func (s *Server) convertStatusToProto(status remediation.ActionStatus) pb.ActionStatus {
	switch status {
	case remediation.StatusPending:
		return pb.ActionStatus_ACTION_STATUS_PENDING
	case remediation.StatusRunning:
		return pb.ActionStatus_ACTION_STATUS_RUNNING
	case remediation.StatusCompleted:
		return pb.ActionStatus_ACTION_STATUS_COMPLETED
	case remediation.StatusFailed:
		return pb.ActionStatus_ACTION_STATUS_FAILED
	case remediation.StatusCancelled:
		return pb.ActionStatus_ACTION_STATUS_CANCELLED
	default:
		return pb.ActionStatus_ACTION_STATUS_PENDING
	}
}

func (s *Server) convertProtoToRisk(risk pb.RiskLevel) remediation.RiskLevel {
	switch risk {
	case pb.RiskLevel_RISK_LEVEL_LOW:
		return remediation.RiskLow
	case pb.RiskLevel_RISK_LEVEL_MEDIUM:
		return remediation.RiskMedium
	case pb.RiskLevel_RISK_LEVEL_HIGH:
		return remediation.RiskHigh
	default:
		return remediation.RiskLow
	}
}

func (s *Server) convertProtoToPriority(priority pb.Priority) remediation.Priority {
	switch priority {
	case pb.Priority_PRIORITY_LOW:
		return remediation.PriorityLow
	case pb.Priority_PRIORITY_MEDIUM:
		return remediation.PriorityMedium
	case pb.Priority_PRIORITY_HIGH:
		return remediation.PriorityHigh
	case pb.Priority_PRIORITY_CRITICAL:
		return remediation.PriorityCritical
	default:
		return remediation.PriorityLow
	}
}

// Utility functions
func convertEventTypes(pbTypes []pb.EventType) []events.EventType {
	var types []events.EventType
	for _, pbType := range pbTypes {
		// Convert proto enum to string and then to internal type
		types = append(types, events.EventType(pbType.String()))
	}
	return types
}

func convertSeverities(pbSeverities []pb.Severity) []events.Severity {
	var severities []events.Severity
	for _, pbSev := range pbSeverities {
		switch pbSev {
		case pb.Severity_SEVERITY_INFO:
			severities = append(severities, events.SeverityInfo)
		case pb.Severity_SEVERITY_WARNING:
			severities = append(severities, events.SeverityWarning)
		case pb.Severity_SEVERITY_ERROR:
			severities = append(severities, events.SeverityError)
		case pb.Severity_SEVERITY_CRITICAL:
			severities = append(severities, events.SeverityCritical)
		}
	}
	return severities
}

func convertDetailsToProto(details map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range details {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func convertProtoDetailsToMap(details map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range details {
		result[k] = v
	}
	return result
}
