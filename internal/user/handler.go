// Package user implements user registration, authentication, and profile management.
package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	errUnauthorized = "unauthorized"
	errInvalidBody  = "invalid request body"
)

// RegisterRequest represents a user registration request.
type RegisterRequest struct {
	Email    string `json:"email"    example:"thulasiraj@flightlog.app"`
	Password string `json:"password"`
	Name     string `json:"name"     example:"Thulasiraj Komminar"`
}

// LoginRequest represents a user login request.
type LoginRequest struct {
	Email    string `json:"email"    example:"thulasiraj@flightlog.app"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response with a token and user.
type AuthResponse struct {
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	User  *domain.User `json:"user"`
}

// UpdateProfileRequest represents a profile update request.
type UpdateProfileRequest struct {
	Email string `json:"email" example:"thulasiraj@flightlog.app"`
	Name  string `json:"name"  example:"Thulasiraj Komminar"`
}

const defaultTokenExpiry = 24 * time.Hour

// Handler handles auth and profile HTTP endpoints.
type Handler struct {
	userService *Service
	jwtSecret   []byte
}

// NewHandler creates a Handler.
func NewHandler(userService *Service, jwtSecret string) *Handler {
	return &Handler{
		userService: userService,
		jwtSecret:   []byte(jwtSecret),
	}
}

// Register handles user registration.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, errInvalidBody)

		return
	}

	user, err := h.userService.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		handleRegistrationError(w, err)

		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "failed to generate token")

		return
	}

	setAuthCookie(w, token, defaultTokenExpiry)

	api.RespondJSON(w, http.StatusCreated, AuthResponse{
		Token: token,
		User:  user,
	})
}

// Login handles user authentication.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, errInvalidBody)

		return
	}

	user, err := h.userService.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			api.RespondError(w, http.StatusUnauthorized, err.Error())

			return
		}

		api.RespondError(w, http.StatusInternalServerError, "authentication failed")

		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "failed to generate token")

		return
	}

	setAuthCookie(w, token, defaultTokenExpiry)

	api.RespondJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

// Logout clears the auth cookie.
func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flightlog_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func setAuthCookie(w http.ResponseWriter, token string, expiry time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flightlog_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(expiry.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetProfile returns the authenticated user's profile.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	profile, err := h.userService.GetProfile(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusNotFound, "user not found")

		return
	}

	api.RespondJSON(w, http.StatusOK, profile)
}

// UpdateProfile updates the authenticated user's profile.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	var req UpdateProfileRequest

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, errInvalidBody)

		return
	}

	profile, err := h.userService.UpdateProfile(r.Context(), user.UserID, req.Email, req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			api.RespondError(w, http.StatusConflict, err.Error())

			return
		}

		if errors.Is(err, domain.ErrValidation) {
			api.RespondError(w, http.StatusBadRequest, err.Error())

			return
		}

		api.RespondError(w, http.StatusInternalServerError, "failed to update profile")

		return
	}

	api.RespondJSON(w, http.StatusOK, profile)
}

func (h *Handler) generateToken(user *domain.User) (string, error) {
	now := time.Now()

	claims := &api.UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(defaultTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   user.ID,
		},
		UserID: user.ID,
		Email:  user.Email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

func handleRegistrationError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrEmailAlreadyExists) {
		api.RespondError(w, http.StatusConflict, err.Error())

		return
	}

	if errors.Is(err, domain.ErrValidation) {
		api.RespondError(w, http.StatusBadRequest, err.Error())

		return
	}

	api.RespondError(w, http.StatusInternalServerError, "registration failed")
}
