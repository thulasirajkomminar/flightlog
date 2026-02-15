package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

// MaxBodySize is the maximum request body size (1 MB).
const MaxBodySize = 1 << 20

// RespondJSON writes a JSON response with the given status code.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			logger.GetLogger().Error("failed to encode response", zap.Error(err))
		}
	}
}

// RespondError writes a JSON error response.
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"error": message})
}
