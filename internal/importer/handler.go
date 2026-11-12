package importer

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/thulasirajkomminar/flightlog/internal/api"
)

const (
	maxUploadSize    = 5 << 20 // 5 MB
	maxImportFlights = 2000
)

// Handler exposes import endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a new import handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Preview parses an uploaded file and returns import stats.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	_, entries := h.parseUpload(w, r)
	if entries == nil {
		return
	}

	api.RespondJSON(w, http.StatusOK, h.service.Preview(entries))
}

// Import parses an uploaded file and starts importing it in the background.
// It answers 202 with the new job; progress is served by Status.
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	user, entries := h.parseUpload(w, r)
	if entries == nil {
		return
	}

	if len(entries) > maxImportFlights {
		api.RespondError(w, http.StatusBadRequest, fmt.Sprintf("too many flights: %d (max %d)", len(entries), maxImportFlights))

		return
	}

	enrich := r.URL.Query().Get("enrich") == "true"
	source := r.URL.Query().Get("source")

	// The import outlives this request, so detach it from the request context.
	job, err := h.service.StartImport(context.WithoutCancel(r.Context()), user.UserID, entries, source, enrich)
	if err != nil {
		if errors.Is(err, ErrImportRunning) {
			api.RespondError(w, http.StatusConflict, err.Error())

			return
		}

		api.RespondError(w, http.StatusInternalServerError, "failed to start import")

		return
	}

	api.RespondJSON(w, http.StatusAccepted, job)
}

// Status returns the user's running or most recently finished import, or 204
// when there is none.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	job, ok := h.service.ImportStatus(r.Context(), api.GetUser(r).UserID)
	if !ok {
		w.WriteHeader(http.StatusNoContent)

		return
	}

	api.RespondJSON(w, http.StatusOK, job)
}

// Sources returns supported import sources.
func (h *Handler) Sources(w http.ResponseWriter, _ *http.Request) {
	api.RespondJSON(w, http.StatusOK, map[string]any{
		"sources": h.service.Sources(),
	})
}

// parseUpload extracts the user, parses the uploaded CSV, and returns entries.
// Returns nil on failure (error response already written).
func (h *Handler) parseUpload(w http.ResponseWriter, r *http.Request) (*api.UserClaims, []ImportEntry) {
	user := api.GetUser(r)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	file, _, err := r.FormFile("file")
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "file is required")

		return nil, nil
	}

	defer func() { _ = file.Close() }()

	source := r.URL.Query().Get("source")

	entries, err := h.service.Parse(source, file)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, err.Error())

		return nil, nil
	}

	if len(entries) == 0 {
		api.RespondError(w, http.StatusBadRequest, "no valid flights found in file")

		return nil, nil
	}

	return user, entries
}
