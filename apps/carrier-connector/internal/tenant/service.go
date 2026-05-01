package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// ServiceImpl implements the tenant service interface
type ServiceImpl struct {
	repository      Repository
	rateLimiter     RateLimiter
	eventPublisher  EventPublisher
	configManager   ConfigManager
	auditLogger     AuditLogger
	metricsCollector MetricsCollector
	logger          *logrus.Logger
}

// NewService creates a new tenant service
func NewService(
	repository Repository,
	rateLimiter RateLimiter,
	eventPublisher EventPublisher,
	configManager ConfigManager,
	auditLogger AuditLogger,
	metricsCollector MetricsCollector,
	logger *logrus.Logger,
) Service {
	return &ServiceImpl{
		repository:      repository,
		rateLimiter:     rateLimiter,
		eventPublisher:  eventPublisher,
		configManager:   configManager,
		auditLogger:     auditLogger,
		metricsCollector: metricsCollector,
		logger:          logger,
	}
}

// CreateTenant creates a new tenant
func (s *ServiceImpl) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
	// Validate request
	if err := s.validateCreateTenantRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if domain already exists
	existing, err := s.repository.GetTenantByDomain(ctx, req.Domain)
	if err == nil && existing != nil {
		return nil, errors.New("domain already exists")
	}

	// Create tenant
	tenant := &Tenant{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Domain:      req.Domain,
		Status:      TenantStatusActive,
		Plan:        req.Plan,
		MaxUsers:    req.MaxUsers,
		MaxProfiles: req.MaxProfiles,
		MaxCarriers: req.MaxCarriers,
		Settings:    req.Settings,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Set default settings if not provided
	if tenant.Settings == nil {
		tenant.Settings = s.getDefaultSettings(req.Plan)
	}

	// Save tenant
	if err := s.repository.CreateTenant(ctx, tenant); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant")
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// Create initial configuration
	config := &TenantConfig{
		TenantID: tenant.ID,
		Config:   make(map[string]interface{}),
		Settings: tenant.Settings,
		Quotas:   s.getDefaultQuotas(req.Plan),
		Features: s.getDefaultFeatures(req.Plan),
	}

	if err := s.repository.UpdateConfig(ctx, config); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant config")
	}

	// Publish tenant created event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		UserID:    "",
		EventType: TenantEventCreated,
		EventData: map[string]interface{}{
			"tenant_id": tenant.ID,
			"name":      tenant.Name,
			"domain":    tenant.Domain,
			"plan":      tenant.Plan,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenant.ID,
		"name":      tenant.Name,
		"domain":    tenant.Domain,
	}).Info("Tenant created successfully")

	return tenant, nil
}

// GetTenant retrieves a tenant by ID
func (s *ServiceImpl) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	tenant, err := s.repository.GetTenant(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", id).Error("Failed to get tenant")
		return nil, err
	}

	return tenant, nil
}

// GetTenantByDomain retrieves a tenant by domain
func (s *ServiceImpl) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	tenant, err := s.repository.GetTenantByDomain(ctx, domain)
	if err != nil {
		s.logger.WithError(err).WithField("domain", domain).Error("Failed to get tenant by domain")
		return nil, err
	}

	return tenant, nil
}

