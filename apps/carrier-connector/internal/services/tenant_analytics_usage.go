package services

import (
	"time"

	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/tenant"
)

// buildUsageByType builds usage analytics by resource type
func (s *TenantServiceImpl) buildUsageByType(usageStats *tenant.TenantUsageStats) map[string]*tenant.ResourceUsageAnalytics {
	usageByType := make(map[string]*tenant.ResourceUsageAnalytics)

	// Build analytics for each resource type
	resourceTypes := []string{"users", "profiles", "carriers", "api_calls", "storage"}

	for _, resourceType := range resourceTypes {
		analytics := &tenant.ResourceUsageAnalytics{
			ResourceType: resourceType,
			TotalUsage:   s.getMockResourceCount(resourceType),
			AverageUsage: s.getMockResourceCount(resourceType) / 30,
			PeakUsage:    s.getMockResourceCount(resourceType),
			PeakTime:     time.Now().Add(-2 * time.Hour),
		}

		usageByType[resourceType] = analytics
	}

	return usageByType
}

// buildUsageTrends builds usage trends over time
func (s *TenantServiceImpl) buildUsageTrends(tenantID, timeRange string) map[string][]*tenant.UsageTrend {
	trends := make(map[string][]*tenant.UsageTrend)

	// Build trends for each resource type
	resourceTypes := []string{"users", "profiles", "carriers", "api_calls", "storage"}

	for _, resourceType := range resourceTypes {
		trendData := make([]*tenant.UsageTrend, 0)

		// Generate mock trend data points
		now := time.Now()
		for i := 6; i >= 0; i-- {
			timestamp := now.AddDate(0, 0, -i)
			usage := int(float64(s.getMockResourceCount(resourceType)) * (1.0 + float64(6-i)*0.1))

			trend := &tenant.UsageTrend{
				Timestamp: timestamp,
				Usage:     usage,
			}

			trendData = append(trendData, trend)
		}

		trends[resourceType] = trendData
	}

	return trends
}

// buildUsagePeaks builds usage peak information
func (s *TenantServiceImpl) buildUsagePeaks(tenantID, timeRange string) map[string]*tenant.UsagePeak {
	peaks := make(map[string]*tenant.UsagePeak)

	// Build peaks for each resource type
	resourceTypes := []string{"users", "profiles", "carriers", "api_calls", "storage"}

	for _, resourceType := range resourceTypes {
		peak := &tenant.UsagePeak{
			Timestamp: time.Now().Add(-2 * time.Hour),
			Usage:     s.getMockResourceCount(resourceType),
			Context: map[string]any{
				"peak_hour":   14, // 2 PM
				"day_of_week": "Wednesday",
				"season":      "Q2",
				"driver":      "business_activity",
			},
		}

		peaks[resourceType] = peak
	}

	return peaks
}

// parseAPIRequestEvents parses API request events from tenant events
func (s *TenantServiceImpl) parseAPIRequestEvents(events []*tenant.TenantEvent) []*tenant.APIRequestEvent {
	apiRequests := make([]*tenant.APIRequestEvent, 0)

	for _, event := range events {
		if event.EventType == "api_request" {
			request := s.parseAPIRequestEvent(event)
			apiRequests = append(apiRequests, request)
		}
	}

	return apiRequests
}

// buildResourcePerformance builds resource performance metrics
func (s *TenantServiceImpl) buildResourcePerformance(tenantID, timeRange string) map[string]*tenant.ResourcePerformance {
	resourcePerformance := make(map[string]*tenant.ResourcePerformance)

	// Build performance for each resource type
	resourceTypes := []string{"users", "profiles", "carriers", "api_calls", "storage"}

	for _, resourceType := range resourceTypes {
		performance := &tenant.ResourcePerformance{
			ResourceType: resourceType,
			ResponseTime: 150.5,  // Mock response time in ms
			Throughput:   1000.0, // Mock requests per second
			ErrorRate:    2.1,    // Mock error rate percentage
		}

		resourcePerformance[resourceType] = performance
	}

	return resourcePerformance
}

// parseErrorEvents parses error events from tenant events
func (s *TenantServiceImpl) parseErrorEvents(events []*tenant.TenantEvent) []*tenant.ErrorEvent {
	errorEvents := make([]*tenant.ErrorEvent, 0)

	for _, event := range events {
		if event.EventType == "error" {
			errorEvent := s.parseErrorEvent(event)
			errorEvents = append(errorEvents, errorEvent)
		}
	}

	return errorEvents
}

// parseSlowQueryEvents parses slow query events from tenant events
func (s *TenantServiceImpl) parseSlowQueryEvents(events []*tenant.TenantEvent) []*tenant.SlowQuery {
	slowQueries := make([]*tenant.SlowQuery, 0)

	for _, event := range events {
		if event.EventType == "slow_query" {
			slowQuery := s.parseSlowQueryEvent(event)
			slowQueries = append(slowQueries, slowQuery)
		}
	}

	return slowQueries
}
