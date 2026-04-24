package repository

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a profile lookup misses.
var ErrNotFound = errors.New("profile not found")

// Profile is the stored representation of an eSIM profile.
type Profile struct {
	ICCID          string    `json:"iccid"`
	EID            string    `json:"eid,omitempty"`
	IMSI           string    `json:"imsi"`
	MCC            string    `json:"mcc"`
	MNC            string    `json:"mnc"`
	ProfileType    string    `json:"profileType"`
	State          string    `json:"state"` // e.g. provisioned, active, disabled, deleted
	TenantID       string    `json:"tenantId,omitempty"`
	ActivationCode string    `json:"activationCode,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ListFilter narrows the List query.
type ListFilter struct {
	TenantID string
	State    string
	Limit    int
	Offset   int
}

// ProfileRepository is the storage contract for eSIM profiles.
type ProfileRepository interface {
	Create(ctx context.Context, p *Profile) error
	Get(ctx context.Context, iccid string) (*Profile, error)
	List(ctx context.Context, f ListFilter) ([]*Profile, int, error)
	UpdateState(ctx context.Context, iccid, state string) (*Profile, error)
	Delete(ctx context.Context, iccid string) error
}

// InMemoryProfileStore is an in-memory ProfileRepository.
// Safe for concurrent use. Intended as a development default; production
// deployments should swap in a Postgres-backed implementation.
type InMemoryProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]*Profile
}

// NewInMemoryProfileStore constructs an empty InMemoryProfileStore.
func NewInMemoryProfileStore() *InMemoryProfileStore {
	return &InMemoryProfileStore{profiles: make(map[string]*Profile)}
}

// Create inserts a profile. Returns an error if the ICCID already exists.
func (s *InMemoryProfileStore) Create(_ context.Context, p *Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[p.ICCID]; exists {
		return errors.New("profile already exists")
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	cp := *p
	s.profiles[p.ICCID] = &cp
	return nil
}

// Get returns a profile by ICCID or ErrNotFound.
func (s *InMemoryProfileStore) Get(_ context.Context, iccid string) (*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[iccid]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

// List returns a filtered, paginated snapshot of profiles.
func (s *InMemoryProfileStore) List(_ context.Context, f ListFilter) ([]*Profile, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*Profile, 0, len(s.profiles))
	for _, p := range s.profiles {
		if f.TenantID != "" && p.TenantID != f.TenantID {
			continue
		}
		if f.State != "" && p.State != f.State {
			continue
		}
		cp := *p
		matched = append(matched, &cp)
	}

	total := len(matched)
	start := f.Offset
	if start > total {
		start = total
	}
	end := total
	if f.Limit > 0 && start+f.Limit < total {
		end = start + f.Limit
	}
	return matched[start:end], total, nil
}

// UpdateState updates a profile's state and UpdatedAt timestamp.
func (s *InMemoryProfileStore) UpdateState(_ context.Context, iccid, state string) (*Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[iccid]
	if !ok {
		return nil, ErrNotFound
	}
	p.State = state
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	return &cp, nil
}

// Delete removes a profile.
func (s *InMemoryProfileStore) Delete(_ context.Context, iccid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.profiles[iccid]; !ok {
		return ErrNotFound
	}
	delete(s.profiles, iccid)
	return nil
}
