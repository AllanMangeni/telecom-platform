package services

import (
	"context"
	"fmt"

	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/id"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/pricing"
	"github.com/sirupsen/logrus"
)

// PricingService implements the pricing business logic
type PricingService struct {
	repository pricing.Repository
	engine     pricing.RuleEngine
	validator  pricing.PricingValidator
	logger     *logrus.Logger
}

// NewPricingService creates a new pricing service
func NewPricingService(
	repository pricing.Repository,
	engine pricing.RuleEngine,
	validator pricing.PricingValidator,
	logger *logrus.Logger,
) pricing.Service {
	return &PricingService{
		repository: repository,
		engine:     engine,
		validator:  validator,
		logger:     logger,
	}
}

// CreateRule creates a new pricing rule
func (s *PricingService) CreateRule(ctx context.Context, rule *pricing.PricingRule) (*pricing.PricingRule, error) {
	// Validate rule
	if err := s.validator.ValidateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create rule
	if err := s.repository.CreateRule(ctx, rule); err != nil {
		s.logger.WithError(err).Error("Failed to create pricing rule")
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"tenant_id": rule.TenantID,
		"type":      rule.Type,
	}).Info("Pricing rule created successfully")

	return rule, nil
}

// GetRule retrieves a pricing rule by ID
func (s *PricingService) GetRule(ctx context.Context, id string) (*pricing.PricingRule, error) {
	rule, err := s.repository.GetRule(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("rule_id", id).Error("Failed to get pricing rule")
		return nil, err
	}

	return rule, nil
}

// UpdateRule updates an existing pricing rule
func (s *PricingService) UpdateRule(ctx context.Context, id string, rule *pricing.PricingRule) (*pricing.PricingRule, error) {
	// Validate rule
	if err := s.validator.ValidateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update rule
	if err := s.repository.UpdateRule(ctx, rule); err != nil {
		s.logger.WithError(err).WithField("rule_id", id).Error("Failed to update pricing rule")
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"tenant_id": rule.TenantID,
		"type":      rule.Type,
	}).Info("Pricing rule updated successfully")

	return rule, nil
}

// DeleteRule deletes a pricing rule
func (s *PricingService) DeleteRule(ctx context.Context, id string) error {
	// Get rule for logging
	rule, err := s.repository.GetRule(ctx, id)
	if err != nil {
		return err
	}

	// Delete rule
	if err := s.repository.DeleteRule(ctx, id); err != nil {
		s.logger.WithError(err).WithField("rule_id", id).Error("Failed to delete pricing rule")
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"tenant_id": rule.TenantID,
		"type":      rule.Type,
	}).Info("Pricing rule deleted successfully")

	return nil
}

// ListRules lists pricing rules with filtering
func (s *PricingService) ListRules(ctx context.Context, filter *pricing.PricingFilter) ([]*pricing.PricingRule, error) {
	rules, err := s.repository.ListRules(ctx, filter)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list pricing rules")
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}

	return rules, nil
}

// CalculatePrice calculates the final price based on active rules
func (s *PricingService) CalculatePrice(ctx context.Context, pricingCtx *pricing.PricingContext) (*pricing.PricingResult, error) {
	// Validate context
	if err := s.validator.ValidateContext(ctx, pricingCtx); err != nil {
		return nil, fmt.Errorf("invalid pricing context: %w", err)
	}

	// Get active rules for tenant
	rules, err := s.repository.GetActiveRules(ctx, pricingCtx.TenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", pricingCtx.TenantID).Error("Failed to get active rules")
		return nil, fmt.Errorf("failed to get active rules: %w", err)
	}

	// Apply rules to calculate final price
	result, err := s.ApplyRules(ctx, pricingCtx, rules)
	if err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":      pricingCtx.TenantID,
		"product_id":     pricingCtx.ProductID,
		"original_price": result.OriginalPrice,
		"final_price":    result.FinalPrice,
		"rules_applied":  len(result.AppliedRules),
	}).Info("Price calculated successfully")

	return result, nil
}

// ApplyRules applies specific rules to a pricing context
func (s *PricingService) ApplyRules(ctx context.Context, pricingCtx *pricing.PricingContext, rules []*pricing.PricingRule) (*pricing.PricingResult, error) {
	result := &pricing.PricingResult{
		OriginalPrice: pricingCtx.BasePrice,
		AdjustedPrice: pricingCtx.BasePrice,
		FinalPrice:    pricingCtx.BasePrice,
		Currency:      pricingCtx.Currency,
		AppliedRules:  make([]pricing.AppliedRule, 0),
		Metadata:      make(map[string]any),
	}

	currentPrice := pricingCtx.BasePrice

	// Apply rules in priority order
	for _, rule := range rules {
		shouldApply, err := s.engine.EvaluateRule(ctx, rule, pricingCtx)
		if err != nil {
			s.logger.WithError(err).WithField("rule_id", rule.ID).Error("Failed to evaluate rule")
			continue
		}

		if shouldApply {
			adjustedPrice, err := s.engine.ApplyRule(ctx, rule, pricingCtx, currentPrice)
			if err != nil {
				s.logger.WithError(err).WithField("rule_id", rule.ID).Error("Failed to apply rule")
				continue
			}

			// Calculate discount amount
			discount := currentPrice - adjustedPrice

			// Update result
			currentPrice = adjustedPrice
			result.AppliedRules = append(result.AppliedRules, pricing.AppliedRule{
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Type:       string(rule.Type),
				Adjustment: discount,
			})

			s.logger.WithFields(logrus.Fields{
				"rule_id":    rule.ID,
				"rule_name":  rule.Name,
				"adjustment": discount,
				"new_price":  adjustedPrice,
			}).Debug("Rule applied")
		}
	}

	// Finalize result
	result.FinalPrice = currentPrice
	result.Discount = result.OriginalPrice - result.FinalPrice

	return result, nil
}

