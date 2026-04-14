package services

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/nutcas3/telecom-platform/apps/api-server/internal/config"
	"github.com/nutcas3/telecom-platform/apps/api-server/internal/database"
	"github.com/nutcas3/telecom-platform/apps/api-server/internal/models"
)

// SubscriberService handles subscriber management operations
type SubscriberService struct {
	db         *database.Database
	config     *config.Config
	amfClient  *AMFClient
	es2Service *ES2Service
}

// NewSubscriberService creates a new subscriber service
func NewSubscriberService(db *database.Database, cfg *config.Config) *SubscriberService {
	return &SubscriberService{
		db:         db,
		config:     cfg,
		amfClient:  NewAMFClient("http://localhost:8081"), // Default AMF URL
		es2Service: NewES2Service(&cfg.ES2),
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

	// Create subscriber
	subscriber := &models.Subscriber{
		IMSI:           imsi,
		MSISDN:         req.MSISDN,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		OrganizationID: req.OrganizationID,
		Status:         models.SubscriberStatusActive,
		PlanID:         req.PlanID,
		AuthKey:        authKey,
		OPc:            opc,
		ServingPLMN:    models.PLMN{MCC: "208", MNC: "93"}, // France
		ProfileStatus:  models.ProfileStatusInactive,
	}

	if err := s.db.CreateSubscriber(ctx, subscriber); err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	// Initiate eSIM profile provisioning if EUICCID provided
	if req.EUICCID != "" {
		subscriber.EUICCID = req.EUICCID
		go func() {
			ctx := context.Background()
			if err := s.provisionESIMProfile(ctx, subscriber.ID); err != nil {
				// Log error but don't fail subscriber creation
				fmt.Printf("Failed to provision eSIM profile for subscriber %d: %v\n", subscriber.ID, err)
			}
		}()
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
		if err := s.deactivateESIMProfile(ctx, subscriber.ID); err != nil {
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
	// Generate 128-bit random key (K)
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", "", err
	}

	// Get OP (Operator variant) from operator configuration
	// This should be consistent across all subscribers for the same operator
	op := s.getOperatorVariant()
	if op == nil {
		return "", "", fmt.Errorf("operator variant not configured")
	}

	// Generate OPc (derived from OP and K) using AES-128 encryption
	// OPc = AES-128(K, OP) where OP is encrypted with K
	opc, err := s.generateOPc(key, op)
	if err != nil {
		return "", "", err
	}

	return hex.EncodeToString(key), hex.EncodeToString(opc), nil
}

// getOperatorVariant returns the operator variant (OP) from configuration
func (s *SubscriberService) getOperatorVariant() []byte {
	// Get operator variant from environment variable or secure configuration
	// This should be the same across all subscribers for the same operator
	opStr := os.Getenv("OPERATOR_VARIANT")
	if opStr == "" {
		// Fallback to default for development
		opStr = "TelecomOP1234567" // 16-byte operator variant
	}

	// Ensure exactly 16 bytes for AES-128
	op := make([]byte, 16)
	copy(op, []byte(opStr))

	// If shorter than 16 bytes, pad with zeros
	if len(opStr) < 16 {
		for i := len(opStr); i < 16; i++ {
			op[i] = 0
		}
	}

	return op
}

// generateOPc derives OPc from OP and K using AES-128 encryption
func (s *SubscriberService) generateOPc(k, op []byte) ([]byte, error) {
	// Create AES-128 cipher block with key K
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// OPc = AES-128(K, OP) - encrypt OP with key K
	opc := make([]byte, aes.BlockSize) // AES block size is 16 bytes

	// Create ECB mode cipher (as per 3GPP specification for OPc derivation)
	// In 3GPP, OPc is derived using AES-128 ECB mode
	if len(op) != aes.BlockSize {
		return nil, fmt.Errorf("OP must be 16 bytes for AES-128")
	}

	// Encrypt OP using ECB mode (single block)
	block.Encrypt(opc, op)

	return opc, nil
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
		if err := s.amfClient.TerminateSession(ctx, imsi, "Subscriber terminated"); err != nil {
			// Log error but continue with other sessions
			fmt.Printf("Failed to notify AMF for session termination: %v\n", err)
		}
	}

	return nil
}

// provisionESIMProfile provisions an eSIM profile for the subscriber
func (s *SubscriberService) provisionESIMProfile(ctx context.Context, subscriberID uint) error {
	// Get subscriber details
	subscriber, err := s.db.GetSubscriber(ctx, subscriberID)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %w", err)
	}

	// Validate EID
	if err := s.es2Service.ValidateEID(subscriber.EUICCID); err != nil {
		return fmt.Errorf("invalid EID: %w", err)
	}

	// Provision profile via ES2+ API
	profileInfo, err := s.es2Service.ProvisionProfile(ctx, subscriber)
	if err != nil {
		return fmt.Errorf("failed to provision profile: %w", err)
	}

	// Update subscriber with profile information
	subscriber.ProfileID = profileInfo.ProfileID
	subscriber.ProfileStatus = models.ProfileStatusDownloading

	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return fmt.Errorf("failed to update subscriber: %w", err)
	}

	// Activate profile
	if err := s.es2Service.ActivateProfile(ctx, subscriber.EUICCID, profileInfo.ProfileID); err != nil {
		return fmt.Errorf("failed to activate profile: %w", err)
	}

	// Update subscriber status
	subscriber.ProfileStatus = models.ProfileStatusActive
	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return fmt.Errorf("failed to update subscriber status: %w", err)
	}

	// Notify AMF of new subscriber
	if err := s.amfClient.NotifySubscriberUpdate(ctx, subscriber.IMSI, models.SubscriberStatusActive); err != nil {
		fmt.Printf("Failed to notify AMF of subscriber activation: %v\n", err)
	}

	return nil
}

// deactivateESIMProfile deactivates an eSIM profile
func (s *SubscriberService) deactivateESIMProfile(ctx context.Context, subscriberID uint) error {
	// Get subscriber details
	subscriber, err := s.db.GetSubscriber(ctx, subscriberID)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %w", err)
	}

	// Check if subscriber has active profile
	if subscriber.ProfileID == "" || subscriber.ProfileStatus != models.ProfileStatusActive {
		return fmt.Errorf("no active profile to deactivate")
	}

	// Deactivate profile via ES2+ API
	if err := s.es2Service.DeactivateProfile(ctx, subscriber.EUICCID, subscriber.ProfileID); err != nil {
		return fmt.Errorf("failed to deactivate profile: %w", err)
	}

	// Update subscriber status
	subscriber.ProfileStatus = models.ProfileStatusInactive
	subscriber.Status = models.SubscriberStatusInactive

	if err := s.db.UpdateSubscriber(ctx, subscriber); err != nil {
		return fmt.Errorf("failed to update subscriber: %w", err)
	}

	// Terminate all sessions
	if err := s.terminateSubscriberSessions(ctx, subscriber.IMSI); err != nil {
		fmt.Printf("Failed to terminate sessions: %v\n", err)
	}

	// Notify AMF of subscriber deactivation
	if err := s.amfClient.NotifySubscriberUpdate(ctx, subscriber.IMSI, models.SubscriberStatusInactive); err != nil {
		fmt.Printf("Failed to notify AMF of subscriber deactivation: %v\n", err)
	}

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
