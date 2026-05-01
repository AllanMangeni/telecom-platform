package rateplan

import (
	"context"
	"fmt"
	"time"
)

// PricingEngine handles rate plan pricing calculations and validations
type PricingEngine struct {
	repo   Repository
	logger Logger
}

// Logger interface for pricing engine
type Logger interface {
	WithError(err error) Logger
	WithField(key string, value interface{}) Logger
	Error(msg string)
	Info(msg string)
	Warning(msg string)
}

// NewPricingEngine creates a new pricing engine
func NewPricingEngine(repo Repository, logger Logger) *PricingEngine {
	return &PricingEngine{
		repo:   repo,
		logger: logger,
	}
}

// ValidateRatePlan performs comprehensive validation of a rate plan
func (pe *PricingEngine) ValidateRatePlan(ctx context.Context, plan *RatePlan) error {
	// Basic field validation
	if err := pe.validateBasicFields(plan); err != nil {
		return err
	}

	// Validate dates
	if err := pe.validateDates(plan); err != nil {
		return err
	}

	// Validate allowances
	if err := pe.validateAllowances(plan); err != nil {
		return err
	}

	// Validate overage rates
	if err := pe.validateOverageRates(plan); err != nil {
		return err
	}

	// Validate discounts
	if err := pe.validateDiscounts(plan); err != nil {
		return err
	}

	// Validate early termination
	if err := pe.validateEarlyTermination(plan); err != nil {
		return err
	}

	return nil
}

// CalculateOptimalPrice calculates the optimal price for a rate plan based on market conditions
func (pe *PricingEngine) CalculateOptimalPrice(ctx context.Context, plan *RatePlan) (*PriceOptimization, error) {
	// Get similar plans in the same region
	filter := &RatePlanFilter{
		Region:    plan.Region,
		PlanType:  plan.PlanType,
		Status:    PlanStatusActive,
		IsActive:  &[]bool{true}[0],
		Limit:     10,
	}

	similarPlans, err := pe.repo.ListRatePlans(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get similar plans: %w", err)
	}

	// Calculate market average
	var totalBasePrice float64
	for _, similarPlan := range similarPlans {
		totalBasePrice += similarPlan.BasePrice
	}

	var marketAverage float64
	if len(similarPlans) > 0 {
		marketAverage = totalBasePrice / float64(len(similarPlans))
	}

	// Calculate recommended price
	recommendedPrice := pe.calculateRecommendedPrice(plan, marketAverage)

	optimization := &PriceOptimization{
		CurrentPrice:     plan.BasePrice,
		MarketAverage:    marketAverage,
		RecommendedPrice: recommendedPrice,
		PriceDifference:  recommendedPrice - plan.BasePrice,
		CompetitorCount:  len(similarPlans),
		OptimizedAt:      time.Now(),
	}

	return optimization, nil
}

// ValidateSubscription validates a subscription request
func (pe *PricingEngine) ValidateSubscription(ctx context.Context, req *SubscribeRequest) error {
	// Get the rate plan
	plan, err := pe.repo.GetRatePlan(ctx, req.RatePlanID)
	if err != nil {
		return fmt.Errorf("rate plan not found: %w", err)
	}

	// Check if plan is available for subscription
	if !plan.IsActive || plan.Status != PlanStatusActive {
		return fmt.Errorf("rate plan is not available for subscription")
	}

	// Check validity dates
	now := time.Now()
	if now.Before(plan.ValidFrom) {
		return fmt.Errorf("rate plan is not yet available")
	}

	if plan.ValidTo != nil && now.After(*plan.ValidTo) {
		return fmt.Errorf("rate plan has expired")
	}

	// Validate discounts
	if len(req.AppliedDiscounts) > 0 {
		if err := pe.validateSubscriptionDiscounts(plan, req.AppliedDiscounts); err != nil {
			return err
		}
	}

	return nil
}

