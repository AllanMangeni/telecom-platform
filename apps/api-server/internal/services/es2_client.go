package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nutcas3/telecom-platform/apps/api-server/internal/config"
	"github.com/nutcas3/telecom-platform/apps/api-server/internal/models"
)

// ES2Service handles GSMA ES2+ operations for eSIM provisioning
type ES2Service struct {
	httpClient *http.Client
	config     *config.ES2Config
}

// NewES2Service creates a new ES2 service
func NewES2Service(cfg *config.ES2Config) *ES2Service {
	return &ES2Service{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: cfg.InsecureSkipVerify,
				},
			},
		},
		config: cfg,
	}
}

// ES2 request/response types
type DownloadOrderRequest struct {
	EID          string            `json:"eid"`
	ICCID        string            `json:"iccid"`
	ProfileType  string            `json:"profileType"`
	Confirmation bool              `json:"confirmation"`
	Metadata     map[string]string `json:"metadata"`
}

type DownloadOrderResponse struct {
	ICCID               string `json:"iccid"`
	ProfileID           string `json:"profileId"`
	ActivationCode      string `json:"activationCode"`
	ConfirmationAddress string `json:"confirmationAddress"`
}

type ActivationRequest struct {
	EID       string `json:"eid"`
	ProfileID string `json:"profileId"`
}

type DeactivationRequest struct {
	EID       string `json:"eid"`
	ProfileID string `json:"profileId"`
}

type DeletionRequest struct {
	EID       string `json:"eid"`
	ProfileID string `json:"profileId"`
}

type ProfileStatusResponse struct {
	ICCID       string `json:"iccid"`
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	State       string `json:"state"`
	Operator    string `json:"operator"`
}

type ListProfilesResponse struct {
	Profiles []ProfileStatusResponse `json:"profiles"`
}

// ProfileInfo represents eSIM profile information
type ProfileInfo struct {
	ICCID       string            `json:"iccid"`
	ProfileID   string            `json:"profileId"`
	ProfileName string            `json:"profileName"`
	State       string            `json:"state"`
	Operator    string            `json:"operator"`
	Activation  ProfileActivation `json:"activation"`
}

// ProfileActivation represents profile activation details
type ProfileActivation struct {
	ActivationCode string `json:"activationCode"`
	ConfAddress    string `json:"confirmationAddress"`
}

// ProvisionProfile provisions an eSIM profile for a subscriber
func (e *ES2Service) ProvisionProfile(ctx context.Context, subscriber *models.Subscriber) (*ProfileInfo, error) {
	// Check if subscriber has EUICCID
	if subscriber.EUICCID == "" {
		return nil, fmt.Errorf("subscriber must have EUICCID for eSIM provisioning")
	}

	// Create profile download request
	req := DownloadOrderRequest{
		EID:          subscriber.EUICCID,
		ICCID:        "", // Will be assigned by SM-SR
		ProfileType:  "operational",
		Confirmation: true,
		Metadata: map[string]string{
			"subscriber_id": fmt.Sprintf("%d", subscriber.ID),
			"imsi":          string(subscriber.IMSI),
			"organization":  subscriber.OrganizationID,
		},
	}

	// Call SM-SR to download profile
	resp, err := e.downloadProfile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to download profile: %w", err)
	}

	// Create profile info
	profileInfo := &ProfileInfo{
		ICCID:       resp.ICCID,
		ProfileID:   resp.ProfileID,
		ProfileName: fmt.Sprintf("Profile-%s", subscriber.IMSI),
		State:       "downloaded",
		Operator:    "Telecom Platform",
		Activation: ProfileActivation{
			ActivationCode: resp.ActivationCode,
			ConfAddress:    resp.ConfirmationAddress,
		},
	}

	return profileInfo, nil
}

// downloadProfile is an internal method to download a profile
func (e *ES2Service) downloadProfile(ctx context.Context, req DownloadOrderRequest) (*DownloadOrderResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal download request: %w", err)
	}

	url := fmt.Sprintf("%s/es2/download", e.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	var response DownloadOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode download response: %w", err)
	}

	return &response, nil
}

// ActivateProfile activates an eSIM profile
func (e *ES2Service) ActivateProfile(ctx context.Context, euiccID, profileID string) error {
	req := ActivationRequest{
		EID:       euiccID,
		ProfileID: profileID,
	}

	return e.sendActivationRequest(ctx, req, "activate")
}

// DeactivateProfile deactivates an eSIM profile
func (e *ES2Service) DeactivateProfile(ctx context.Context, euiccID, profileID string) error {
	req := DeactivationRequest{
		EID:       euiccID,
		ProfileID: profileID,
	}

	return e.sendDeactivationRequest(ctx, req)
}

// DeleteProfile deletes an eSIM profile
func (e *ES2Service) DeleteProfile(ctx context.Context, euiccID, profileID string) error {
	req := DeletionRequest{
		EID:       euiccID,
		ProfileID: profileID,
	}

	return e.sendDeletionRequest(ctx, req)
}

// GetProfileStatus retrieves the status of an eSIM profile
func (e *ES2Service) GetProfileStatus(ctx context.Context, euiccID, profileID string) (*ProfileInfo, error) {
	url := fmt.Sprintf("%s/es2/eid/%s/profile/%s/status", e.config.BaseURL, euiccID, profileID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	var response ProfileStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	profileInfo := &ProfileInfo{
		ICCID:       response.ICCID,
		ProfileID:   response.ProfileID,
		ProfileName: response.ProfileName,
		State:       response.State,
		Operator:    response.Operator,
	}

	return profileInfo, nil
}

// ListProfiles lists all profiles on an eUICC
func (e *ES2Service) ListProfiles(ctx context.Context, euiccID string) ([]*ProfileInfo, error) {
	url := fmt.Sprintf("%s/es2/eid/%s/profiles", e.config.BaseURL, euiccID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	var response ListProfilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	var profiles []*ProfileInfo
	for _, profile := range response.Profiles {
		profileInfo := &ProfileInfo{
			ICCID:       profile.ICCID,
			ProfileID:   profile.ProfileID,
			ProfileName: profile.ProfileName,
			State:       profile.State,
			Operator:    profile.Operator,
		}
		profiles = append(profiles, profileInfo)
	}

	return profiles, nil
}

// Helper methods for ES2 operations
func (e *ES2Service) sendActivationRequest(ctx context.Context, req ActivationRequest, action string) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal activation request: %w", err)
	}

	url := fmt.Sprintf("%s/es2/activate", e.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send activation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	return nil
}

func (e *ES2Service) sendDeactivationRequest(ctx context.Context, req any) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal deactivation request: %w", err)
	}

	url := fmt.Sprintf("%s/es2/deactivate", e.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send deactivation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	return nil
}

func (e *ES2Service) sendDeletionRequest(ctx context.Context, req DeletionRequest) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal deletion request: %w", err)
	}

	url := fmt.Sprintf("%s/es2/delete", e.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", generateRequestID())
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send deletion request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ES2 server returned status %d", resp.StatusCode)
	}

	return nil
}

// ValidateEID validates an EID format
func (e *ES2Service) ValidateEID(eid string) error {
	if len(eid) != 32 {
		return fmt.Errorf("invalid EID length: expected 32 characters, got %d", len(eid))
	}

	// Additional validation logic can be added here
	return nil
}

// ValidateICCID validates an ICCID format
func (e *ES2Service) ValidateICCID(iccid string) error {
	if len(iccid) < 18 || len(iccid) > 22 {
		return fmt.Errorf("invalid ICCID length: expected 18-22 characters, got %d", len(iccid))
	}

	// Additional validation logic can be added here
	return nil
}
