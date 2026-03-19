package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/flight"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
	"github.com/thulasirajkomminar/flightlog/internal/provider"
	"github.com/thulasirajkomminar/flightlog/internal/user"
)

const (
	compressLevel  = 5
	requestTimeout = 30 * time.Second
)

// Dependencies holds router dependencies.
type Dependencies struct {
	FlightHandler   *flight.Handler
	ProviderHandler *provider.Handler
	UserHandler     *user.Handler

	JWTSecret             string
	Version               string
	WebFS                 fs.FS
	ScriptHashes          []string
	IPRequestsPerMinute   int
	UserRequestsPerMinute int
}

// SetupRouter creates and configures the chi router.
func SetupRouter(deps *Dependencies) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(Logger(logger.GetLogger()))
	r.Use(chimiddleware.Recoverer)
	r.Use(SecurityHeaders(deps.ScriptHashes))
	r.Use(chimiddleware.Compress(compressLevel))
	r.Use(chimiddleware.Timeout(requestTimeout))
	r.Use(RateLimitByIP(deps.IPRequestsPerMinute, 1*time.Minute))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"time":    time.Now().Unix(),
			"version": deps.Version,
		})
		if err != nil {
			logger.GetLogger().Error("failed to encode health response", zap.Error(err))
		}
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/auth/register", deps.UserHandler.Register)
		r.Post("/auth/login", deps.UserHandler.Login)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(Auth(deps.JWTSecret))
			r.Use(RateLimitByUser(deps.UserRequestsPerMinute, 1*time.Minute))

			r.Get("/auth/me", deps.UserHandler.GetProfile)
			r.Put("/auth/me", deps.UserHandler.UpdateProfile)
			r.Post("/auth/logout", deps.UserHandler.Logout)

			r.Get("/providers/{provider}/flights/search", deps.ProviderHandler.SearchFlights)
			r.Get("/flights/search", deps.FlightHandler.SearchFlights)
			r.Get("/flights/stats", deps.FlightHandler.GetStats)
			r.Get("/flights", deps.FlightHandler.ListFlights)
			r.Get("/flights/export", deps.FlightHandler.ExportFlights)
			r.Get("/flights/{id}", deps.FlightHandler.GetFlight)
			r.Post("/flights/{id}/add", deps.FlightHandler.AddFlight)
			r.Delete("/flights/{id}", deps.FlightHandler.DeleteFlight)
		})
	})

	if deps.WebFS != nil {
		addSPAHandler(r, deps.WebFS)
	}

	return r
}

func addSPAHandler(r *chi.Mux, webFS fs.FS) {
	fileServer := http.FileServerFS(webFS)

	handler := func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		if serveStaticFile(w, r, path, webFS, fileServer) {
			return
		}

		index, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	}

	r.Get("/", handler)
	r.Get("/*", handler)
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, path string, webFS fs.FS, fileServer http.Handler) bool {
	f, err := webFS.Open(path)
	if err != nil {
		return false
	}

	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		return false
	}

	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	fileServer.ServeHTTP(w, r)

	return true
}
