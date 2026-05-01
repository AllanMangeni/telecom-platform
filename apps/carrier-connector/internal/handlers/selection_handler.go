package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/smdp"
	"github.com/sirupsen/logrus"
)

// SelectionHandler handles carrier selection API endpoints
type SelectionHandler struct {
	manager *smdp.SMDPManager
	logger  *logrus.Logger
}

// NewSelectionHandler creates a new selection handler
func NewSelectionHandler(manager *smdp.SMDPManager) *SelectionHandler {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &SelectionHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers selection-related routes
func (h *SelectionHandler) RegisterRoutes(mux *http.ServeMux) {
	// Carrier selection endpoints
	mux.HandleFunc("/api/v1/selection/optimal", h.SelectOptimalCarrier)
	mux.HandleFunc("/api/v1/selection/carrier", h.SelectCarrier)
	mux.HandleFunc("/api/v1/selection/history/", h.GetSelectionHistory)
	mux.HandleFunc("/api/v1/selection/learning", h.UpdateLearning)

	// Analytics endpoints
	mux.HandleFunc("/api/v1/selection/analytics/selection", h.GetSelectionAnalytics)
	mux.HandleFunc("/api/v1/selection/analytics/performance", h.GetPerformanceAnalytics)
}

// SelectOptimalCarrierRequest represents the request for optimal carrier selection
type SelectOptimalCarrierRequest struct {
	Region            string  `json:"region"`
	ProfileType       string  `json:"profile_type"`
	Urgency           string  `json:"urgency"`
	CostSensitivity   float64 `json:"cost_sensitivity"`
	PerformanceWeight float64 `json:"performance_weight"`
	ReliabilityWeight float64 `json:"reliability_weight"`
}

// SelectOptimalCarrierResponse represents the response for optimal carrier selection
type SelectOptimalCarrierResponse struct {
	Success      bool               `json:"success"`
	CarrierScore *smdp.CarrierScore `json:"carrier_score,omitempty"`
	Error        string             `json:"error,omitempty"`
}

// SelectOptimalCarrier selects the optimal carrier based on criteria
func (h *SelectionHandler) SelectOptimalCarrier(w http.ResponseWriter, r *http.Request) {
	var req SelectOptimalCarrierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Urgency == "" {
		req.Urgency = "medium"
	}
	if req.CostSensitivity < 0 || req.CostSensitivity > 1 {
		req.CostSensitivity = 0.5
	}
	if req.PerformanceWeight < 0 || req.PerformanceWeight > 1 {
		req.PerformanceWeight = 0.4
	}
	if req.ReliabilityWeight < 0 || req.ReliabilityWeight > 1 {
		req.ReliabilityWeight = 0.4
	}

	criteria := &smdp.SelectionCriteria{
		Region:            req.Region,
		ProfileType:       req.ProfileType,
		Urgency:           req.Urgency,
		CostSensitivity:   req.CostSensitivity,
		PerformanceWeight: req.PerformanceWeight,
		ReliabilityWeight: req.ReliabilityWeight,
	}

	score, err := h.manager.SelectOptimalCarrier(r.Context(), criteria)
	if err != nil {
		h.logger.WithError(err).Error("Failed to select optimal carrier")
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := SelectOptimalCarrierResponse{
		Success:      true,
		CarrierScore: score,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// SelectCarrierResponse represents the response for carrier selection
type SelectCarrierResponse struct {
	Success bool          `json:"success"`
	Carrier *smdp.Carrier `json:"carrier,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// SelectCarrier selects a carrier using default criteria
func (h *SelectionHandler) SelectCarrier(w http.ResponseWriter, r *http.Request) {
	carrier, err := h.manager.SelectCarrier(r.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to select carrier")
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := SelectCarrierResponse{
		Success: true,
		Carrier: carrier,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// SelectionHistoryResponse represents the response for selection history
type SelectionHistoryResponse struct {
	Success bool                `json:"success"`
	History []smdp.CarrierScore `json:"history,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// GetSelectionHistory returns the selection history for a carrier
func (h *SelectionHandler) GetSelectionHistory(w http.ResponseWriter, r *http.Request) {
	// Extract carrier ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/selection/history/")
	carrierID := strings.TrimSuffix(path, "/")

	if carrierID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Carrier ID is required")
		return
	}

	history := h.manager.GetSelectionHistory(carrierID)

	response := SelectionHistoryResponse{
		Success: true,
		History: history,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// UpdateLearningRequest represents the request for updating learning
type UpdateLearningRequest struct {
	CarrierID         string  `json:"carrier_id"`
	ActualPerformance float64 `json:"actual_performance"`
}

// UpdateLearningResponse represents the response for updating learning
type UpdateLearningResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// UpdateLearning updates the selection algorithm with performance feedback
func (h *SelectionHandler) UpdateLearning(w http.ResponseWriter, r *http.Request) {
	var req UpdateLearningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.CarrierID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Carrier ID is required")
		return
	}
	if req.ActualPerformance < 0 || req.ActualPerformance > 100 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Actual performance must be between 0 and 100")
		return
	}

	h.manager.UpdateLearning(req.CarrierID, req.ActualPerformance)

	response := UpdateLearningResponse{
		Success: true,
		Message: "Learning updated successfully",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// SelectionAnalyticsResponse represents the response for selection analytics
type SelectionAnalyticsResponse struct {
	Success   bool                `json:"success"`
	Analytics *SelectionAnalytics `json:"analytics,omitempty"`
	Error     string              `json:"error,omitempty"`
}

// SelectionAnalytics represents carrier selection analytics
type SelectionAnalytics struct {
	TotalSelections    int                     `json:"total_selections"`
	CarrierStats       map[string]*CarrierStat `json:"carrier_stats"`
	AveragePerformance float64                 `json:"average_performance"`
	TopPerformers      []string                `json:"top_performers"`
	SelectionTrends    map[string]int          `json:"selection_trends"`
}

// CarrierStat represents statistics for a carrier
type CarrierStat struct {
	CarrierID      string  `json:"carrier_id"`
	SelectionCount int     `json:"selection_count"`
	AverageScore   float64 `json:"average_score"`
	SuccessRate    float64 `json:"success_rate"`
	LastSelected   string  `json:"last_selected"`
}

// GetSelectionAnalytics returns selection analytics
func (h *SelectionHandler) GetSelectionAnalytics(w http.ResponseWriter, r *http.Request) {
	// Get carrier status
	carriers := h.manager.GetCarrierStatus()

	analytics := &SelectionAnalytics{
		TotalSelections:    0,
		CarrierStats:       make(map[string]*CarrierStat),
		AveragePerformance: 0,
		TopPerformers:      []string{},
		SelectionTrends:    make(map[string]int),
	}

	totalScore := 0.0
	totalSelections := 0

	for carrierID, carrier := range carriers {
		history := h.manager.GetSelectionHistory(carrierID)

		stat := &CarrierStat{
			CarrierID:      carrierID,
			SelectionCount: len(history),
			AverageScore:   0,
			SuccessRate:    0,
			LastSelected:   "",
		}

		if len(history) > 0 {
			var scoreSum float64
			for _, score := range history {
				scoreSum += score.TotalScore
			}
			stat.AverageScore = scoreSum / float64(len(history))
			stat.LastSelected = history[len(history)-1].SelectedAt.Format("2006-01-02 15:04:05")

			// Calculate success rate from carrier metrics
			if carrier.Metrics.TotalRequests > 0 {
				stat.SuccessRate = float64(carrier.Metrics.SuccessfulRequests) / float64(carrier.Metrics.TotalRequests) * 100
			}
		}

		analytics.CarrierStats[carrierID] = stat
		totalScore += stat.AverageScore
		totalSelections += stat.SelectionCount
	}

	analytics.TotalSelections = totalSelections
	if len(carriers) > 0 {
		analytics.AveragePerformance = totalScore / float64(len(carriers))
	}

	response := SelectionAnalyticsResponse{
		Success:   true,
		Analytics: analytics,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// PerformanceAnalyticsResponse represents the response for performance analytics
type PerformanceAnalyticsResponse struct {
	Success   bool                  `json:"success"`
	Analytics *PerformanceAnalytics `json:"analytics,omitempty"`
	Error     string                `json:"error,omitempty"`
}

// PerformanceAnalytics represents performance analytics
type PerformanceAnalytics struct {
	CarrierPerformance map[string]*PerformanceMetrics `json:"carrier_performance"`
	SystemHealth       *SystemHealth                  `json:"system_health"`
	Recommendations    []string                       `json:"recommendations"`
}

// PerformanceMetrics represents performance metrics for a carrier
type PerformanceMetrics struct {
	CarrierID      string  `json:"carrier_id"`
	ResponseTime   float64 `json:"response_time_ms"`
	SuccessRate    float64 `json:"success_rate"`
	RequestRate    float64 `json:"requests_per_second"`
	HealthScore    float64 `json:"health_score"`
	Recommendation string  `json:"recommendation"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	HealthyCarriers   int     `json:"healthy_carriers"`
	DegradedCarriers  int     `json:"degraded_carriers"`
	UnhealthyCarriers int     `json:"unhealthy_carriers"`
	OverallHealth     float64 `json:"overall_health"`
}

// GetPerformanceAnalytics returns performance analytics
func (h *SelectionHandler) GetPerformanceAnalytics(w http.ResponseWriter, r *http.Request) {
	carriers := h.manager.GetCarrierStatus()

	analytics := &PerformanceAnalytics{
		CarrierPerformance: make(map[string]*PerformanceMetrics),
		SystemHealth: &SystemHealth{
			HealthyCarriers:   0,
			DegradedCarriers:  0,
			UnhealthyCarriers: 0,
			OverallHealth:     0,
		},
		Recommendations: []string{},
	}

	for carrierID, carrier := range carriers {
		metrics := &PerformanceMetrics{
			CarrierID:      carrierID,
			ResponseTime:   float64(carrier.Metrics.AverageResponseTime.Milliseconds()),
			SuccessRate:    0,
			RequestRate:    carrier.Metrics.RequestRate,
			HealthScore:    0,
			Recommendation: "",
		}

		// Calculate success rate
		if carrier.Metrics.TotalRequests > 0 {
			metrics.SuccessRate = float64(carrier.Metrics.SuccessfulRequests) / float64(carrier.Metrics.TotalRequests) * 100
		}

		// Calculate health score
		switch carrier.HealthStatus {
		case "healthy":
			analytics.SystemHealth.HealthyCarriers++
			metrics.HealthScore = 100
		case "degraded":
			analytics.SystemHealth.DegradedCarriers++
			metrics.HealthScore = 50
		default:
			analytics.SystemHealth.UnhealthyCarriers++
			metrics.HealthScore = 0
		}

		// Generate recommendations
		if metrics.SuccessRate < 95 {
			metrics.Recommendation = "Monitor success rate - below optimal threshold"
		} else if metrics.ResponseTime > 500 {
			metrics.Recommendation = "Consider optimizing response time"
		} else if metrics.HealthScore < 50 {
			metrics.Recommendation = "Carrier health degraded - investigate issues"
		} else {
			metrics.Recommendation = "Carrier performing well"
		}

		analytics.CarrierPerformance[carrierID] = metrics
	}

	// Calculate overall health
	totalCarriers := len(carriers)
	if totalCarriers > 0 {
		analytics.SystemHealth.OverallHealth = float64(analytics.SystemHealth.HealthyCarriers) / float64(totalCarriers) * 100
	}

	// Generate system recommendations
	if analytics.SystemHealth.OverallHealth < 80 {
		analytics.Recommendations = append(analytics.Recommendations, "System health below optimal - investigate carrier issues")
	}
	if analytics.SystemHealth.UnhealthyCarriers > 0 {
		analytics.Recommendations = append(analytics.Recommendations, "Some carriers are unhealthy - consider failover")
	}

	response := PerformanceAnalyticsResponse{
		Success:   true,
		Analytics: analytics,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

func (h *SelectionHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *SelectionHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}{
		Success: false,
		Error:   message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
