package aerodatabox

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	rateLimitPrefix     = "x-ratelimit-"
	remainingSuffix     = "-remaining"
	limitSuffix         = "-limit"
	headerRetryAfter    = "Retry-After"
	defaultQuotaBackoff = 15 * time.Minute
	maxQuotaBackoff     = 24 * time.Hour
	// backgroundReservePercent of the quota is kept for searches people start
	// themselves; background work stops once less than this is left.
	backgroundReservePercent = 20
	percent                  = 100
)

// quota tracks what the API says about the remaining request quota, so the
// app can stop spending it on background work when it runs low and stop
// calling altogether while the API is refusing requests.
//
// RapidAPI reports quotas in X-RateLimit-<name>-Limit / -Remaining header
// pairs. When several pairs are present the tightest one counts.
type quota struct {
	mu           sync.Mutex
	known        bool
	limit        int
	remaining    int
	blockedUntil time.Time
}

// observe records the status and quota headers of a response. A 429 that
// survived the transport's retries means the quota is exhausted, so calls are
// paused.
func (q *quota) observe(status int, h http.Header, now time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if pair, ok := tightestLimit(h); ok {
		q.known, q.limit, q.remaining = true, pair.limit, pair.remaining
	}

	if status == http.StatusTooManyRequests {
		q.blockedUntil = now.Add(retryAfter(h))
	}
}

// snapshot returns the last reported quota for display.
func (q *quota) snapshot(now time.Time) domain.ProviderQuota {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := domain.ProviderQuota{Known: q.known, Limit: q.limit, Remaining: q.remaining}

	if now.Before(q.blockedUntil) {
		until := q.blockedUntil
		out.PausedUntil = &until
	}

	return out
}

// blocked reports whether the API is currently refusing requests.
func (q *quota) blocked(now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return now.Before(q.blockedUntil)
}

// allowBackground reports whether non-essential calls may be made: not while
// blocked, and not once the remaining quota is down to the reserve. An
// unknown quota allows them.
func (q *quota) allowBackground(now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if now.Before(q.blockedUntil) {
		return false
	}

	if !q.known || q.limit <= 0 {
		return true
	}

	return q.remaining*percent > q.limit*backgroundReservePercent
}

// limitPair is one quota as reported by a Limit/Remaining header pair.
type limitPair struct {
	limit, remaining int
}

// tighterThan reports whether p has a smaller share of its quota left than other.
func (p limitPair) tighterThan(other limitPair) bool {
	return p.remaining*other.limit < other.remaining*p.limit
}

// tightestLimit finds the limit/remaining header pair with the smallest share
// of quota left.
func tightestLimit(h http.Header) (limitPair, bool) {
	var (
		best  limitPair
		found bool
	)

	for name := range h {
		pair, ok := parseLimitPair(h, name)
		if ok && (!found || pair.tighterThan(best)) {
			best, found = pair, true
		}
	}

	return best, found
}

// parseLimitPair reads the quota named by a "-Remaining" header and its
// matching "-Limit" header.
func parseLimitPair(h http.Header, name string) (limitPair, bool) {
	lower := strings.ToLower(name)
	if !strings.HasPrefix(lower, rateLimitPrefix) || !strings.HasSuffix(lower, remainingSuffix) {
		return limitPair{}, false
	}

	remaining, err := strconv.Atoi(h.Get(name))
	if err != nil {
		return limitPair{}, false
	}

	limit, err := strconv.Atoi(h.Get(strings.TrimSuffix(lower, remainingSuffix) + limitSuffix))
	if err != nil || limit <= 0 {
		return limitPair{}, false
	}

	return limitPair{limit: limit, remaining: remaining}, true
}

// retryAfter reads how long the API asks callers to wait, bounded to a
// sensible range.
func retryAfter(h http.Header) time.Duration {
	seconds, err := strconv.Atoi(h.Get(headerRetryAfter))
	if err != nil || seconds <= 0 {
		return defaultQuotaBackoff
	}

	return min(time.Duration(seconds)*time.Second, maxQuotaBackoff)
}