// CalculateSubscriptionCost calculates the total cost for a subscription
func (pe *PricingEngine) CalculateSubscriptionCost(ctx context.Context, req *CalculateCostRequest) (*CostBreakdown, error) {
	// Get the rate plan
	plan, err := pe.repo.GetRatePlan(ctx, req.RatePlanID)
	if err != nil {
		return nil, fmt.Errorf("rate plan not found: %w", err)
	}

	breakdown := &CostBreakdown{
		RatePlanID:     req.RatePlanID,
		Currency:       plan.Currency,
		CalculatedAt:   time.Now(),
		BreakdownItems: []CostItem{},
	}

	// Base cost
	baseCost := plan.BasePrice
	breakdown.BreakdownItems = append(breakdown.BreakdownItems, CostItem{
		Type:        "base_price",
		Description: "Base subscription cost",
		Amount:      baseCost,
		Currency:    plan.Currency,
	})

	// Calculate data overage
	dataOverageCost := pe.calculateDataOverage(plan, req.DataUsed)
	if dataOverageCost > 0 {
		breakdown.BreakdownItems = append(breakdown.BreakdownItems, CostItem{
			Type:        "data_overage",
			Description: "Data usage over allowance",
			Amount:      dataOverageCost,
			Currency:    plan.Currency,
		})
	}

	// Calculate voice overage
	voiceOverageCost := pe.calculateVoiceOverage(plan, req.VoiceUsed)
	if voiceOverageCost > 0 {
		breakdown.BreakdownItems = append(breakdown.BreakdownItems, CostItem{
			Type:        "voice_overage",
			Description: "Voice usage over allowance",
			Amount:      voiceOverageCost,
			Currency:    plan.Currency,
		})
	}

	// Calculate SMS overage
	smsOverageCost := pe.calculateSMSOverage(plan, req.SMSUsed)
	if smsOverageCost > 0 {
		breakdown.BreakdownItems = append(breakdown.BreakdownItems, CostItem{
			Type:        "sms_overage",
			Description: "SMS usage over allowance",
			Amount:      smsOverageCost,
			Currency:    plan.Currency,
		})
	}

	// Apply discounts
	discountAmount := pe.calculateDiscounts(plan, req.AppliedDiscounts, baseCost)
	if discountAmount > 0 {
		breakdown.BreakdownItems = append(breakdown.BreakdownItems, CostItem{
			Type:        "discount",
			Description: "Applied discounts",
			Amount:      -discountAmount,
			Currency:    plan.Currency,
		})
	}

	// Calculate total
	totalCost := baseCost + dataOverageCost + voiceOverageCost + smsOverageCost - discountAmount
	breakdown.TotalCost = totalCost
	breakdown.Subtotal = baseCost + dataOverageCost + voiceOverageCost + smsOverageCost
	breakdown.DiscountTotal = discountAmount

	return breakdown, nil
}

// Helper methods

func (pe *PricingEngine) validateBasicFields(plan *RatePlan) error {
	if plan.Name == "" {
		return fmt.Errorf("rate plan name is required")
	}
	if plan.CarrierID == "" {
		return fmt.Errorf("carrier ID is required")
	}
	if plan.Region == "" {
		return fmt.Errorf("region is required")
	}
	if plan.BasePrice < 0 {
		return fmt.Errorf("base price cannot be negative")
	}
	if plan.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if plan.BillingCycle == "" {
		return fmt.Errorf("billing cycle is required")
	}
	return nil
}

func (pe *PricingEngine) validateDates(plan *RatePlan) error {
	if plan.ValidFrom.IsZero() {
		return fmt.Errorf("valid from date is required")
	}

	now := time.Now()
	if plan.ValidFrom.After(now) {
		pe.logger.Warning("Rate plan valid from date is in the future")
	}

	if plan.ValidTo != nil && plan.ValidTo.Before(plan.ValidFrom) {
		return fmt.Errorf("valid to date cannot be before valid from date")
	}

	return nil
}

func (pe *PricingEngine) validateAllowances(plan *RatePlan) error {
	// Validate data allowance
	if plan.DataAllowance != nil {
		if plan.DataAllowance.Amount <= 0 && !plan.DataAllowance.Unlimited {
			return fmt.Errorf("data allowance amount must be positive or unlimited")
		}
		if plan.DataAllowance.Unit == "" {
			return fmt.Errorf("data allowance unit is required")
		}
	}

	// Validate voice allowance
	if plan.VoiceAllowance != nil {
		if plan.VoiceAllowance.Minutes <= 0 && !plan.VoiceAllowance.Unlimited {
			return fmt.Errorf("voice allowance minutes must be positive or unlimited")
		}
	}

	// Validate SMS allowance
	if plan.SMSAllowance != nil {
		if plan.SMSAllowance.Messages <= 0 && !plan.SMSAllowance.Unlimited {
			return fmt.Errorf("SMS allowance messages must be positive or unlimited")
		}
	}

	return nil
}

func (pe *PricingEngine) validateOverageRates(plan *RatePlan) error {
	if plan.OverageRates != nil {
		if plan.OverageRates.DataRate < 0 {
			return fmt.Errorf("data overage rate cannot be negative")
		}
		if plan.OverageRates.VoiceRate < 0 {
			return fmt.Errorf("voice overage rate cannot be negative")
		}
		if plan.OverageRates.SMSRate < 0 {
			return fmt.Errorf("SMS overage rate cannot be negative")
		}
		if plan.OverageRates.Currency == "" {
			return fmt.Errorf("overage rates currency is required")
		}
	}
	return nil
}

func (pe *PricingEngine) validateDiscounts(plan *RatePlan) error {
	if plan.Discounts != nil {
		for _, discount := range plan.Discounts {
			if discount.Name == "" {
				return fmt.Errorf("discount name is required")
			}
			if discount.Value <= 0 {
				return fmt.Errorf("discount value must be positive")
			}
			if discount.ValidFrom.IsZero() {
				return fmt.Errorf("discount valid from date is required")
			}
			if discount.ValidTo != nil && discount.ValidTo.Before(discount.ValidFrom) {
				return fmt.Errorf("discount valid to date cannot be before valid from date")
			}
		}
	}
	return nil
}

