// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package admission provides admission webhook functionality for the right-sizer.
package admission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/validation"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	readHeaderTimeout    = 30 * time.Second
	trueStr              = "true"
	initialPatchCapacity = 2
)

// WebhookServer represents the admission webhook server
type WebhookServer struct {
	server       *http.Server
	client       client.Client
	clientset    *kubernetes.Clientset
	validator    *validation.ResourceValidator
	config       *config.Config
	metrics      *metrics.OperatorMetrics
	codecs       serializer.CodecFactory
	deserializer runtime.Decoder
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	CertPath          string
	KeyPath           string
	Port              int
	EnableValidation  bool
	EnableMutation    bool
	DryRun            bool
	RequireAnnotation bool
}

// NewWebhookServer creates a new admission webhook server
func NewWebhookServer(
	client client.Client,
	clientset *kubernetes.Clientset,
	validator *validation.ResourceValidator,
	cfg *config.Config,
	metrics *metrics.OperatorMetrics,
	webhookConfig WebhookConfig,
) (*WebhookServer, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}
	codecs := serializer.NewCodecFactory(scheme)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", webhookConfig.Port),
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ws := &WebhookServer{
		server:       server,
		client:       client,
		clientset:    clientset,
		validator:    validator,
		config:       cfg,
		metrics:      metrics,
		codecs:       codecs,
		deserializer: codecs.UniversalDeserializer(),
	}

	// Register webhook endpoints
	if webhookConfig.EnableValidation {
		mux.HandleFunc("/validate", ws.handleValidate)
		logger.Info("Registered validation webhook at /validate")
	}

	if webhookConfig.EnableMutation {
		mux.HandleFunc("/mutate", ws.handleMutate)
		logger.Info("Registered mutation webhook at /mutate")
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("healthy")); err != nil {
			logger.Error("Failed to write health response: %v", err)
		}
	})

	return ws, nil
}

// Start starts the webhook server
func (ws *WebhookServer) Start(certPath, keyPath string) error {
	logger.Info("Starting admission webhook server on %s", ws.server.Addr)

	if certPath != "" && keyPath != "" {
		return ws.server.ListenAndServeTLS(certPath, keyPath)
	}

	logger.Warn("Running webhook server without TLS (not recommended for production)")
	return ws.server.ListenAndServe()
}

// Stop stops the webhook server
func (ws *WebhookServer) Stop(ctx context.Context) error {
	logger.Info("Stopping admission webhook server")
	return ws.server.Shutdown(ctx)
}

// handleValidate handles validation admission requests
func (ws *WebhookServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	timer := metrics.NewTimer()
	defer func() {
		if ws.metrics != nil {
			ws.metrics.RecordProcessingDuration("validation_webhook", timer.Duration())
		}
	}()

	body, err := ws.readRequestBody(r)
	if err != nil {
		ws.sendError(w, fmt.Errorf("failed to read request body: %w", err))
		return
	}

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		ws.sendError(w, fmt.Errorf("failed to decode admission review: %w", err))
		return
	}

	response := ws.validatePodResourceChange(r.Context(), &review)
	ws.sendResponse(w, response.Response)
}

// handleMutate handles mutation admission requests
func (ws *WebhookServer) handleMutate(w http.ResponseWriter, r *http.Request) {
	timer := metrics.NewTimer()
	defer func() {
		if ws.metrics != nil {
			ws.metrics.RecordProcessingDuration("mutation_webhook", timer.Duration())
		}
	}()

	body, err := ws.readRequestBody(r)
	if err != nil {
		ws.sendError(w, fmt.Errorf("failed to read request body: %w", err))
		return
	}

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		ws.sendError(w, fmt.Errorf("failed to decode admission review: %w", err))
		return
	}

	response := ws.mutatePodResources(&review)
	ws.sendResponse(w, response.Response)
}

