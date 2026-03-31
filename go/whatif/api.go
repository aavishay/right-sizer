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

package whatif

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// APIHandler provides HTTP endpoints for what-if analysis
type APIHandler struct {
	analyzer *Analyzer
	logger   *zap.Logger
}

// NewAPIHandler creates a new API handler for what-if analysis
func NewAPIHandler(analyzer *Analyzer, logger *zap.Logger) *APIHandler {
	return &APIHandler{
		analyzer: analyzer,
		logger:   logger,
	}
}

// RegisterRoutes registers the what-if API routes
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/whatif/analyze", h.handleAnalyze)
	mux.HandleFunc("/api/v1/whatif/compare", h.handleCompare)
	mux.HandleFunc("/api/v1/whatif/health", h.handleHealth)
}

// handleAnalyze handles single scenario analysis requests
func (h *APIHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Failed to decode analysis request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.analyzer.Analyze(r.Context(), req)
	if err != nil {
		h.logger.Error("Analysis failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// CompareRequest represents a request to compare multiple scenarios
type CompareRequest struct {
	Base      AnalysisRequest   `json:"base"`
	Scenarios []AnalysisRequest `json:"scenarios"`
}

// CompareResponse represents the response for scenario comparison
type CompareResponse struct {
	Results []*AnalysisResult `json:"results"`
	Count   int               `json:"count"`
}

// handleCompare handles multi-scenario comparison requests
func (h *APIHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Failed to decode compare request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	results, err := h.analyzer.CompareScenarios(r.Context(), req.Base, req.Scenarios)
	if err != nil {
		h.logger.Error("Scenario comparison failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := CompareResponse{
		Results: results,
		Count:   len(results),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleHealth provides a health check endpoint
func (h *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
