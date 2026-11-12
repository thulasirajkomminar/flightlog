package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

var errSessionStore = errors.New("session store down")

func TestMain(m *testing.M) {
	// Initialise the lazy global logger once, before parallel tests use it.
	logger.GetLogger()
	m.Run()
}

// mapComponent is the frontend file that hard-codes the map tile URL.
const mapComponent = "../../web/app/components/flight-map.tsx"

var tileHostRe = regexp.MustCompile(`"(https://[^/"]+)/[^"]*\{z\}`)

func contentSecurityPolicy(t *testing.T) string {
	t.Helper()

	handler := SecurityHeaders(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	return rec.Header().Get("Content-Security-Policy")
}

// TestCSPAllowsMapTileHost fails when the frontend's tile provider changes
// without the CSP being updated, which silently blanks the map in production.
func TestCSPAllowsMapTileHost(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(mapComponent)
	if err != nil {
		t.Fatalf("read map component: %v", err)
	}

	match := tileHostRe.FindSubmatch(source)
	if match == nil {
		t.Fatalf("no tile URL found in %s", mapComponent)
	}

	host := strings.ReplaceAll(string(match[1]), "{s}", "*")

	var connectSrc string

	for directive := range strings.SplitSeq(contentSecurityPolicy(t), ";") {
		if d := strings.TrimSpace(directive); strings.HasPrefix(d, "connect-src ") {
			connectSrc = d
		}
	}

	// The map fetches tiles, so the host must be allowed in connect-src.
	if !strings.Contains(" "+connectSrc+" ", " "+host+" ") {
		t.Errorf("CSP %q does not allow the map tile host %s used in %s", connectSrc, host, mapComponent)
	}
}

// fakeSessions answers session checks with a fixed result.
type fakeSessions struct {
	valid bool
	err   error
}

func (f fakeSessions) SessionValid(context.Context, string, time.Time) (bool, error) {
	return f.valid, f.err
}

func signedToken(t *testing.T, secret string) string {
	t.Helper()

	//nolint:modernize // golangci-lint's own typecheck rejects the go1.27 flattened-embedded-field literal it suggests here
	claims := &api.UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "user-1",
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return token
}

func TestAuthChecksTheSessionIsStillAccepted(t *testing.T) {
	t.Parallel()

	const secret = "test-secret"

	tests := []struct {
		name     string
		sessions fakeSessions
		token    string
		want     int
	}{
		{"valid session", fakeSessions{valid: true}, signedToken(t, secret), http.StatusOK},
		{"deleted account or changed password", fakeSessions{valid: false}, signedToken(t, secret), http.StatusUnauthorized},
		{"session check fails", fakeSessions{err: errSessionStore}, signedToken(t, secret), http.StatusInternalServerError},
		{"token signed with another secret", fakeSessions{valid: true}, signedToken(t, "other-secret"), http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := Auth(secret, tt.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if api.GetUser(r) == nil {
					t.Error("handler reached without a user in the context")
				}

				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/flights", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
