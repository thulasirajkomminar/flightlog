package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

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
