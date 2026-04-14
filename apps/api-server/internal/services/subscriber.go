package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nutcas3/telecom-platform/apps/api-server/internal/config"
	"github.com/nutcas3/telecom-platform/apps/api-server/internal/database"
	"github.com/nutcas3/telecom-platform/apps/api-server/internal/models"
)

// SubscriberService handles subscriber management operations
type SubscriberService struct {
	db     *database.Database
	config *config.Config
}

// NewSubscriberService creates a new subscriber service
func NewSubscriberService(db *database.Database, cfg *config.Config) *SubscriberService {
	return &SubscriberService{
		db:     db,
		config: cfg,
	}
}

// CreateSubscriber creates a new subscriber with allocated IMSI
func (s *SubscriberService) CreateSubscriber(ctx context.Context, req *CreateSubscriberRequest) (*models.Subscriber, error) {
	// Allocate IMSI
	imsi, err := s.allocateIMSI(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IMSI: %w", err)
	}

	// Generate authentication keys
	authKey, opc, err := s.generateAuthKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth keys: %w", err)
	}

	subscriber := &models.Subscriber{
		IMSI:           imsi,
		MSISDN:         req.MSISDN,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		OrganizationID: req.OrganizationID,
		Status:         models.SubscriberStatusProvisioning,
		PlanID:         req.PlanID,
		AuthKey:        authKey,
		OPc:            opc,
		ServingPLMN:    models.PLMN{MCC: "208", MNC: "93"}, // Default France PLMN
	}

	// Create subscriber
	if err := s.db.CreateSubscriber(ctx, subscriber); err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	// TODO: Initiate eSIM profile provisioning if EUICCID provided
	if req.EUICCID != "" {
		subscriber.EUICCID = req.EUICCID
		subscriber.ProfileStatus = models.ProfileStatusDownloading
		s.db.UpdateSubscriber(ctx, subscriber)

		// Trigger eSIM provisioning asynchronously
		go s.provisionESIMProfile(subscriber.ID)
	} else {
		// Activate immediately for physical SIM
		subscriber.Status = models.SubscriberStatusActive
		subscriber.ProfileStatus = models.ProfileStatusActive
		s.db.UpdateSubscriber(ctx, subscriber)
	}

	return subscriber, nil
}

// GetSubscriber retrieves a subscriber by ID
func (s *SubscriberService) GetSubscriber(ctx context.Context, id uint) (*models.Subscriber, error) {
	subscriber, err := s.db.GetSubscriber(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}
	return subscriber, nil
}

// GetSubscriberByIMSI retrieves a subscriber by IMSI
func (s *SubscriberService) GetSubscriberByIMSI(ctx context.Context, imsi models.IMSI) (*models.Subscriber, error) {
	subscriber, err := s.db.GetSubscriberByIMSI(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber by IMSI: %w", err)
	}
	return subscriber, nil
}

// UpdateSubscriber updates subscriber information
func (s *SubscriberService) UpdateSubscriber(ctx context.Context, id uint, req *UpdateSubscriberRequest) (*models.Subscriber, error) {
	subscriber, err := s.db.GetSubscriber(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}

	// Update fields
	if req.FirstName != nil && *req.FirstName != "" {
		subscriber.FirstName = *req.FirstName
	}
	if req.LastName != nil && *req.LastName != "" {
		subscriber.LastName = *req.LastName
	}
	if req.Email != nil && *req.Email != "" {
		subscriber.Email = *req.Email
	}
	if req.Status != "" {
		subscriber.Status = req.Status
	}
	if req.PlanID != nil {
		subscriber.PlanID = *req.PlanID
	}

	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return nil, fmt.Errorf("failed to update subscriber: %w", err)
	}

	return subscriber, nil
}

// SuspendSubscriber suspends a subscriber
func (s *SubscriberService) SuspendSubscriber(ctx context.Context, id uint) error {
	subscriber, err := s.db.GetSubscriber(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %w", err)
	}

	subscriber.Status = models.SubscriberStatusSuspended

	// Terminate active sessions
	if err := s.terminateSubscriberSessions(ctx, subscriber.IMSI); err != nil {
		return fmt.Errorf("failed to terminate sessions: %w", err)
	}

	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return fmt.Errorf("failed to suspend subscriber: %w", err)
	}

	return nil
}

// TerminateSubscriber terminates a subscriber
func (s *SubscriberService) TerminateSubscriber(ctx context.Context, id uint) error {
	subscriber, err := s.db.GetSubscriber(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %w", err)
	}

	subscriber.Status = models.SubscriberStatusTerminated

	// Terminate active sessions
	if err := s.terminateSubscriberSessions(ctx, subscriber.IMSI); err != nil {
		return fmt.Errorf("failed to terminate sessions: %w", err)
	}

	// Deactivate eSIM profile if active
	if subscriber.EUICCID != "" && subscriber.ProfileStatus == models.ProfileStatusActive {
		if err := s.deactivateESIMProfile(subscriber.ID); err != nil {
			return fmt.Errorf("failed to deactivate eSIM profile: %w", err)
		}
	}

	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return fmt.Errorf("failed to terminate subscriber: %w", err)
	}

	return nil
}

