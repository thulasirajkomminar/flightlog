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
)

// User represents a registered user entity.
type User struct {
	ID           string    `json:"id"        gorm:"primaryKey"                    example:"user-abc-123"`
	Email        string    `json:"email"     gorm:"uniqueIndex;not null"          example:"thulasiraj@flightlog.app"`
	PasswordHash string    `json:"-"         gorm:"column:password_hash;not null"`
	Name         string    `json:"name"      gorm:"not null"                      example:"Thulasiraj Komminar"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// GenerateID creates a UUID for the user if not already set.
func (u *User) GenerateID() {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
}
