package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrEmailAlreadyExists and related domain errors.
var (
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrValidation         = errors.New("validation error")
	ErrFlightAlreadyAdded = errors.New("flight already added to your collection")
	ErrFlightNotFound     = errors.New("flight not found")
	// ErrProviderQuota means the flight data provider is refusing requests
	// because the API quota is used up.
	ErrProviderQuota = errors.New("flight data quota is used up")
)

// User represents a registered user entity.
type User struct {
	ID           string    `json:"id"        gorm:"primaryKey"                    example:"user-abc-123"`
	Email        string    `json:"email"     gorm:"uniqueIndex;not null"          example:"thulasiraj@flightlog.app"`
	PasswordHash string    `json:"-"         gorm:"column:password_hash;not null"`
	Name         string    `json:"name"      gorm:"not null"                      example:"Thulasiraj Komminar"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	// SessionsValidFrom invalidates older sessions: a login token issued
	// before this moment is no longer accepted. Zero means no cut-off.
	SessionsValidFrom time.Time `json:"-" gorm:"column:sessions_valid_from"`
}

// GenerateID creates a UUID for the user if not already set.
func (u *User) GenerateID() {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
}
