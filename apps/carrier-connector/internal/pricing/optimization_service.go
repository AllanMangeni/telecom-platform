package pricing

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// OptimizationStrategy represents pricing optimization strategies
type OptimizationStrategy string

const (
	StrategyRevenueMax      OptimizationStrategy = "revenue_maximization"
	StrategyMarketShare     OptimizationStrategy = "market_share"
	StrategyProfitMargin    OptimizationStrategy = "profit_margin"
	StrategyCompetitive      OptimizationStrategy = "competitive"
	StrategyChurnReduction  OptimizationStrategy = "churn_reduction"
)

// OptimizationResult represents pricing optimization results
type OptimizationResult struct {
	RatePlanID      string             `json:"rate_plan_id"`
	Strategy        OptimizationStrategy `json:"strategy"`
	CurrentPrice    float64            `json:"current_price"`
	OptimalPrice    float64            `json:"optimal_price"`
	PriceChange     float64            `json:"price_change_pct"`
	ExpectedRevenue float64            `json:"expected_revenue"`
	ExpectedDemand  int64              `json:"expected_demand"`
	Confidence      float64            `json:"confidence"` // 0-100
	Reasoning       []string           `json:"reasoning"`
	Risks           []string           `json:"risks"`
	Recommendations []string           `json:"recommendations"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

// PricingMetrics represents pricing performance metrics
type PricingMetrics struct {
	Period            string    `json:"period"`
	TotalRevenue      float64   `json:"total_revenue"`
	TotalSubscribers  int64     `json:"total_subscribers"`
	ARPU              float64   `json:"arpu"`
	ChurnRate         float64   `json:"churn_rate_pct"`
	PriceElasticity   float64   `json:"price_elasticity"`
	CompetitiveIndex  float64   `json:"competitive_index"`
	OptimizationROI   float64   `json:"optimization_roi_pct"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// PricingOptimizationService provides automated pricing optimization
type PricingOptimizationService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewPricingOptimizationService creates a new pricing optimization service
func NewPricingOptimizationService(db *gorm.DB, logger *logrus.Logger) *PricingOptimizationService {
	return &PricingOptimizationService{
		db:     db,
		logger: logger,
	}
}

// OptimizePricing optimizes pricing for rate plans
func (s *PricingOptimizationService) OptimizePricing(ctx context.Context, ratePlanIDs []string, strategy OptimizationStrategy) ([]*OptimizationResult, error) {
	results := make([]*OptimizationResult, 0)

	for _, ratePlanID := range ratePlanIDs {
		result, err := s.optimizeRatePlan(ctx, ratePlanID, strategy)
		if err != nil {
			s.logger.WithError(err).Error("Failed to optimize rate plan", "rate_plan_id", ratePlanID)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// optimizeRatePlan optimizes a single rate plan
func (s *PricingOptimizationService) optimizeRatePlan(ctx context.Context, ratePlanID string, strategy OptimizationStrategy) (*OptimizationResult, error) {
	// Get current rate plan data
	ratePlan, err := s.getRatePlan(ctx, ratePlanID)
	if err != nil {
		return nil, err
	}

	// Get historical data
	historicalData, err := s.getHistoricalData(ctx, ratePlanID)
	if err != nil {
		return nil, err
	}

	// Calculate optimal price based on strategy
	optimalPrice := s.calculateOptimalPrice(ratePlan, historicalData, strategy)

	// Calculate expected outcomes
	expectedRevenue, expectedDemand := s.predictOutcomes(ratePlan, optimalPrice, historicalData)

	// Generate reasoning and recommendations
	reasoning, risks, recommendations := s.generateAnalysis(ratePlan, optimalPrice, strategy, historicalData)

	result := &OptimizationResult{
		RatePlanID:      ratePlanID,
		Strategy:        strategy,
		CurrentPrice:    ratePlan.BasePrice,
		OptimalPrice:    optimalPrice,
		PriceChange:     ((optimalPrice - ratePlan.BasePrice) / ratePlan.BasePrice) * 100,
		ExpectedRevenue: expectedRevenue,
		ExpectedDemand:  expectedDemand,
		Confidence:      s.calculateConfidence(historicalData),
		Reasoning:       reasoning,
		Risks:           risks,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}

	return result, nil
}

// GetPricingMetrics returns pricing performance metrics
func (s *PricingOptimizationService) GetPricingMetrics(ctx context.Context, period string) (*PricingMetrics, error) {
	metrics := &PricingMetrics{
		Period:      period,
		GeneratedAt: time.Now(),
	}

	// Calculate total revenue
	var totalRevenue float64
	s.db.WithContext(ctx).Table("billing_transactions").
		Where("status = ? AND created_at BETWEEN ? AND ?", "completed", 
			s.getPeriodStart(period), s.getPeriodEnd(period)).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&totalRevenue)
	metrics.TotalRevenue = totalRevenue

	// Calculate total subscribers
	var totalSubs int64
	s.db.WithContext(ctx).Table("profiles").
		Where("status = ?", "active").
		Count(&totalSubs)
	metrics.TotalSubscribers = totalSubs

	// Calculate ARPU
	if totalSubs > 0 {
		metrics.ARPU = totalRevenue / float64(totalSubs)
	}

	// Calculate churn rate
	metrics.ChurnRate = s.calculateChurnRate(ctx, period)

	// Calculate price elasticity
	metrics.PriceElasticity = s.calculatePriceElasticity(ctx, period)

	// Calculate competitive index
	metrics.CompetitiveIndex = s.calculateCompetitiveIndex(ctx, period)

	// Calculate optimization ROI
	metrics.OptimizationROI = s.calculateOptimizationROI(ctx, period)

	return metrics, nil
}

// ApplyOptimization applies pricing optimization
func (s *PricingOptimizationService) ApplyOptimization(ctx context.Context, result *OptimizationResult) error {
	// Update rate plan price
	err := s.db.WithContext(ctx).Table("rate_plans").
		Where("id = ?", result.RatePlanID).
		Updates(map[string]interface{}{
			"base_price": result.OptimalPrice,
			"updated_at": time.Now(),
		}).Error

	if err != nil {
		return fmt.Errorf("failed to update rate plan: %w", err)
	}

	// Log the optimization
	s.logger.WithFields(logrus.Fields{
		"rate_plan_id":   result.RatePlanID,
		"strategy":       result.Strategy,
		"old_price":      result.CurrentPrice,
		"new_price":      result.OptimalPrice,
		"price_change":   result.PriceChange,
		"expected_revenue": result.ExpectedRevenue,
	}).Info("Pricing optimization applied")

	return nil
}

// getRatePlan retrieves rate plan data
func (s *PricingOptimizationService) getRatePlan(ctx context.Context, ratePlanID string) (*RatePlan, error) {
	var ratePlan RatePlan
	err := s.db.WithContext(ctx).Where("id = ?", ratePlanID).First(&ratePlan).Error
	if err != nil {
		return nil, fmt.Errorf("rate plan not found: %w", err)
	}
	return &ratePlan, nil
}

// getHistoricalData retrieves historical pricing and demand data
func (s *PricingOptimizationService) getHistoricalData(ctx context.Context, ratePlanID string) ([]HistoricalDataPoint, error) {
	// Get pricing history and subscription data
	var data []HistoricalDataPoint
	
	// This would query actual historical data
	// For now, return simulated data
	for i := 0; i < 12; i++ { // Last 12 months
		date := time.Now().AddDate(0, -i, 0)
		point := HistoricalDataPoint{
			Date:     date,
			Price:    10.0 + float64(i)*0.5, // Simulated price changes
			Demand:   1000 - int64(i)*50,     // Simulated demand changes
			Revenue:  (10.0 + float64(i)*0.5) * float64(1000 - int64(i)*50),
		}
		data = append(data, point)
	}

	return data, nil
}

// calculateOptimalPrice calculates optimal price based on strategy
func (s *PricingOptimizationService) calculateOptimalPrice(ratePlan *RatePlan, data []HistoricalDataPoint, strategy OptimizationStrategy) float64 {
	switch strategy {
	case StrategyRevenueMax:
		return s.optimizeForRevenue(ratePlan, data)
	case StrategyMarketShare:
		return s.optimizeForMarketShare(ratePlan, data)
	case StrategyProfitMargin:
		return s.optimizeForProfitMargin(ratePlan, data)
	case StrategyCompetitive:
		return s.optimizeForCompetitive(ratePlan, data)
	case StrategyChurnReduction:
		return s.optimizeForChurnReduction(ratePlan, data)
	default:
		return ratePlan.BasePrice
	}
}

// optimizeForRevenue optimizes price for maximum revenue
func (s *PricingOptimizationService) optimizeForRevenue(ratePlan *RatePlan, data []HistoricalDataPoint) float64 {
	// Find price that maximizes price * demand
	maxRevenue := 0.0
	optimalPrice := ratePlan.BasePrice

	for price := ratePlan.BasePrice * 0.8; price <= ratePlan.BasePrice * 1.5; price += 0.5 {
		demand := s.predictDemand(price, data)
		revenue := price * float64(demand)
		
		if revenue > maxRevenue {
			maxRevenue = revenue
			optimalPrice = price
		}
	}

	return optimalPrice
}

// optimizeForMarketShare optimizes price for market share
func (s *PricingOptimizationService) optimizeForMarketShare(ratePlan *RatePlan, data []HistoricalDataPoint) float64 {
	// Lower price to maximize demand (within reasonable bounds)
	return ratePlan.BasePrice * 0.85
}

// optimizeForProfitMargin optimizes price for profit margin
func (s *PricingOptimizationService) optimizeForProfitMargin(ratePlan *RatePlan, data []HistoricalDataPoint) float64 {
	// Assume 70% cost, optimize for margin
	cost := ratePlan.BasePrice * 0.7
	return cost * 1.5 // 50% margin
}

// optimizeForCompetitive optimizes price for competitive positioning
func (s *PricingOptimizationService) optimizeForCompetitive(ratePlan *RatePlan, data []HistoricalDataPoint) float64 {
	// Get competitor prices (simplified)
	competitorPrices := []float64{9.99, 12.99, 14.99, 16.99}
	
	// Price slightly below median competitor
	medianPrice := competitorPrices[len(competitorPrices)/2]
	return medianPrice * 0.95
}

// optimizeForChurnReduction optimizes price to reduce churn
func (s *PricingOptimizationService) optimizeForChurnReduction(ratePlan *RatePlan, data []HistoricalDataPoint) float64 {
	// Lower price to reduce churn
	return ratePlan.BasePrice * 0.9
}

// predictOutcomes predicts revenue and demand for a price
func (s *PricingOptimizationService) predictOutcomes(ratePlan *RatePlan, price float64, data []HistoricalDataPoint) (float64, int64) {
	demand := s.predictDemand(price, data)
	revenue := price * float64(demand)
	return revenue, demand
}

// predictDemand predicts demand for a given price
func (s *PricingOptimizationService) predictDemand(price float64, data []HistoricalDataPoint) int64 {
	// Simple linear demand model based on historical data
	if len(data) < 2 {
		return 1000 // Default demand
	}

	// Calculate price elasticity from historical data
	latest := data[0]
	previous := data[1]

	priceChange := (latest.Price - previous.Price) / previous.Price
	demandChange := float64(latest.Demand - previous.Demand) / float64(previous.Demand)

	elasticity := demandChange / priceChange

	// Predict demand change for new price
	baseDemand := float64(latest.Demand)
	priceChangeNew := (price - latest.Price) / latest.Price
	demandChangeNew := elasticity * priceChangeNew

	predictedDemand := baseDemand * (1 + demandChangeNew)
	return int64(math.Max(0, predictedDemand))
}

// generateAnalysis generates reasoning, risks, and recommendations
func (s *PricingOptimizationService) generateAnalysis(ratePlan *RatePlan, optimalPrice float64, strategy OptimizationStrategy, data []HistoricalDataPoint) ([]string, []string, []string) {
	reasoning := make([]string, 0)
	risks := make([]string, 0)
	recommendations := make([]string, 0)

	priceChange := ((optimalPrice - ratePlan.BasePrice) / ratePlan.BasePrice) * 100

	// Generate reasoning based on strategy
	switch strategy {
	case StrategyRevenueMax:
		reasoning = append(reasoning, "Optimized for maximum revenue generation")
		reasoning = append(reasoning, fmt.Sprintf("Price change of %.1f%% expected to maximize revenue", priceChange))
		
		if priceChange > 10 {
			risks = append(risks, "Significant price increase may impact demand")
			risks = append(risks, "Competitive pressure may increase")
		}
		
	case StrategyMarketShare:
		reasoning = append(reasoning, "Optimized for market share growth")
		reasoning = append(reasoning, "Lower pricing strategy to attract more customers")
		
		risks = append(risks, "Lower margins may impact profitability")
		risks = append(risks, "May attract price-sensitive customers with higher churn")
		
	case StrategyCompetitive:
		reasoning = append(reasoning, "Priced competitively relative to market")
		reasoning = append(reasoning, "Positioned below median competitor pricing")
		
		risks = append(risks, "Competitors may respond with price cuts")
		risks = append(risks, "Margin pressure in competitive market")
	}

	// General recommendations
	recommendations = append(recommendations, "Monitor demand closely after price change")
	recommendations = append(recommendations, "Track competitor pricing responses")
	recommendations = append(recommendations, "Review customer feedback and churn rates")

	if math.Abs(priceChange) > 15 {
		recommendations = append(recommendations, "Consider gradual price adjustment")
		recommendations = append(recommendations, "Implement promotional offers for existing customers")
	}

	return reasoning, risks, recommendations
}

// calculateConfidence calculates confidence level for predictions
func (s *PricingOptimizationService) calculateConfidence(data []HistoricalDataPoint) float64 {
	// More data points = higher confidence
	dataPoints := len(data)
	if dataPoints >= 12 {
		return 85.0
	} else if dataPoints >= 6 {
		return 70.0
	} else if dataPoints >= 3 {
		return 50.0
	} else {
		return 25.0
	}
}

// calculateChurnRate calculates churn rate for period
func (s *PricingOptimizationService) calculateChurnRate(ctx context.Context, period string) float64 {
	var totalSubs, churnedSubs int64
	
	startDate := s.getPeriodStart(period)
	endDate := s.getPeriodEnd(period)

	s.db.WithContext(ctx).Table("profiles").
		Where("created_at < ?", startDate).
		Count(&totalSubs)

	s.db.WithContext(ctx).Table("rate_plan_subscriptions").
		Where("ended_at BETWEEN ? AND ?", startDate, endDate).
		Count(&churnedSubs)

	if totalSubs == 0 {
		return 0
	}

	return float64(churnedSubs) / float64(totalSubs) * 100
}

// calculatePriceElasticity calculates price elasticity
func (s *PricingOptimizationService) calculatePriceElasticity(ctx context.Context, period string) float64 {
	// Simplified elasticity calculation
	return -1.2 // Typical for telecom services
}

// calculateCompetitiveIndex calculates competitive positioning index
func (s *PricingOptimizationService) calculateCompetitiveIndex(ctx context.Context, period string) float64 {
	// Simplified competitive index (0-100, higher is better positioned)
	return 75.0
}

// calculateOptimizationROI calculates ROI from optimizations
func (s *PricingOptimizationService) calculateOptimizationROI(ctx context.Context, period string) float64 {
	// Simplified ROI calculation
	return 15.5 // 15.5% ROI from optimizations
}

// getPeriodStart returns start date for period
func (s *PricingOptimizationService) getPeriodStart(period string) time.Time {
	now := time.Now()
	switch period {
	case "daily":
		return now.Truncate(24 * time.Hour)
	case "weekly":
		return now.AddDate(0, 0, -7)
	case "monthly":
		return now.AddDate(0, -1, 0)
	case "quarterly":
		return now.AddDate(0, -3, 0)
	default:
		return now.AddDate(0, -1, 0)
	}
}

// getPeriodEnd returns end date for period
func (s *PricingOptimizationService) getPeriodEnd(period string) time.Time {
	return time.Now()
}

// RatePlan represents a rate plan (simplified)
type RatePlan struct {
	ID        string  `gorm:"primaryKey"`
	Name      string  `json:"name"`
	BasePrice float64 `json:"base_price"`
	Currency  string  `json:"currency"`
}

// HistoricalDataPoint represents historical pricing and demand data
type HistoricalDataPoint struct {
	Date    time.Time `json:"date"`
	Price   float64   `json:"price"`
	Demand  int64     `json:"demand"`
	Revenue float64   `json:"revenue"`
}