// validatePodResourceChange validates pod resource changes
func (ws *WebhookServer) validatePodResourceChange(ctx context.Context, review *admissionv1.AdmissionReview) admissionv1.AdmissionReview {
	req := review.Request
	response := &admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
	}

	// Only validate pods
	if req.Kind.Kind != "Pod" {
		return admissionv1.AdmissionReview{Response: response}
	}

	// Skip if not enabled for this namespace
	if !ws.config.IsNamespaceIncluded(req.Namespace) {
		logger.Debug("Skipping validation for namespace %s (not included)", req.Namespace)
		return admissionv1.AdmissionReview{Response: response}
	}

	// Parse old and new pod objects
	var oldPod, newPod corev1.Pod
	if req.OldObject.Raw != nil {
		if err := json.Unmarshal(req.OldObject.Raw, &oldPod); err != nil {
			response.Allowed = false
			response.Result = &metav1.Status{
				Message: fmt.Sprintf("Failed to parse old pod: %v", err),
			}
			return admissionv1.AdmissionReview{Response: response}
		}
	}

	if err := json.Unmarshal(req.Object.Raw, &newPod); err != nil {
		response.Allowed = false
		response.Result = &metav1.Status{
			Message: fmt.Sprintf("Failed to parse new pod: %v", err),
		}
		return admissionv1.AdmissionReview{Response: response}
	}

	// Skip if pod has opt-out annotation
	if ws.shouldSkipValidation(&newPod) {
		logger.Debug("Skipping validation for pod %s/%s (opt-out annotation)", newPod.Namespace, newPod.Name)
		return admissionv1.AdmissionReview{Response: response}
	}

	// Validate resource changes for each container
	var validationErrors []string
	var validationWarnings []string

	for i := range newPod.Spec.Containers {
		container := &newPod.Spec.Containers[i]
		// Compare with old container resources if available
		var oldResources corev1.ResourceRequirements
		if req.Operation == admissionv1.Update && len(oldPod.Spec.Containers) > i {
			oldResources = oldPod.Spec.Containers[i].Resources
		}

		// Skip if no resource change
		if ws.areResourcesEqual(oldResources, container.Resources) {
			continue
		}

		// Validate the resource change
		validationResult := ws.validator.ValidateResourceChange(
			ctx,
			&newPod,
			container.Resources,
			container.Name,
		)

		if !validationResult.IsValid() {
			validationErrors = append(validationErrors, validationResult.Errors...)
		}

		if validationResult.HasWarnings() {
			validationWarnings = append(validationWarnings, validationResult.Warnings...)
		}

		// Record metrics
		if ws.metrics != nil && !validationResult.IsValid() {
			ws.metrics.RecordResourceValidationError("admission_webhook", strings.Join(validationResult.Errors, "; "))
		}
	}

	// Set response based on validation results
	if len(validationErrors) > 0 {
		response.Allowed = false
		response.Result = &metav1.Status{
			Code:    http.StatusForbidden,
			Message: "Resource validation failed: " + strings.Join(validationErrors, "; "),
		}
	} else if len(validationWarnings) > 0 {
		// Allow but include warnings
		response.Warnings = validationWarnings
		logger.Warn("Pod %s/%s has resource validation warnings: %s",
			newPod.Namespace, newPod.Name, strings.Join(validationWarnings, "; "))
	}

	return admissionv1.AdmissionReview{Response: response}
}

// mutatePodResources applies automatic resource adjustments
func (ws *WebhookServer) mutatePodResources(review *admissionv1.AdmissionReview) admissionv1.AdmissionReview {
	req := review.Request
	response := &admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
	}

	// Only mutate pods
	if req.Kind.Kind != "Pod" {
		return admissionv1.AdmissionReview{Response: response}
	}

	// Skip if not enabled for this namespace
	if !ws.config.IsNamespaceIncluded(req.Namespace) {
		return admissionv1.AdmissionReview{Response: response}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		response.Allowed = false
		response.Result = &metav1.Status{
			Message: fmt.Sprintf("Failed to parse pod: %v", err),
		}
		return admissionv1.AdmissionReview{Response: response}
	}

	// Skip if pod has opt-out annotation
	if ws.shouldSkipMutation(&pod) {
		return admissionv1.AdmissionReview{Response: response}
	}

	// Check current QoS class before mutation
	currentQoS := ws.getQoSClass(&pod)
	if currentQoS == corev1.PodQOSGuaranteed {
		logger.Debug("Pod %s/%s has Guaranteed QoS, will maintain during mutation", pod.Namespace, pod.Name)
	}

	// Apply mutations
	patches := ws.generateResourcePatches(&pod)
	if len(patches) > 0 {
		patchBytes, err := json.Marshal(patches)
		if err != nil {
			logger.Error("Failed to marshal patches: %v", err)
			return admissionv1.AdmissionReview{Response: response}
		}

		patchType := admissionv1.PatchTypeJSONPatch
		response.Patch = patchBytes
		response.PatchType = &patchType

		logger.Info("Applied resource patches to pod %s/%s (QoS: %s)", pod.Namespace, pod.Name, currentQoS)
	}

	return admissionv1.AdmissionReview{Response: response}
}

