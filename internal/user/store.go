package user

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// Store persists user accounts in SQLite.
type Store struct {
	db *gorm.DB
}

// NewStore returns a new store backed by db.
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new user record.
func (s *Store) Create(ctx context.Context, user *domain.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

// GetByID retrieves a user by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetByEmail retrieves a user by email.
func (s *Store) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Update saves changes to an existing user record.
func (s *Store) Update(ctx context.Context, user *domain.User) error {
	return s.db.WithContext(ctx).Save(user).Error
}
