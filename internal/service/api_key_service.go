package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

type APIKeyService interface {
	CreateKey(ctx context.Context, userID uuid.UUID, name string, domainID *uuid.UUID) (string, *models.APIKey, error)
	GetKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error)
	ListKeys(ctx context.Context, userID uuid.UUID) ([]*models.APIKey, error)
	RevokeKey(ctx context.Context, id uuid.UUID) error
}

type apiKeyService struct {
	keyRepo    repository.APIKeyRepository
	domainRepo repository.DomainRepository
}

func NewAPIKeyService(keyRepo repository.APIKeyRepository, domainRepo repository.DomainRepository) APIKeyService {
	return &apiKeyService{
		keyRepo:    keyRepo,
		domainRepo: domainRepo,
	}
}

func (s *apiKeyService) CreateKey(ctx context.Context, userID uuid.UUID, name string, domainID *uuid.UUID) (string, *models.APIKey, error) {
	// Verify domain belongs to the user if domainID is provided
	var nullDomainID uuid.NullUUID
	if domainID != nil && *domainID != uuid.Nil {
		domain, err := s.domainRepo.GetByID(ctx, *domainID)
		if err != nil {
			return "", nil, fmt.Errorf("verify domain: %w", err)
		}
		if domain == nil {
			return "", nil, errors.New("domain not found")
		}
		if domain.UserID != userID {
			return "", nil, errors.New("unauthorized domain binding")
		}
		nullDomainID = uuid.NullUUID{UUID: *domainID, Valid: true}
	}

	// Generate secure token: rs_live_ + 32 URL-safe base64 characters
	prefix := "rs_live_"
	entropyBytes := make([]byte, 24) // 24 bytes => 32 base64 chars
	if _, err := rand.Read(entropyBytes); err != nil {
		return "", nil, fmt.Errorf("read crypto rand: %w", err)
	}
	entropy := base64.RawURLEncoding.EncodeToString(entropyBytes)
	rawKey := prefix + entropy

	// Compute SHA-256 hash of rawKey
	hashBytes := sha256.Sum256([]byte(rawKey))
	hashHex := hex.EncodeToString(hashBytes[:])

	last4 := rawKey[len(rawKey)-4:]

	apiKey := &models.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hashHex,
		Last4:     last4,
		Scopes:    []string{"email:send"}, // Default send permission scope
		DomainID:  nullDomainID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	err := s.keyRepo.Create(ctx, apiKey)
	if err != nil {
		return "", nil, fmt.Errorf("save key: %w", err)
	}

	// Return rawKey to reveal exactly once, alongside the stored metadata
	return rawKey, apiKey, nil
}

func (s *apiKeyService) GetKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	key, err := s.keyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
	}
	if key == nil {
		return nil, errors.New("api key not found")
	}
	return key, nil
}

func (s *apiKeyService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*models.APIKey, error) {
	keys, err := s.keyRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	return keys, nil
}

func (s *apiKeyService) RevokeKey(ctx context.Context, id uuid.UUID) error {
	key, err := s.keyRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("check key exist: %w", err)
	}
	if key == nil {
		return errors.New("api key not found")
	}

	err = s.keyRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("revoke/delete key: %w", err)
	}
	return nil
}