// shouldSkipValidation checks if validation should be skipped for this pod
func (ws *WebhookServer) shouldSkipValidation(pod *corev1.Pod) bool {
	// Check for opt-out annotation
	if pod.Annotations != nil {
		if skip, exists := pod.Annotations["rightsizer.io/skip-validation"]; exists && skip == trueStr {
			return true
		}
		if disable, exists := pod.Annotations["rightsizer.io/disable"]; exists && disable == trueStr {
			return true
		}
	}

	// Check for opt-out label
	if pod.Labels != nil {
		if skip, exists := pod.Labels["rightsizer.skip-validation"]; exists && skip == trueStr {
			return true
		}
	}

	return false
}

// shouldSkipMutation checks if mutation should be skipped for this pod
func (ws *WebhookServer) shouldSkipMutation(pod *corev1.Pod) bool {
	// Check for opt-out annotation
	if pod.Annotations != nil {
		if skip, exists := pod.Annotations["rightsizer.io/skip-mutation"]; exists && skip == trueStr {
			return true
		}
		if disable, exists := pod.Annotations["rightsizer.io/disable"]; exists && disable == trueStr {
			return true
		}
	}

	// Check for opt-out label
	if pod.Labels != nil {
		if skip, exists := pod.Labels["rightsizer.skip-mutation"]; exists && skip == trueStr {
			return true
		}
	}

	return false
}

// areResourcesEqual compares two resource requirements
func (ws *WebhookServer) areResourcesEqual(old, newRes corev1.ResourceRequirements) bool {
	// Compare requests
	if !ws.areResourceListsEqual(old.Requests, newRes.Requests) {
		return false
	}

	// Compare limits
	if !ws.areResourceListsEqual(old.Limits, newRes.Limits) {
		return false
	}

	return true
}

// areResourceListsEqual compares two resource lists
func (ws *WebhookServer) areResourceListsEqual(old, newRes corev1.ResourceList) bool {
	if len(old) != len(newRes) {
		return false
	}

	for k, v := range old {
		if newV, exists := newRes[k]; !exists || !v.Equal(newV) {
			return false
		}
	}

	return true
}

// generateResourcePatches generates JSON patches for resource optimization
func (ws *WebhookServer) generateResourcePatches(pod *corev1.Pod) []JSONPatch {
	patches := make([]JSONPatch, 0, initialPatchCapacity)

	// Determine if we should maintain Guaranteed QoS
	maintainGuaranteed := false
	if pod.Annotations != nil {
		if qos, exists := pod.Annotations["rightsizer.io/qos-class"]; exists && qos == "Guaranteed" {
			maintainGuaranteed = true
		}
	}

	// Add default resource requests if missing
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Resources.Requests == nil {
			cpuRequest := fmt.Sprintf("%dm", ws.config.MinCPURequest)
			memRequest := fmt.Sprintf("%dMi", ws.config.MinMemoryRequest)

			patches = append(patches, JSONPatch{
				Op:   "add",
				Path: fmt.Sprintf("/spec/containers/%d/resources/requests", i),
				Value: map[string]string{
					"cpu":    cpuRequest,
					"memory": memRequest,
				},
			})

			// If maintaining Guaranteed QoS, also add matching limits
			if maintainGuaranteed {
				patches = append(patches, JSONPatch{
					Op:   "add",
					Path: fmt.Sprintf("/spec/containers/%d/resources/limits", i),
					Value: map[string]string{
						"cpu":    cpuRequest,
						"memory": memRequest,
					},
				})
			}
		} else if maintainGuaranteed && container.Resources.Limits == nil {
			// If we have requests but no limits and need Guaranteed QoS, add matching limits
			patches = append(patches, JSONPatch{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/resources/limits", i),
				Value: container.Resources.Requests,
			})
		}

		// Add resize policy only if the UpdateResizePolicy feature flag is enabled
		// This enables in-place updates without container restart (K8s 1.33+)
		if ws.config != nil && ws.config.UpdateResizePolicy {
			// Check if container already has a resize policy
			hasResizePolicy := container.ResizePolicy != nil && len(container.ResizePolicy) > 0

			resizePolicy := []corev1.ContainerResizePolicy{
				{
					ResourceName:  corev1.ResourceCPU,
					RestartPolicy: corev1.NotRequired,
				},
				{
					ResourceName:  corev1.ResourceMemory,
					RestartPolicy: corev1.NotRequired,
				},
			}

			// Use "add" if no resize policy exists, "replace" if it does
			resizePolicyOp := "add"
			if hasResizePolicy {
				resizePolicyOp = "replace"
			}

			patches = append(patches, JSONPatch{
				Op:    resizePolicyOp,
				Path:  fmt.Sprintf("/spec/containers/%d/resizePolicy", i),
				Value: resizePolicy,
			})
		}
	}

	// Add labels for tracking
	if pod.Labels == nil {
		patches = append(patches, JSONPatch{
			Op:    "add",
			Path:  "/metadata/labels",
			Value: map[string]string{},
		})
	}

	patches = append(patches, JSONPatch{
		Op:    "add",
		Path:  "/metadata/labels/rightsizer.io~1managed",
		Value: "true",
	})

	return patches
}