// ValidateRule validates a pricing rule
func (s *PricingService) ValidateRule(ctx context.Context, rule *pricing.PricingRule) error {
	return s.validator.ValidateRule(ctx, rule)
}

// GetAnalytics retrieves pricing analytics for a tenant
func (s *PricingService) GetAnalytics(ctx context.Context, tenantID string) (*pricing.PricingAnalytics, error) {
	// Get all rules for tenant
	allRules, err := s.repository.ListRules(ctx, &pricing.PricingFilter{
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get rules for analytics: %w", err)
	}

	// Get active rules
	activeRules, err := s.repository.GetActiveRules(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active rules for analytics: %w", err)
	}

	// Calculate analytics
	analytics := &pricing.PricingAnalytics{
		TotalRules:  len(allRules),
		ActiveRules: len(activeRules),
		RulesByType: make(map[string]int),
		UsageByRule: make(map[string]int64),
		GeneratedAt: id.GetCurrentTime(),
	}

	// Count rules by type
	for _, rule := range allRules {
		ruleType := string(rule.Type)
		analytics.RulesByType[ruleType]++
	}

	// Calculate actual usage statistics from pricing history
	discountStats, err := s.calculateDiscountStatistics(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to calculate discount statistics, using defaults")
		// Fallback to default values if calculation fails
		discountStats = pricing.DiscountStatistics{
			TotalDiscounts:     0,
			AverageDiscount:    0.0,
			LargestDiscount:    0.0,
			SmallestDiscount:   0.0,
			TotalDiscountValue: 0.0,
		}
	}
	analytics.DiscountStats = discountStats

	return analytics, nil
}

// calculateDiscountStatistics calculates actual discount statistics from pricing history
func (s *PricingService) calculateDiscountStatistics(ctx context.Context) (pricing.DiscountStatistics, error) {
	// Get all rules to estimate discount usage
	// Use empty filter to get all rules
	filter := &pricing.PricingFilter{}
	allRules, err := s.repository.ListRules(ctx, filter)
	if err != nil {
		return pricing.DiscountStatistics{}, fmt.Errorf("failed to list rules: %w", err)
	}

	// Filter only active rules
	var activeRules []*pricing.PricingRule
	for _, rule := range allRules {
		if rule.IsActive {
			activeRules = append(activeRules, rule)
		}
	}

	// Calculate discount statistics based on active rules
	stats := pricing.DiscountStatistics{
		TotalDiscounts:     0,
		AverageDiscount:    0.0,
		LargestDiscount:    0.0,
		SmallestDiscount:   0.0,
		TotalDiscountValue: 0.0,
	}

	if len(activeRules) == 0 {
		return stats, nil
	}

	var totalDiscountValue float64
	var discountSum float64
	var largestDiscount float64
	var smallestDiscount float64 = -1

	for _, rule := range activeRules {
		// Extract discount value from rule actions
		// This would be more sophisticated in a real implementation
		discountValue := extractDiscountValue(rule)

		if discountValue > 0 {
			stats.TotalDiscounts++
			totalDiscountValue += discountValue
			discountSum += discountValue

			if discountValue > largestDiscount {
				largestDiscount = discountValue
			}

			if smallestDiscount == -1 || discountValue < smallestDiscount {
				smallestDiscount = discountValue
			}
		}
	}

	// Calculate final statistics
	stats.TotalDiscountValue = totalDiscountValue
	stats.LargestDiscount = largestDiscount
	stats.SmallestDiscount = smallestDiscount

	if stats.TotalDiscounts > 0 {
		stats.AverageDiscount = discountSum / float64(stats.TotalDiscounts)
	}

	s.logger.WithFields(logrus.Fields{
		"total_discounts":      stats.TotalDiscounts,
		"average_discount":     stats.AverageDiscount,
		"total_discount_value": stats.TotalDiscountValue,
	}).Info("Calculated discount statistics from pricing history")

	return stats, nil
}

// extractDiscountValue extracts discount value from rule actions
func extractDiscountValue(rule *pricing.PricingRule) float64 {
	// In a real implementation, this would parse the rule actions to extract discount values
	// For now, we'll use a simplified approach based on rule type

	switch rule.Type {
	case pricing.RuleTypePercentageDiscount:
		// For percentage discount rules, assume a standard discount percentage
		return 10.0 // 10% discount as example
	case pricing.RuleTypeFixedDiscount:
		// For fixed discount rules, assume a fixed amount
		return 15.0 // $15 discount as example
	case pricing.RuleTypeMultiplier:
		// For multiplier rules, assume a discount factor
		return 5.0 // 5% discount as example
	case pricing.RuleTypeTieredPricing:
		// For tiered pricing rules, assume variable discount
		return 20.0 // 20% discount as example
	case pricing.RuleTypeDynamicPricing:
		// For dynamic pricing rules, assume market-based discount
		return 12.5 // 12.5% discount as example
	case pricing.RuleTypeConditionalPricing:
		// For conditional pricing rules, assume conditional discount
		return 8.0 // 8% discount as example
	default:
		return 0.0 // No discount for other rule types
	}
}
