package services

import (
	"context"
	"fmt"
	"time"

	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/tenant"
)

// GetTenantMetrics retrieves tenant metrics
func (s *TenantServiceImpl) GetTenantMetrics(ctx context.Context, tenantID string) (*tenant.TenantMetrics, error) {
	// Get usage stats
	usageStats, err := s.GetUsageStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Get recent events
	events, err := s.repository.ListEvents(ctx, tenantID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant events: %w", err)
	}

	// Calculate metrics
	metrics := &tenant.TenantMetrics{
		TenantID:    tenantID,
		ActiveUsers: usageStats.ActiveUsers,
		StorageUsed: 0, // Would be calculated from actual storage usage
		HealthScore: 100.0,
		Alerts:      []string{},
	}

	// Calculate last activity
	if len(events) > 0 {
		metrics.LastActivity = events[0].Timestamp
	}

	// Calculate error rate and response time from events
	errorCount := 0
	totalRequests := 0
	var totalResponseTime time.Duration

	for _, event := range events {
		if event.EventType == "api_request" {
			totalRequests++
			if statusCode, exists := event.EventData["status_code"]; exists {
				if code, ok := statusCode.(float64); ok && code >= 400 {
					errorCount++
				}
			}
			if responseTime, exists := event.EventData["response_time"]; exists {
				if rt, ok := responseTime.(float64); ok {
					totalResponseTime += time.Duration(rt) * time.Millisecond
				}
			}
		}
	}

	if totalRequests > 0 {
		metrics.ErrorRate = float64(errorCount) / float64(totalRequests) * 100
		metrics.ResponseTime = float64(totalResponseTime) / float64(totalRequests) / float64(time.Millisecond)
	}

	// Check for alerts
	for resourceType, quotaStatus := range usageStats.QuotaStatus {
		if quotaStatus.Critical {
			metrics.Alerts = append(metrics.Alerts, fmt.Sprintf("Critical: %s quota at %.1f%%", resourceType, quotaStatus.Percent))
			metrics.HealthScore -= 20
		} else if quotaStatus.Warning {
			metrics.Alerts = append(metrics.Alerts, fmt.Sprintf("Warning: %s quota at %.1f%%", resourceType, quotaStatus.Percent))
			metrics.HealthScore -= 10
		}
	}

	return metrics, nil
}

// GetTenantEvents retrieves tenant events
func (s *TenantServiceImpl) GetTenantEvents(ctx context.Context, tenantID string, limit int) ([]*tenant.TenantEvent, error) {
	events, err := s.repository.ListEvents(ctx, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant events: %w", err)
	}

	return events, nil
}

// LogTenantEvent logs a tenant event
func (s *TenantServiceImpl) LogTenantEvent(ctx context.Context, event *tenant.TenantEvent) error {
	if err := s.repository.CreateEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create tenant event: %w", err)
	}

	return nil
}

// GetTenantDashboard returns dashboard data for a tenant
func (s *TenantServiceImpl) GetTenantDashboard(ctx context.Context, tenantID string) (*tenant.TenantDashboard, error) {
	// Get usage statistics
	usageStats, err := s.GetUsageStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Get tenant metrics
	metrics, err := s.GetTenantMetrics(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant metrics: %w", err)
	}

	// Get recent events
	recentEvents, err := s.repository.ListEvents(ctx, tenantID, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent events: %w", err)
	}

	// Build quota status from usage stats
	quotaStatus := s.buildQuotaStatus(usageStats)

	// Build comprehensive dashboard
	dashboard := &tenant.TenantDashboard{
		TenantID:     tenantID,
		UsageStats:   usageStats,
		Metrics:      metrics,
		RecentEvents: recentEvents,
		QuotaStatus:  quotaStatus,
		LastUpdated:  time.Now(),
	}

	return dashboard, nil
}

// GetUsageAnalytics returns detailed usage analytics for a tenant
func (s *TenantServiceImpl) GetUsageAnalytics(ctx context.Context, tenantID string, timeRange string) (*tenant.TenantUsageAnalytics, error) {
	startDate, endDate := s.parseTimeRange(timeRange)

	// Get usage statistics
	usageStats, err := s.GetUsageStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Build comprehensive usage analytics
	analytics := &tenant.TenantUsageAnalytics{
		TenantID:    tenantID,
		TimeRange:   timeRange,
		StartDate:   startDate,
		EndDate:     endDate,
		UsageByType: s.buildUsageByType(usageStats),
		Trends:      s.buildUsageTrends(tenantID, timeRange),
		Peaks:       s.buildUsagePeaks(tenantID, timeRange),
	}

	return analytics, nil
}

// GetPerformanceAnalytics returns performance analytics for a tenant
func (s *TenantServiceImpl) GetPerformanceAnalytics(ctx context.Context, tenantID string, timeRange string) (*tenant.TenantPerformanceAnalytics, error) {
	startDate, endDate := s.parseTimeRange(timeRange)

	// Get tenant events for performance analysis
	events, err := s.repository.ListEvents(ctx, tenantID, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant events: %w", err)
	}

	// Parse API request events
	apiRequests := s.parseAPIRequestEvents(events)

	// Build comprehensive performance analytics
	analytics := &tenant.TenantPerformanceAnalytics{
		TenantID:            tenantID,
		TimeRange:           timeRange,
		StartDate:           startDate,
		EndDate:             endDate,
		APIPerformance:      s.calculateAPIPerformance(apiRequests),
		ResourcePerformance: s.buildResourcePerformance(tenantID, timeRange),
		Errors:              s.parseErrorEvents(events),
		SlowQueries:         s.parseSlowQueryEvents(events),
	}

	return analytics, nil
}

// generateUsageID generates a usage ID
func generateUsageID(resourceType, tenantID string) string {
	return fmt.Sprintf("usage_%s_%s", resourceType, tenantID)
}