// JSONPatch represents a JSON patch operation
type JSONPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// getQoSClass determines the QoS class of a pod
func (ws *WebhookServer) getQoSClass(pod *corev1.Pod) corev1.PodQOSClass {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)
	zeroQuantity := resource.MustParse("0")
	isGuaranteed := true

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		// Accumulate requests
		for name, quantity := range container.Resources.Requests {
			if value, exists := requests[name]; !exists {
				requests[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				requests[name] = value
			}
		}

		// Accumulate limits
		for name, quantity := range container.Resources.Limits {
			if value, exists := limits[name]; !exists {
				limits[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}

	// Check if guaranteed - must have both CPU and memory requests/limits and they must be equal
	if len(requests) < 2 || len(limits) < 2 {
		isGuaranteed = false
	} else {
		// Check CPU and Memory specifically
		cpuReq, hasCPUReq := requests[corev1.ResourceCPU]
		cpuLim, hasCPULim := limits[corev1.ResourceCPU]
		memReq, hasMemReq := requests[corev1.ResourceMemory]
		memLim, hasMemLim := limits[corev1.ResourceMemory]

		if !hasCPUReq || !hasCPULim || !hasMemReq || !hasMemLim {
			isGuaranteed = false
		} else if cpuReq.Cmp(cpuLim) != 0 || memReq.Cmp(memLim) != 0 {
			isGuaranteed = false
		}
	}

	if isGuaranteed {
		return corev1.PodQOSGuaranteed
	}

	// Check if burstable (has some requests or limits)
	for _, req := range requests {
		if req.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	for _, limit := range limits {
		if limit.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	return corev1.PodQOSBestEffort
}

// readRequestBody reads and validates the request body
func (ws *WebhookServer) readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, errors.New("request body is empty")
	}
	defer r.Body.Close()

	if r.Header.Get("Content-Type") != "application/json" {
		return nil, errors.New("expected Content-Type application/json")
	}

	body := make([]byte, r.ContentLength)
	if _, err := r.Body.Read(body); err != nil {
		return nil, err
	}

	return body, nil
}

// sendResponse sends an admission review response
func (ws *WebhookServer) sendResponse(w http.ResponseWriter, response *admissionv1.AdmissionResponse) {
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: response,
	}

	respBytes, err := json.Marshal(review)
	if err != nil {
		logger.Error("Failed to marshal admission response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBytes); err != nil {
		logger.Error("Failed to write admission response: %v", err)
	}
}

// sendError sends an error response
func (ws *WebhookServer) sendError(w http.ResponseWriter, err error) {
	logger.Error("Admission webhook error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)

	if ws.metrics != nil {
		ws.metrics.RecordProcessingError("", "", "admission_webhook")
	}
}

// WebhookManager manages the lifecycle of admission webhooks
type WebhookManager struct {
	server *WebhookServer
	config WebhookConfig
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(
	client client.Client,
	clientset *kubernetes.Clientset,
	validator *validation.ResourceValidator,
	cfg *config.Config,
	metrics *metrics.OperatorMetrics,
	webhookConfig WebhookConfig,
) *WebhookManager {
	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	if err != nil {
		logger.Error("Failed to create webhook server: %v", err)
		return nil
	}
	return &WebhookManager{
		server: server,
		config: webhookConfig,
	}
}

// Start starts the webhook manager
func (wm *WebhookManager) Start(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		errChan <- wm.server.Start(wm.config.CertPath, wm.config.KeyPath)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return wm.server.Stop(ctx)
	}
}

// Stop stops the webhook manager
func (wm *WebhookManager) Stop(ctx context.Context) error {
	return wm.server.Stop(ctx)
}
