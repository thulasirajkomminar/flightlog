// Package server configures HTTP routing and middleware.
package server

import (
	"context"
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

const logKeyPath = "path"

var (
	errUnexpectedSignMethod = errors.New("unexpected signing method")
	errInvalidClaims        = errors.New("invalid claims")
)

// SessionChecker decides whether a correctly signed token is still accepted.
type SessionChecker interface {
	// SessionValid reports whether a token issued at issuedAt for userID is
	// still good, for example that the account has not been deleted since.
	SessionValid(ctx context.Context, userID string, issuedAt time.Time) (bool, error)
}

// Auth returns JWT validation middleware. A token must be correctly signed
// and still accepted by sessions.
func Auth(jwtSecret string, sessions SessionChecker) func(next http.Handler) http.Handler {
	secret := []byte(jwtSecret)
	log := logger.NewComponentLogger("auth")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				log.Warn("missing auth token",
					zap.String(logKeyPath, r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)

				return
			}

			claims, err := validateJWT(tokenStr, secret)
			if err != nil {
				log.Warn("invalid auth token",
					zap.String(logKeyPath, r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)

				return
			}

			valid, err := sessions.SessionValid(r.Context(), claims.UserID, issuedAt(claims))
			if err != nil {
				log.Error("session check failed", err, zap.String(logKeyPath, r.URL.Path))
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)

				return
			}

			if !valid {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)

				return
			}

			ctx := api.SetUser(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// issuedAt returns when a token was issued; a token without that claim counts
// as older than any cut-off.
func issuedAt(claims *api.UserClaims) time.Time {
	if claims.IssuedAt == nil {
		return time.Time{}
	}

	return claims.IssuedAt.Time
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
				zap.String(logKeyPath, r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("duration", time.Since(start)),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}

// ClientIP resolves the client IP for downstream middleware such as
// RateLimitByIP. With trusted proxy CIDRs configured it reads the client IP
// from X-Forwarded-For, skipping trusted hops; otherwise it uses the
// connection's remote address, which is correct when Flightlog is exposed
// directly rather than behind a reverse proxy.
func ClientIP(trustedProxies []string) func(http.Handler) http.Handler {
	if len(trustedProxies) > 0 {
		return middleware.ClientIPFromXFF(trustedProxies...)
	}

	return middleware.ClientIPFromRemoteAddr
}

// RateLimitByIP returns per-IP rate limiting middleware. It keys off the IP
// resolved by the ClientIP middleware, which must be installed upstream;
// requests without a resolved IP share one global bucket.
func RateLimitByIP(requests int, window time.Duration) func(next http.Handler) http.Handler {
	return httprate.LimitBy(requests, window, func(r *http.Request) (string, error) {
		return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
	})
}

// RateLimitByUser returns per-user rate limiting middleware.
func RateLimitByUser(requests int, window time.Duration) func(next http.Handler) http.Handler {
	return httprate.LimitBy(requests, window, api.UserRateKey)
}

// mapTileHost serves the dashboard map's basemap tiles. It must match the tile
// URL in web/app/components/flight-map.tsx.
const mapTileHost = "https://server.arcgisonline.com"

// SecurityHeaders adds browser security headers to all responses.
func SecurityHeaders(scriptHashes []string) func(http.Handler) http.Handler {
	scriptSrc := "'self'"
	if len(scriptHashes) > 0 {
		scriptSrc = "'self' " + strings.Join(scriptHashes, " ")
	}

	csp := "default-src 'self'; " +
		"script-src " + scriptSrc + "; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self' data:; " +
		// The map fetches its basemap tiles from the tile host.
		"connect-src 'self' " + mapTileHost + "; " +
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
