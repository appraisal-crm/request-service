package handler

import (
	"net/http"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/httputil"
)

type errorResponse = httputil.ErrorResponse

func respondError(w http.ResponseWriter, status int, message string) {
	httputil.RespondError(w, status, message)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	httputil.RespondJSON(w, status, data)
}
