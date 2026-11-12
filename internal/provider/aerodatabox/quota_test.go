package aerodatabox

import (
	"net/http"
	"testing"
	"time"
)

const (
	requestsLimit     = "X-RateLimit-Requests-Limit"
	requestsRemaining = "X-RateLimit-Requests-Remaining"
)

func headers(values map[string]string) http.Header {
	h := http.Header{}
	for k, v := range values {
		h.Set(k, v)
	}

	return h
}

func TestQuotaAllowsBackgroundUntilReserve(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"no quota headers: unknown, allowed", nil, true},
		{"plenty left", map[string]string{requestsLimit: "600", requestsRemaining: "400"}, true},
		{"down to the reserve", map[string]string{requestsLimit: "600", requestsRemaining: "120"}, false},
		{"tightest of several limits counts", map[string]string{
			requestsLimit: "600", requestsRemaining: "500",
			"X-RateLimit-Api-Units-Limit": "300", "X-RateLimit-Api-Units-Remaining": "10",
		}, false},
		{"unparseable headers are ignored", map[string]string{requestsLimit: "many", requestsRemaining: "few"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var q quota

			q.observe(http.StatusOK, headers(tt.headers), now)

			if got := q.allowBackground(now); got != tt.want {
				t.Errorf("allowBackground = %v, want %v", got, tt.want)
			}

			if q.blocked(now) {
				t.Error("blocked after a 200 response")
			}
		})
	}
}

func TestQuotaBlocksAfterTooManyRequests(t *testing.T) {
	t.Parallel()

	now := time.Now()

	var q quota

	q.observe(http.StatusTooManyRequests, headers(map[string]string{"Retry-After": "120"}), now)

	if !q.blocked(now.Add(time.Minute)) || q.allowBackground(now.Add(time.Minute)) {
		t.Error("want calls blocked within the Retry-After period")
	}

	if q.blocked(now.Add(3 * time.Minute)) {
		t.Error("want calls allowed again after the Retry-After period")
	}

	var noHint quota

	noHint.observe(http.StatusTooManyRequests, nil, now)

	if !noHint.blocked(now.Add(10*time.Minute)) || noHint.blocked(now.Add(20*time.Minute)) {
		t.Error("want a 15 minute pause when the API gives no Retry-After")
	}
}
