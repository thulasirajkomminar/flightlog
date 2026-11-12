package user

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

func TestMain(m *testing.M) {
	// Initialise the lazy global logger once, before parallel tests use it.
	logger.GetLogger()
	m.Run()
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&domain.User{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewHandler(NewService(NewStore(db)), "test-secret")
}

func postJSON(t *testing.T, handle http.HandlerFunc, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handle(rec, req)

	return rec
}

func authCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == authCookieName {
			return c
		}
	}

	t.Fatalf("no %s cookie in response (status %d: %s)", authCookieName, rec.Code, rec.Body.String())

	return nil
}

func TestAuthCookieSecureFollowsRequestScheme(t *testing.T) {
	t.Parallel()

	const body = `{"email":"a@example.com","password":"password123","name":"A"}`

	tests := []struct {
		name       string
		headers    map[string]string
		wantSecure bool
	}{
		{"plain HTTP gets a cookie the browser will keep", nil, false},
		{"HTTPS behind a proxy gets a Secure cookie", map[string]string{"X-Forwarded-Proto": "https"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newTestHandler(t)
			c := authCookie(t, postJSON(t, h.Register, body, tt.headers))

			if c.Secure != tt.wantSecure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.wantSecure)
			}

			if !c.HttpOnly || c.Value == "" {
				t.Errorf("cookie = %+v, want HttpOnly with a token", c)
			}
		})
	}
}

func TestEmailIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)

	rec := postJSON(t, h.Register, `{"email":"Jane@Example.com","password":"password123","name":"Jane"}`, nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"email":"jane@example.com"`) {
		t.Fatalf("register: status %d, body %s; want 201 with a lower-cased email", rec.Code, rec.Body.String())
	}

	rec = postJSON(t, h.Register, `{"email":"jane@example.com","password":"password123","name":"Jane 2"}`, nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("registering the same address in another case: status %d, want 409", rec.Code)
	}

	rec = postJSON(t, h.Login, `{"email":"JANE@EXAMPLE.COM","password":"password123"}`, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("login with different case: status %d, want 200", rec.Code)
	}
}

func TestLoginFindsLegacyMixedCaseAccount(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)

	// An account stored before normalisation keeps its mixed-case address.
	legacy, err := h.userService.Register(t.Context(), "old@example.com", "password123", "Old")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	legacy.Email = "Old@Example.com"

	err = h.userService.repo.Update(t.Context(), legacy)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	rec := postJSON(t, h.Login, `{"email":"old@example.com","password":"password123"}`, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("login to legacy account: status %d, want 200", rec.Code)
	}
}
