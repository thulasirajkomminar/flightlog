package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/user"
)

func newTestRouter(t *testing.T, ping func(context.Context) error) http.Handler {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&domain.User{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	users := user.NewService(user.NewStore(db))

	return SetupRouter(&Dependencies{
		UserHandler: user.NewHandler(users, "test-secret", true),
		Sessions:    users,
		JWTSecret:   "test-secret",
		Ping:        ping,
	})
}

func request(t *testing.T, router http.Handler, method, path, body string) int {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec.Code
}

func TestLoginAttemptsAreThrottled(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, nil)

	const wrongLogin = `{"email":"nobody@example.com","password":"wrong-password"}`

	for i := range authAttemptsPerMinute {
		if code := request(t, router, http.MethodPost, "/api/v1/auth/login", wrongLogin); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, code)
		}
	}

	if code := request(t, router, http.MethodPost, "/api/v1/auth/login", wrongLogin); code != http.StatusTooManyRequests {
		t.Errorf("attempt %d: status = %d, want 429", authAttemptsPerMinute+1, code)
	}

	// The tight limit is for signing in only; the rest of the API is unaffected.
	if code := request(t, router, http.MethodGet, "/api/v1/auth/config", ""); code != http.StatusOK {
		t.Errorf("other endpoint after the login limit: status = %d, want 200", code)
	}
}

func TestHealthReflectsTheDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ping func(context.Context) error
		want int
	}{
		{"database reachable", func(context.Context) error { return nil }, http.StatusOK},
		{"database down", func(context.Context) error { return errSessionStore }, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if code := request(t, newTestRouter(t, tt.ping), http.MethodGet, "/health", ""); code != tt.want {
				t.Errorf("status = %d, want %d", code, tt.want)
			}
		})
	}
}
