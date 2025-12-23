// Package storage provides data persistence implementations
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
	"golang.org/x/crypto/bcrypt"
)

// InMemoryAuthStore provides a simple in-memory auth store for development
type InMemoryAuthStore struct {
	users  map[string]*userRecord
	tokens map[string]*tokenRecord
	mu     sync.RWMutex
}

type userRecord struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Permissions  []rest.Permission
}

type tokenRecord struct {
	UserID    string
	ExpiresAt time.Time
}

// NewInMemoryAuthStore creates a new in-memory auth store with a demo user
func NewInMemoryAuthStore() *InMemoryAuthStore {
	store := &InMemoryAuthStore{
		users:  make(map[string]*userRecord),
		tokens: make(map[string]*tokenRecord),
	}

	// Add demo user: admin@kubewise.io / kubewise123
	hash, _ := bcrypt.GenerateFromPassword([]byte("kubewise123"), bcrypt.DefaultCost)
	store.users["admin@kubewise.io"] = &userRecord{
		ID:           "user-1",
		Email:        "admin@kubewise.io",
		Name:         "Admin User",
		PasswordHash: string(hash),
		Permissions: []rest.Permission{
			"read:recommendations",
			"write:recommendations",
			"read:costs",
			"read:anomalies",
			"read:clusters",
			"admin",
		},
	}

	return store
}

// Authenticate validates credentials and returns user with token
func (s *InMemoryAuthStore) Authenticate(ctx context.Context, email, password string) (*rest.User, string, error) {
	s.mu.RLock()
	user, exists := s.users[email]
	s.mu.RUnlock()

	if !exists {
		return nil, "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	// Generate token
	token, err := generateRandomToken()
	if err != nil {
		return nil, "", err
	}

	// Store token
	s.mu.Lock()
	s.tokens[token] = &tokenRecord{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	s.mu.Unlock()

	return &rest.User{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}, token, nil
}

// ValidateToken checks if a token is valid and returns the user ID
func (s *InMemoryAuthStore) ValidateToken(ctx context.Context, token string) (string, error) {
	s.mu.RLock()
	record, exists := s.tokens[token]
	s.mu.RUnlock()

	if !exists {
		return "", errors.New("invalid token")
	}

	if time.Now().After(record.ExpiresAt) {
		s.mu.Lock()
		delete(s.tokens, token)
		s.mu.Unlock()
		return "", errors.New("token expired")
	}

	return record.UserID, nil
}

// GetUser returns a user by ID
func (s *InMemoryAuthStore) GetUser(ctx context.Context, userID string) (*rest.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.ID == userID {
			return &rest.User{
				ID:    user.ID,
				Email: user.Email,
				Name:  user.Name,
			}, nil
		}
	}

	return nil, errors.New("user not found")
}

// GetPermissions returns permissions for a user
func (s *InMemoryAuthStore) GetPermissions(ctx context.Context, userID string) ([]rest.Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.ID == userID {
			return user.Permissions, nil
		}
	}

	return nil, errors.New("user not found")
}

func generateRandomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
