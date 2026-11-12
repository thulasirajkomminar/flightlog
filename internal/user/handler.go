// Package user implements user registration, authentication, and profile management.
package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	keyMessage              = "message"
	errInvalidBody          = "invalid request body"
	errRegistrationDisabled = "registration is disabled on this instance"
	// errWrongPassword is answered with 400 rather than 401: a 401 would tell
	// the client its session has expired.
	errWrongPassword = "current password is incorrect"
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

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// DeleteAccountRequest confirms account deletion with the user's password.
type DeleteAccountRequest struct {
	Password string `json:"password"`
}

const (
	defaultTokenExpiry = 24 * time.Hour
	authCookieName     = "flightlog_token"
)

// Handler handles auth and profile HTTP endpoints.
type Handler struct {
	userService         *Service
	jwtSecret           []byte
	registrationEnabled bool
}

// NewHandler creates a Handler. When registrationEnabled is false, no new
// accounts can be created.
func NewHandler(userService *Service, jwtSecret string, registrationEnabled bool) *Handler {
	return &Handler{
		userService:         userService,
		jwtSecret:           []byte(jwtSecret),
		registrationEnabled: registrationEnabled,
	}
}

// Config returns the public auth settings the sign-in pages need.
func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	api.RespondJSON(w, http.StatusOK, map[string]bool{"registrationEnabled": h.registrationEnabled})
}

// Register handles user registration.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.registrationEnabled {
		api.RespondError(w, http.StatusForbidden, errRegistrationDisabled)

		return
	}

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

	setAuthCookie(w, r, token, defaultTokenExpiry)

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

	setAuthCookie(w, r, token, defaultTokenExpiry)

	api.RespondJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

// Logout clears the auth cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	setAuthCookie(w, r, "", -time.Second)

	api.RespondJSON(w, http.StatusOK, map[string]string{keyMessage: "logged out"})
}

// setAuthCookie sets the session cookie; a negative expiry deletes it. The
// cookie is marked Secure only when the request arrived over HTTPS, because
// browsers drop Secure cookies on plain-HTTP origins other than localhost,
// which would make login fail silently on a LAN deployment.
func setAuthCookie(w http.ResponseWriter, r *http.Request, token string, expiry time.Duration) {
	//nolint:gosec // G124: Secure is set whenever the request used HTTPS; forcing it breaks plain-HTTP LAN installs.
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(expiry.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// isHTTPS reports whether the client reached the server over HTTPS, either
// directly or through a TLS-terminating reverse proxy.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// GetProfile returns the authenticated user's profile.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

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

// ChangePassword updates the authenticated user's password.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, errInvalidBody)

		return
	}

	err = h.userService.ChangePassword(r.Context(), api.GetUser(r).UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		respondPasswordError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{keyMessage: "password changed"})
}

// DeleteAccount permanently deletes the authenticated user's account.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	var req DeleteAccountRequest

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, errInvalidBody)

		return
	}

	err = h.userService.DeleteAccount(r.Context(), api.GetUser(r).UserID, req.Password)
	if err != nil {
		respondPasswordError(w, err)

		return
	}

	setAuthCookie(w, r, "", -time.Second)
	api.RespondJSON(w, http.StatusOK, map[string]string{keyMessage: "account deleted"})
}

// respondPasswordError maps errors from password-confirmed account actions.
func respondPasswordError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		api.RespondError(w, http.StatusBadRequest, errWrongPassword)
	case errors.Is(err, domain.ErrValidation):
		api.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		api.RespondError(w, http.StatusNotFound, domain.ErrUserNotFound.Error())
	default:
		api.RespondError(w, http.StatusInternalServerError, "request failed")
	}
}

func (h *Handler) generateToken(user *domain.User) (string, error) {
	now := time.Now()

	//nolint:modernize // golangci-lint's own typecheck rejects the go1.27 flattened-embedded-field literal it suggests here
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
