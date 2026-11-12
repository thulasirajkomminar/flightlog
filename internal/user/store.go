package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// importJobsTable is owned by the importer package; it is named here only to
// clear a user's row when their account is deleted.
const importJobsTable = "import_jobs"

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

// GetByEmail retrieves a user by email, ignoring case so accounts created
// before addresses were normalised can still be found.
func (s *Store) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Delete removes a user, their flight collection and their import job.
// Cached flights are shared between users and are kept.
func (s *Store) Delete(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ?", id).Delete(&domain.UserFlight{}).Error
		if err != nil {
			return err
		}

		// The user's import job can still hold rows from their uploaded file.
		if tx.Migrator().HasTable(importJobsTable) {
			err = tx.Exec("DELETE FROM "+importJobsTable+" WHERE user_id = ?", id).Error
			if err != nil {
				return err
			}
		}

		return tx.Delete(&domain.User{}, "id = ?", id).Error
	})
	if err != nil {
		return fmt.Errorf("delete user and collection: %w", err)
	}

	return nil
}

// Update saves changes to an existing user record.
func (s *Store) Update(ctx context.Context, user *domain.User) error {
	return s.db.WithContext(ctx).Save(user).Error
}
