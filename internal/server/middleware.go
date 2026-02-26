// Package server configures HTTP routing and middleware.
package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

var (
	errUnexpectedSignMethod = errors.New("unexpected signing method")
	errInvalidClaims        = errors.New("invalid claims")
)

// Auth returns JWT validation middleware.
func Auth(jwtSecret string) func(next http.Handler) http.Handler {
	secret := []byte(jwtSecret)
	log := logger.NewComponentLogger("auth")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				log.Warn("missing auth token",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)

				return
			}

			claims, err := validateJWT(tokenStr, secret)
			if err != nil {
				log.Warn("invalid auth token",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)

				return
			}

			ctx := api.SetUser(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	c, err := r.Cookie("flightlog_token")
	if err == nil {
		return c.Value
	}

	h := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(h, "Bearer "); ok {
		return token
	}

	return ""
}

func validateJWT(tokenStr string, secret []byte) (*api.UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &api.UserClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: %v", errUnexpectedSignMethod, t.Header["alg"])
			}

			return secret, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*api.UserClaims)
	if !ok || !token.Valid {
		return nil, errInvalidClaims
	}

	return claims, nil
}

// Logger returns request logging middleware.
func Logger(log *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			next.ServeHTTP(ww, r)

			log.Info("request",
				zap.String("request_id", middleware.GetReqID(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("duration", time.Since(start)),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}

// RateLimitByIP returns per-IP rate limiting middleware.
func RateLimitByIP(requests int, window time.Duration) func(next http.Handler) http.Handler {
	return httprate.LimitByIP(requests, window)
}

// RateLimitByUser returns per-user rate limiting middleware.
func RateLimitByUser(requests int, window time.Duration) func(next http.Handler) http.Handler {
	return httprate.Limit(requests, window, httprate.WithKeyFuncs(api.UserRateKey))
}

// SecurityHeaders adds browser security headers to all responses.
func SecurityHeaders(scriptHashes []string) func(http.Handler) http.Handler {
	scriptSrc := "'self'"
	if len(scriptHashes) > 0 {
		scriptSrc = "'self' " + strings.Join(scriptHashes, " ")
	}

	csp := "default-src 'self'; " +
		"script-src " + scriptSrc + "; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https://*.basemaps.cartocdn.com; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", csp)

			next.ServeHTTP(w, r)
		})
	}
}
