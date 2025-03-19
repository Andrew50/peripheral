package health

import (
	"net/http"
)

// Handler returns an HTTP handler for health checks
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
