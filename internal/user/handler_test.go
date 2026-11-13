package user

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/api"
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

	h, _ := newTestHandlerWith(t, true)

	return h
}

func newTestHandlerWith(t *testing.T, registrationEnabled bool) (*Handler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&domain.User{}, &domain.Flight{}, &domain.UserFlight{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewHandler(NewService(NewStore(db)), "test-secret", registrationEnabled), db
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

// asUser sends a JSON request to an authenticated handler as the given user.
func asUser(t *testing.T, handle http.HandlerFunc, userID, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req = req.WithContext(api.SetUser(req.Context(), &api.UserClaims{UserID: userID}))
	rec := httptest.NewRecorder()
	handle(rec, req)

	return rec
}

func registerUser(t *testing.T, h *Handler) *domain.User {
	t.Helper()

	u, err := h.userService.Register(t.Context(), "jane@example.com", "password123", "Jane")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	return u
}

func TestRegistrationCanBeDisabled(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandlerWith(t, false)

	rec := postJSON(t, h.Register, `{"email":"a@example.com","password":"password123","name":"A"}`, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("register status = %d, want 403", rec.Code)
	}

	rec = postJSON(t, h.Config, "", nil)
	if !strings.Contains(rec.Body.String(), `"registrationEnabled":false`) {
		t.Errorf("config = %s, want registrationEnabled false", rec.Body.String())
	}
}

func TestChangePassword(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	u := registerUser(t, h)

	steps := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"wrong current password is a 400, not a session-ending 401", `{"currentPassword":"nope","newPassword":"new-password-1"}`, http.StatusBadRequest},
		{"too short", `{"currentPassword":"password123","newPassword":"short"}`, http.StatusBadRequest},
		{"valid change", `{"currentPassword":"password123","newPassword":"new-password-1"}`, http.StatusOK},
	}

	for _, step := range steps {
		rec := asUser(t, h.ChangePassword, u.ID, step.body)
		if rec.Code != step.wantStatus {
			t.Errorf("%s: status = %d, want %d (%s)", step.name, rec.Code, step.wantStatus, rec.Body.String())
		}
	}

	rec := postJSON(t, h.Login, `{"email":"jane@example.com","password":"password123"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password: status = %d, want 401", rec.Code)
	}

	rec = postJSON(t, h.Login, `{"email":"jane@example.com","password":"new-password-1"}`, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("login with new password: status = %d, want 200", rec.Code)
	}
}

func TestDeleteAccountRemovesUserAndCollection(t *testing.T) {
	t.Parallel()

	h, db := newTestHandlerWith(t, true)
	u := registerUser(t, h)

	links := []domain.UserFlight{
		{ID: "uf-1", UserID: u.ID, FlightID: "f-1"},
		{ID: "uf-2", UserID: "someone-else", FlightID: "f-1"},
	}

	err := db.Create(&links).Error
	if err != nil {
		t.Fatalf("create links: %v", err)
	}

	// An import job as the importer stores it, for this user and another one.
	jobs := []string{
		"CREATE TABLE import_jobs (user_id text PRIMARY KEY, entries text)",
		"INSERT INTO import_jobs (user_id, entries) VALUES ('" + u.ID + "', '[]'), ('someone-else', '[]')",
	}
	for _, stmt := range jobs {
		err = db.Exec(stmt).Error
		if err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	rec := asUser(t, h.DeleteAccount, u.ID, `{"password":"wrong"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete with wrong password: status = %d, want 400", rec.Code)
	}

	rec = asUser(t, h.DeleteAccount, u.ID, `{"password":"password123"}`)
	if rec.Code != http.StatusOK || authCookie(t, rec).MaxAge >= 0 {
		t.Fatalf("delete: status = %d, want 200 with the auth cookie cleared", rec.Code)
	}

	var users, remaining, jobsLeft int64

	db.Model(&domain.User{}).Count(&users)
	db.Model(&domain.UserFlight{}).Count(&remaining)
	db.Table("import_jobs").Count(&jobsLeft)

	if users != 0 || remaining != 1 || jobsLeft != 1 {
		t.Errorf("users = %d, links = %d, import jobs = %d; want 0 users and only the other user's link and job", users, remaining, jobsLeft)
	}
}

func TestSessionsEndOnPasswordChangeAndAccountDeletion(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	u := registerUser(t, h)
	ctx := t.Context()
	loggedInAt := time.Now().Add(-time.Hour)

	valid, err := h.userService.SessionValid(ctx, u.ID, loggedInAt)
	if err != nil || !valid {
		t.Fatalf("session before any change: valid = %v, err = %v; want valid", valid, err)
	}

	rec := asUser(t, h.ChangePassword, u.ID, `{"currentPassword":"password123","newPassword":"new-password-1"}`)
	if rec.Code != http.StatusOK || authCookie(t, rec).Value == "" {
		t.Fatalf("change password: status %d, want 200 with a fresh session cookie", rec.Code)
	}

	valid, _ = h.userService.SessionValid(ctx, u.ID, loggedInAt)
	if valid {
		t.Error("a session opened before the password change is still valid")
	}

	valid, _ = h.userService.SessionValid(ctx, u.ID, time.Now().Add(time.Second))
	if !valid {
		t.Error("a session opened after the password change is not valid")
	}

	rec = asUser(t, h.DeleteAccount, u.ID, `{"password":"new-password-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete account: status %d", rec.Code)
	}

	valid, err = h.userService.SessionValid(ctx, u.ID, time.Now().Add(time.Second))
	if err != nil || valid {
		t.Errorf("session of a deleted account: valid = %v, err = %v; want invalid", valid, err)
	}
}

func TestResetPasswordIssuesAWorkingRandomPassword(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	u := registerUser(t, h)
	ctx := t.Context()

	password, err := h.userService.ResetPassword(ctx, "JANE@example.com")
	if err != nil || len(password) < 16 {
		t.Fatalf("ResetPassword = %q, err %v; want a password of at least 16 characters", password, err)
	}

	_, err = h.userService.Authenticate(ctx, u.Email, password)
	if err != nil {
		t.Errorf("login with the new password: %v", err)
	}

	_, err = h.userService.Authenticate(ctx, u.Email, "password123")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("login with the old password: err = %v, want invalid credentials", err)
	}

	valid, _ := h.userService.SessionValid(ctx, u.ID, time.Now().Add(-time.Hour))
	if valid {
		t.Error("a session opened before the reset is still valid")
	}

	_, err = h.userService.ResetPassword(ctx, "nobody@example.com")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("reset for an unknown email: err = %v, want user not found", err)
	}
}
