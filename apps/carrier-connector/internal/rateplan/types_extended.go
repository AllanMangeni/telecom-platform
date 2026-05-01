package rateplan

import (
	"time"
)

// SubscriptionFilter defines filtering options for subscription queries
type SubscriptionFilter struct {
	Status       SubscriptionStatus `json:"status,omitempty"`
	RatePlanID   string             `json:"rate_plan_id,omitempty"`
	StartedAfter *time.Time         `json:"started_after,omitempty"`
	StartedBefore *time.Time        `json:"started_before,omitempty"`
	Limit        int                `json:"limit,omitempty"`
	Offset       int                `json:"offset,omitempty"`
}

// UsageAnalyticsFilter defines filtering options for usage analytics
type UsageAnalyticsFilter struct {
	RatePlanID   string     `json:"rate_plan_id,omitempty"`
	CarrierID    string     `json:"carrier_id,omitempty"`
	Region       string     `json:"region,omitempty"`
	StartDate    time.Time  `json:"start_date"`
	EndDate      time.Time  `json:"end_date"`
	GroupBy      string     `json:"group_by,omitempty"` // "day", "week", "month"
}

// RevenueAnalyticsFilter defines filtering options for revenue analytics
type RevenueAnalyticsFilter struct {
	RatePlanID   string     `json:"rate_plan_id,omitempty"`
	CarrierID    string     `json:"carrier_id,omitempty"`
	Region       string     `json:"region,omitempty"`
	StartDate    time.Time  `json:"start_date"`
	EndDate      time.Time  `json:"end_date"`
	GroupBy      string     `json:"group_by,omitempty"` // "day", "week", "month"
}

// UsageAnalytics contains usage statistics
type UsageAnalytics struct {
	TotalDataUsed    int64                     `json:"total_data_used"`
	TotalVoiceUsed   int64                     `json:"total_voice_used"`
	TotalSMSUsed     int64                     `json:"total_sms_used"`
	ActiveUsers      int                       `json:"active_users"`
	AverageUsage     map[string]float64        `json:"average_usage"`
	UsageByPlan      map[string]int64          `json:"usage_by_plan"`
	UsageByRegion    map[string]int64          `json:"usage_by_region"`
	TimelineData     []TimelineDataPoint       `json:"timeline_data"`
}

// RevenueAnalytics contains revenue statistics
type RevenueAnalytics struct {
	TotalRevenue     float64                   `json:"total_revenue"`
	RevenueByPlan    map[string]float64        `json:"revenue_by_plan"`
	RevenueByCarrier map[string]float64        `json:"revenue_by_carrier"`
	RevenueByRegion  map[string]float64        `json:"revenue_by_region"`
	AverageRevenue   map[string]float64        `json:"average_revenue"`
	TimelineData     []TimelineDataPoint       `json:"timeline_data"`
}

// TimelineDataPoint represents a data point in time series
type TimelineDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}
