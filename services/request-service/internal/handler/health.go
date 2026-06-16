package handler

import (
	"encoding/json"
	"net/http"
)

// Health godoc
// @Summary     Health check
// @Tags        system
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /health [get]
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