// UpdateTenant updates an existing tenant
func (s *ServiceImpl) UpdateTenant(ctx context.Context, id string, req *UpdateTenantRequest) (*Tenant, error) {
	// Get existing tenant
	tenant, err := s.repository.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Status != nil {
		tenant.Status = *req.Status
	}
	if req.Plan != nil {
		tenant.Plan = *req.Plan
	}
	if req.MaxUsers != nil {
		tenant.MaxUsers = *req.MaxUsers
	}
	if req.MaxProfiles != nil {
		tenant.MaxProfiles = *req.MaxProfiles
	}
	if req.MaxCarriers != nil {
		tenant.MaxCarriers = *req.MaxCarriers
	}
	if req.Settings != nil {
		tenant.Settings = req.Settings
	}
	if req.Metadata != nil {
		tenant.Metadata = req.Metadata
	}

	tenant.UpdatedAt = time.Now()

	// Save tenant
	if err := s.repository.UpdateTenant(ctx, tenant); err != nil {
		s.logger.WithError(err).Error("Failed to update tenant")
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	// Log tenant updated event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		UserID:    "",
		EventType: TenantEventUpdated,
		EventData: map[string]interface{}{
			"tenant_id": tenant.ID,
			"updates":   req,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithField("tenant_id", tenant.ID).Info("Tenant updated successfully")

	return tenant, nil
}

// DeleteTenant deletes a tenant
func (s *ServiceImpl) DeleteTenant(ctx context.Context, id string) error {
	// Get tenant for logging
	tenant, err := s.repository.GetTenant(ctx, id)
	if err != nil {
		return err
	}

	// Delete tenant
	if err := s.repository.DeleteTenant(ctx, id); err != nil {
		s.logger.WithError(err).Error("Failed to delete tenant")
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	// Log tenant deleted event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  id,
		UserID:    "",
		EventType: TenantEventDeleted,
		EventData: map[string]interface{}{
			"tenant_id": id,
			"name":      tenant.Name,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithField("tenant_id", id).Info("Tenant deleted successfully")

	return nil
}

// ListTenants lists tenants with filtering
func (s *ServiceImpl) ListTenants(ctx context.Context, filter *TenantFilter) ([]*Tenant, error) {
	tenants, err := s.repository.ListTenants(ctx, filter)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list tenants")
		return nil, err
	}

	return tenants, nil
}

// AddUserToTenant adds a user to a tenant
func (s *ServiceImpl) AddUserToTenant(ctx context.Context, req *CreateTenantUserRequest) (*TenantUser, error) {
	// Validate request
	if err := s.validateCreateTenantUserRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user already exists
	existing, err := s.repository.GetTenantUser(ctx, req.TenantID, req.UserID)
	if err == nil && existing != nil {
		return nil, errors.New("user already exists in tenant")
	}

	// Create tenant user
	tenantUser := &TenantUser{
		ID:        uuid.New().String(),
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		Email:     req.Email,
		Role:      req.Role,
		Status:    TenantUserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save tenant user
	if err := s.repository.CreateTenantUser(ctx, tenantUser); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant user")
		return nil, fmt.Errorf("failed to add user to tenant: %w", err)
	}

	// Log user added event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		EventType: TenantEventUserAdded,
		EventData: map[string]interface{}{
			"tenant_id": req.TenantID,
			"user_id":   req.UserID,
			"email":     req.Email,
			"role":      req.Role,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": req.TenantID,
		"user_id":   req.UserID,
		"role":      req.Role,
	}).Info("User added to tenant successfully")

	return tenantUser, nil
}

// GetTenantUser retrieves a tenant user
func (s *ServiceImpl) GetTenantUser(ctx context.Context, tenantID, userID string) (*TenantUser, error) {
	user, err := s.repository.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"user_id":   userID,
		}).Error("Failed to get tenant user")
		return nil, err
	}

	return user, nil
}

// UpdateTenantUser updates a tenant user
func (s *ServiceImpl) UpdateTenantUser(ctx context.Context, tenantID, userID string, req *UpdateTenantUserRequest) (*TenantUser, error) {
	// Get existing user
	user, err := s.repository.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	user.UpdatedAt = time.Now()

	// Save user
	if err := s.repository.UpdateTenantUser(ctx, user); err != nil {
		s.logger.WithError(err).Error("Failed to update tenant user")
		return nil, fmt.Errorf("failed to update tenant user: %w", err)
	}

	// Log user updated event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		EventType: TenantEventUserUpdated,
		EventData: map[string]interface{}{
			"tenant_id": tenantID,
			"user_id":   userID,
			"updates":   req,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Tenant user updated successfully")

	return user, nil
}

// RemoveUserFromTenant removes a user from a tenant
func (s *ServiceImpl) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	// Delete tenant user
	if err := s.repository.DeleteTenantUser(ctx, tenantID, userID); err != nil {
		s.logger.WithError(err).Error("Failed to remove user from tenant")
		return fmt.Errorf("failed to remove user from tenant: %w", err)
	}

	// Log user removed event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		EventType: TenantEventUserRemoved,
		EventData: map[string]interface{}{
			"tenant_id": tenantID,
			"user_id":   userID,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("User removed from tenant successfully")

	return nil
}

// ListTenantUsers lists tenant users with filtering
func (s *ServiceImpl) ListTenantUsers(ctx context.Context, filter *TenantUserFilter) ([]*TenantUser, error) {
	users, err := s.repository.ListTenantUsers(ctx, filter)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list tenant users")
		return nil, err
	}

	return users, nil
}

