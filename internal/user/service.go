package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

const minPasswordLength = 8

// Repository defines the contract for user persistence.
type Repository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// Service implements user operations.
type Service struct {
	repo Repository
	log  *logger.ComponentLogger
}

// NewService creates a Service.
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		log:  logger.NewComponentLogger("user_service"),
	}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	err := validateRegistration(email, password, name)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("check email %s: %w", email, err)
	}

	if existing != nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
	}
	user.GenerateID()

	err = s.repo.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	s.log.Info("user registered", zap.String("user_id", user.ID))

	return user, nil
}

// Authenticate verifies user credentials.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			s.log.Warn("login attempt for unknown email")

			return nil, domain.ErrInvalidCredentials
		}

		return nil, fmt.Errorf("find user by email: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		s.log.Warn("invalid password attempt", zap.String("user_id", user.ID))

		return nil, domain.ErrInvalidCredentials
	}

	s.log.Info("user authenticated", zap.String("user_id", user.ID))

	return user, nil
}

// GetProfile retrieves a user by ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, fmt.Errorf("user profile: %w", err)
		}

		return nil, fmt.Errorf("get profile %s: %w", userID, err)
	}

	return user, nil
}

// UpdateProfile updates a user's name and email.
func (s *Service) UpdateProfile(ctx context.Context, userID, email, name string) (*domain.User, error) {
	err := validateEmail(email)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", userID, err)
	}

	err = s.checkEmailAvailability(ctx, email, user.Email)
	if err != nil {
		return nil, err
	}

	user.Email = email
	user.Name = name

	err = s.repo.Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("update user %s: %w", userID, err)
	}

	return user, nil
}

func validateRegistration(email, password, name string) error {
	err := validateEmail(email)
	if err != nil {
		return err
	}

	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", domain.ErrValidation, minPasswordLength)
	}

	if name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrValidation)
	}

	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("%w: email is required", domain.ErrValidation)
	}

	at := strings.Index(email, "@")
	if at < 1 || strings.LastIndex(email, ".") <= at {
		return fmt.Errorf("%w: invalid email format", domain.ErrValidation)
	}

	return nil
}

func (s *Service) checkEmailAvailability(ctx context.Context, newEmail, currentEmail string) error {
	if newEmail == currentEmail {
		return nil
	}

	existing, err := s.repo.GetByEmail(ctx, newEmail)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("check email %s: %w", newEmail, err)
	}

	if existing != nil {
		return domain.ErrEmailAlreadyExists
	}

	return nil
}