// ListSubscribers lists subscribers with pagination and filtering
func (s *SubscriberService) ListSubscribers(ctx context.Context, req *ListSubscribersRequest) (*ListSubscribersResponse, error) {
	dbReq := &database.ListSubscribersRequest{
		Page:           req.Page,
		PageSize:       req.PageSize,
		Status:         req.Status,
		OrganizationID: req.OrganizationID,
		Search:         req.Search,
	}
	subscribers, total, err := s.db.ListSubscribers(ctx, dbReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscribers: %w", err)
	}

	return &ListSubscribersResponse{
		Subscribers: subscribers,
		Total:       total,
		Page:        req.Page,
		PageSize:    req.PageSize,
	}, nil
}

// allocateIMSI allocates a new IMSI from the configured range
func (s *SubscriberService) allocateIMSI(ctx context.Context) (models.IMSI, error) {
	// Get current IMSI allocation state
	alloc, err := s.db.GetIMSIAllocation(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get IMSI allocation: %w", err)
	}

	// Check if we have IMSIs available
	if alloc.LastIMSI >= alloc.MaxIMSI {
		return "", fmt.Errorf("IMSI range exhausted")
	}

	// Allocate next IMSI
	nextIMSI := alloc.LastIMSI + 1
	alloc.LastIMSI = nextIMSI

	// Update allocation state
	if err := s.db.UpdateIMSIAllocation(ctx, alloc); err != nil {
		return "", fmt.Errorf("failed to update IMSI allocation: %w", err)
	}

	// Format IMSI: MCC (3) + MNC (2-3) + subscriber number
	imsiStr := fmt.Sprintf("%s%010d", s.config.IMSI.Prefix, nextIMSI)
	return models.IMSI(imsiStr), nil
}

// generateAuthKeys generates authentication keys for the subscriber
func (s *SubscriberService) generateAuthKeys() (string, string, error) {
	// Generate 128-bit random key
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", "", err
	}

	// Generate OPc (derived from OP and K)
	// For simplicity, using another random key here
	opc := make([]byte, 16)
	if _, err := rand.Read(opc); err != nil {
		return "", "", err
	}

	return hex.EncodeToString(key), hex.EncodeToString(opc), nil
}

// terminateSubscriberSessions terminates all active sessions for a subscriber
func (s *SubscriberService) terminateSubscriberSessions(ctx context.Context, imsi models.IMSI) error {
	sessions, err := s.db.GetActiveSessionsByIMSI(ctx, imsi)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		now := time.Now()
		session.Status = models.SessionStatusInactive
		session.EndTime = &now

		if err := s.db.UpdateSession(ctx, &session); err != nil {
			return err
		}

		// Notify AMF to terminate session
		// TODO: Implement AMF notification
	}

	return nil
}

// provisionESIMProfile provisions an eSIM profile for the subscriber
func (s *SubscriberService) provisionESIMProfile(subscriberID uint) {
	// TODO: Implement GSMA ES2+ API integration
	// This would involve:
	// 1. Calling SM-SR to download profile
	// 2. Installing profile on eUICC
	// 3. Updating subscriber status
}

// deactivateESIMProfile deactivates an eSIM profile
func (s *SubscriberService) deactivateESIMProfile(subscriberID uint) error {
	// TODO: Implement GSMA ES2+ API integration
	// This would involve:
	// 1. Calling SM-SR to deactivate profile
	// 2. Updating subscriber status
	return nil
}

// Request/Response types
type CreateSubscriberRequest struct {
	MSISDN         string `json:"msisdn" validate:"required"`
	FirstName      string `json:"first_name" validate:"required"`
	LastName       string `json:"last_name" validate:"required"`
	Email          string `json:"email" validate:"required,email"`
	OrganizationID string `json:"organization_id"`
	PlanID         uint   `json:"plan_id" validate:"required"`
	EUICCID        string `json:"euicc_id"`
}

type UpdateSubscriberRequest struct {
	FirstName *string                 `json:"first_name"`
	LastName  *string                 `json:"last_name"`
	Email     *string                 `json:"email"`
	Status    models.SubscriberStatus `json:"status"`
	PlanID    *uint                   `json:"plan_id"`
}

type ListSubscribersRequest struct {
	Page           int                     `json:"page" query:"page"`
	PageSize       int                     `json:"page_size" query:"page_size"`
	Status         models.SubscriberStatus `json:"status" query:"status"`
	OrganizationID string                  `json:"organization_id" query:"organization_id"`
	Search         string                  `json:"search" query:"search"`
}

type ListSubscribersResponse struct {
	Subscribers []models.Subscriber `json:"subscribers"`
	Total       int64               `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
}
