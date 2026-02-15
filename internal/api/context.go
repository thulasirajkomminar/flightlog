// Package api provides shared HTTP utilities for handlers.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userCtxKey contextKey = "user"

// UserClaims represents JWT claims.
type UserClaims struct {
	jwt.RegisteredClaims

	UserID string `json:"userId"`
	Email  string `json:"email"`
}

// errNoUserInContext indicates no user was found in context.
var errNoUserInContext = errors.New("no user in context")

// SetUser stores user claims in the request context.
func SetUser(ctx context.Context, claims *UserClaims) context.Context {
	return context.WithValue(ctx, userCtxKey, claims)
}

// GetUser extracts user claims from the request context.
func GetUser(r *http.Request) *UserClaims {
	claims, ok := r.Context().Value(userCtxKey).(*UserClaims)
	if !ok {
		return nil
	}

	return claims
}

// UserRateKey extracts the user ID for rate limiting.
func UserRateKey(r *http.Request) (string, error) {
	if claims := GetUser(r); claims != nil {
		return claims.UserID, nil
	}

	return "", errNoUserInContext
}