func (pe *PricingEngine) validateEarlyTermination(plan *RatePlan) error {
	if plan.EarlyTermination != nil && plan.EarlyTermination.Enabled {
		if plan.EarlyTermination.FeeType == "" {
			return fmt.Errorf("early termination fee type is required")
		}
		if plan.EarlyTermination.FeeType == "fixed" && plan.EarlyTermination.FeeAmount <= 0 {
			return fmt.Errorf("early termination fee amount must be positive for fixed fee type")
		}
		if plan.EarlyTermination.FeeType == "percentage" && (plan.EarlyTermination.FeePercentage <= 0 || plan.EarlyTermination.FeePercentage > 100) {
			return fmt.Errorf("early termination fee percentage must be between 0 and 100")
		}
	}
	return nil
}

func (pe *PricingEngine) calculateRecommendedPrice(plan *RatePlan, marketAverage float64) float64 {
	// Basic pricing strategy: position slightly below market average for competitive advantage
	if marketAverage > 0 {
		return marketAverage * 0.95 // 5% below market average
	}
	return plan.BasePrice
}

func (pe *PricingEngine) validateSubscriptionDiscounts(plan *RatePlan, discountIDs []string) error {
	if plan.Discounts == nil {
		return fmt.Errorf("no discounts available for this rate plan")
	}

	for _, discountID := range discountIDs {
		found := false
		for _, discount := range plan.Discounts {
			if discount.ID == discountID {
				if !discount.IsActive {
					return fmt.Errorf("discount %s is not active", discountID)
				}
				now := time.Now()
				if now.Before(discount.ValidFrom) || (discount.ValidTo != nil && now.After(*discount.ValidTo)) {
					return fmt.Errorf("discount %s is not currently valid", discountID)
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("discount %s not found", discountID)
		}
	}

	return nil
}

func (pe *PricingEngine) calculateDataOverage(plan *RatePlan, dataUsed int64) float64 {
	if plan.DataAllowance == nil || plan.DataAllowance.Unlimited || plan.OverageRates == nil {
		return 0
	}

	allowanceMB := plan.DataAllowance.Amount
	if plan.DataAllowance.Unit == "GB" {
		allowanceMB *= 1024
	}

	if dataUsed <= allowanceMB {
		return 0
	}

	overageMB := dataUsed - allowanceMB
	return float64(overageMB) * plan.OverageRates.DataRate
}

func (pe *PricingEngine) calculateVoiceOverage(plan *RatePlan, voiceUsed int64) float64 {
	if plan.VoiceAllowance == nil || plan.VoiceAllowance.Unlimited || plan.OverageRates == nil {
		return 0
	}

	if voiceUsed <= plan.VoiceAllowance.Minutes {
		return 0
	}

	overageMinutes := voiceUsed - plan.VoiceAllowance.Minutes
	return float64(overageMinutes) * plan.OverageRates.VoiceRate
}

func (pe *PricingEngine) calculateSMSOverage(plan *RatePlan, smsUsed int64) float64 {
	if plan.SMSAllowance == nil || plan.SMSAllowance.Unlimited || plan.OverageRates == nil {
		return 0
	}

	if smsUsed <= plan.SMSAllowance.Messages {
		return 0
	}

	overageSMS := smsUsed - plan.SMSAllowance.Messages
	return float64(overageSMS) * plan.OverageRates.SMSRate
}

func (pe *PricingEngine) calculateDiscounts(plan *RatePlan, discountIDs []string, baseCost float64) float64 {
	if plan.Discounts == nil || len(discountIDs) == 0 {
		return 0
	}

	totalDiscount := 0.0
	for _, discountID := range discountIDs {
		for _, discount := range plan.Discounts {
			if discount.ID == discountID && discount.IsActive {
				if discount.Type == DiscountTypePercentage {
					totalDiscount += baseCost * discount.Value / 100
				} else if discount.Type == DiscountTypeFixed {
					totalDiscount += discount.Value
				}
			}
		}
	}

	return totalDiscount
}

// Supporting types

type PriceOptimization struct {
	CurrentPrice     float64   `json:"current_price"`
	MarketAverage    float64   `json:"market_average"`
	RecommendedPrice float64   `json:"recommended_price"`
	PriceDifference  float64   `json:"price_difference"`
	CompetitorCount  int       `json:"competitor_count"`
	OptimizedAt      time.Time `json:"optimized_at"`
}

type CostBreakdown struct {
	RatePlanID     string     `json:"rate_plan_id"`
	Currency       string     `json:"currency"`
	TotalCost      float64    `json:"total_cost"`
	Subtotal       float64    `json:"subtotal"`
	DiscountTotal  float64    `json:"discount_total"`
	BreakdownItems []CostItem `json:"breakdown_items"`
	CalculatedAt   time.Time  `json:"calculated_at"`
}

type CostItem struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
}