// CreateAPIKey creates a new API key for a tenant
func (s *ServiceImpl) CreateAPIKey(ctx context.Context, tenantID string, req *CreateAPIKeyRequest) (*TenantAPIKey, string, error) {
	// Validate request
	if err := s.validateCreateAPIKeyRequest(req); err != nil {
		return nil, "", fmt.Errorf("validation failed: %w", err)
	}

	// Generate API key
	apiKey := s.generateAPIKey()
	keyHash, err := s.hashAPIKey(apiKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash API key: %w", err)
	}

	// Create API key record
	apiKeyRecord := &TenantAPIKey{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        req.Name,
		KeyHash:     keyHash,
		KeyPrefix:   apiKey[:8],
		Permissions: req.Permissions,
		RateLimit:   req.RateLimit,
		ExpiresAt:   req.ExpiresAt,
		Status:      APIKeyStatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save API key
	if err := s.repository.CreateAPIKey(ctx, apiKeyRecord); err != nil {
		s.logger.WithError(err).Error("Failed to create API key")
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Log API key created event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    "",
		EventType: TenantEventAPIKeyCreated,
		EventData: map[string]interface{}{
			"tenant_id": tenantID,
			"key_id":    apiKeyRecord.ID,
			"name":      req.Name,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"key_id":    apiKeyRecord.ID,
		"name":      req.Name,
	}).Info("API key created successfully")

	return apiKeyRecord, apiKey, nil
}

// GetAPIKey retrieves an API key by ID
func (s *ServiceImpl) GetAPIKey(ctx context.Context, id string) (*TenantAPIKey, error) {
	apiKey, err := s.repository.GetAPIKey(ctx, id)
	if err != nil {
		s.logger.WithError(err).WithField("key_id", id).Error("Failed to get API key")
		return nil, err
	}

	return apiKey, nil
}

// UpdateAPIKey updates an API key
func (s *ServiceImpl) UpdateAPIKey(ctx context.Context, id string, req *UpdateAPIKeyRequest) (*TenantAPIKey, error) {
	// Get existing API key
	apiKey, err := s.repository.GetAPIKey(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		apiKey.Name = *req.Name
	}
	if req.Permissions != nil {
		apiKey.Permissions = req.Permissions
	}
	if req.RateLimit != nil {
		apiKey.RateLimit = *req.RateLimit
	}
	if req.ExpiresAt != nil {
		apiKey.ExpiresAt = req.ExpiresAt
	}
	if req.Status != nil {
		apiKey.Status = *req.Status
	}

	apiKey.UpdatedAt = time.Now()

	// Save API key
	if err := s.repository.UpdateAPIKey(ctx, apiKey); err != nil {
		s.logger.WithError(err).Error("Failed to update API key")
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	s.logger.WithField("key_id", id).Info("API key updated successfully")

	return apiKey, nil
}

// DeleteAPIKey deletes an API key
func (s *ServiceImpl) DeleteAPIKey(ctx context.Context, id string) error {
	// Get API key for logging
	apiKey, err := s.repository.GetAPIKey(ctx, id)
	if err != nil {
		return err
	}

	// Delete API key
	if err := s.repository.DeleteAPIKey(ctx, id); err != nil {
		s.logger.WithError(err).Error("Failed to delete API key")
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	// Log API key revoked event
	event := &TenantEvent{
		ID:        uuid.New().String(),
		TenantID:  apiKey.TenantID,
		UserID:    "",
		EventType: TenantEventAPIKeyRevoked,
		EventData: map[string]interface{}{
			"tenant_id": apiKey.TenantID,
			"key_id":    id,
			"name":      apiKey.Name,
		},
		Timestamp: time.Now(),
	}

	if err := s.repository.CreateEvent(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to create tenant event")
	}

	s.logger.WithField("key_id", id).Info("API key deleted successfully")

	return nil
}

// ListAPIKeys lists API keys for a tenant
func (s *ServiceImpl) ListAPIKeys(ctx context.Context, tenantID string) ([]*TenantAPIKey, error) {
	apiKeys, err := s.repository.ListAPIKeys(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to list API keys")
		return nil, err
	}

	return apiKeys, nil
}

// ValidateAPIKey validates an API key and returns the key record
func (s *ServiceImpl) ValidateAPIKey(ctx context.Context, key string) (*TenantAPIKey, error) {
	// Hash the provided key
	keyHash, err := s.hashAPIKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Look up API key by hash
	apiKey, err := s.repository.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Check if key is active
	if apiKey.Status != APIKeyStatusActive {
		return nil, errors.New("API key is not active")
	}

	// Check if key has expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, errors.New("API key has expired")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsed = &now
	if err := s.repository.UpdateAPIKey(ctx, apiKey); err != nil {
		s.logger.WithError(err).Error("Failed to update API key last used")
	}

	return apiKey, nil
}

// Helper methods
func (s *ServiceImpl) validateCreateTenantRequest(req *CreateTenantRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Domain == "" {
		return errors.New("domain is required")
	}
	if req.Plan == "" {
		return errors.New("plan is required")
	}
	return nil
}

func (s *ServiceImpl) validateCreateTenantUserRequest(req *CreateTenantUserRequest) error {
	if req.TenantID == "" {
		return errors.New("tenant ID is required")
	}
	if req.UserID == "" {
		return errors.New("user ID is required")
	}
	if req.Email == "" {
		return errors.New("email is required")
	}
	if req.Role == "" {
		return errors.New("role is required")
	}
	return nil
}

func (s *ServiceImpl) validateCreateAPIKeyRequest(req *CreateAPIKeyRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (s *ServiceImpl) getDefaultSettings(plan TenantPlan) *TenantSettings {
	settings := &TenantSettings{
		DefaultCurrency:   "USD",
		SupportedCurrencies: []string{"USD", "EUR", "GBP"},
		APIRateLimitPerMinute: 60,
		APIRateLimitPerHour:   1000,
		SessionTimeout:        120, // 2 hours
		DataRetentionDays:     90,
		ComplianceRegions:     []string{"US", "EU"},
	}

	switch plan {
	case TenantPlanFree:
		settings.EnableMultiCurrency = false
		settings.EnableAdvancedAnalytics = false
		settings.EnableAPIAccess = true
		settings.EnableWebhooks = false
		settings.Require2FA = false
	case TenantPlanBasic:
		settings.EnableMultiCurrency = true
		settings.EnableAdvancedAnalytics = false
		settings.EnableAPIAccess = true
		settings.EnableWebhooks = false
		settings.Require2FA = false
	case TenantPlanPro:
		settings.EnableMultiCurrency = true
		settings.EnableAdvancedAnalytics = true
		settings.EnableAPIAccess = true
		settings.EnableWebhooks = true
		settings.Require2FA = true
	case TenantPlanEnterprise:
		settings.EnableMultiCurrency = true
		settings.EnableAdvancedAnalytics = true
		settings.EnableAPIAccess = true
		settings.EnableWebhooks = true
		settings.Require2FA = true
		settings.APIRateLimitPerMinute = 1000
		settings.APIRateLimitPerHour = 10000
	}

	return settings
}

func (s *ServiceImpl) getDefaultQuotas(plan TenantPlan) []ResourceQuota {
	quotas := []ResourceQuota{}

	switch plan {
	case TenantPlanFree:
		quotas = append(quotas, ResourceQuota{ResourceType: "users", Limit: 5, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "profiles", Limit: 100, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "carriers", Limit: 3, Period: "monthly"})
	case TenantPlanBasic:
		quotas = append(quotas, ResourceQuota{ResourceType: "users", Limit: 25, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "profiles", Limit: 1000, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "carriers", Limit: 10, Period: "monthly"})
	case TenantPlanPro:
		quotas = append(quotas, ResourceQuota{ResourceType: "users", Limit: 100, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "profiles", Limit: 10000, Period: "monthly"})
		quotas = append(quotas, ResourceQuota{ResourceType: "carriers", Limit: 50, Period: "monthly"})
	case TenantPlanEnterprise:
		quotas = append(quotas, ResourceQuota{ResourceType: "users", Limit: -1, Period: "monthly"}) // Unlimited
		quotas = append(quotas, ResourceQuota{ResourceType: "profiles", Limit: -1, Period: "monthly"}) // Unlimited
		quotas = append(quotas, ResourceQuota{ResourceType: "carriers", Limit: -1, Period: "monthly"}) // Unlimited
	}

	return quotas
}

func (s *ServiceImpl) getDefaultFeatures(plan TenantPlan) map[string]bool {
	features := map[string]bool{
		"multi_currency":     false,
		"advanced_analytics": false,
		"api_access":         true,
		"webhooks":           false,
		"custom_branding":    false,
		"priority_support":   false,
	}

	switch plan {
	case TenantPlanBasic:
		features["multi_currency"] = true
	case TenantPlanPro:
		features["multi_currency"] = true
		features["advanced_analytics"] = true
		features["webhooks"] = true
		features["custom_branding"] = true
	case TenantPlanEnterprise:
		features["multi_currency"] = true
		features["advanced_analytics"] = true
		features["webhooks"] = true
		features["custom_branding"] = true
		features["priority_support"] = true
	}

	return features
}

func (s *ServiceImpl) generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less secure method if crypto/rand fails
		return uuid.New().String()
	}
	return "tk_" + hex.EncodeToString(bytes)
}

func (s *ServiceImpl) hashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
