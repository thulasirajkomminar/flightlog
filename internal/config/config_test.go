package config

import "testing"

func TestWeakSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		{"placeholder from .env.example", "your_jwt_secret_here", true},
		{"short", "hunter2", true},
		{"openssl rand -base64 32", "0Vq1bJ9m3o0b3c1Yb1yT1gkq2h0q3mVb8mD1v0mQz1E=", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := (AuthConfig{JWTSecret: tt.secret}).WeakSecret(); got != tt.want {
				t.Errorf("WeakSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
