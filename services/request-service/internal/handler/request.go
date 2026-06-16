package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/middleware"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type requestHandler struct {
	svc service.RequestService
}

func newRequestHandler(svc service.RequestService) *requestHandler {
	return &requestHandler{svc: svc}
}

// Create godoc
// @Summary     Create a request
// @Tags        requests
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body createRequestDTO true "Request data"
// @Success     201 {object} domain.Request
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /requests [post]
func (h *requestHandler) Create(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto createRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req, err := h.svc.Create(r.Context(), clientID, dto.ObjectType, dto.Address)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	respondJSON(w, http.StatusCreated, req)
}

// GetByID godoc
// @Summary     Get request by ID
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Request ID"
// @Success     200 {object} domain.Request
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     403 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /requests/{id} [get]
func (h *requestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	req, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}

	if middleware.HasRole(r.Context(), "client") {
		userID, _ := middleware.UserIDFromContext(r.Context())
		if req.ClientID != userID {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	respondJSON(w, http.StatusOK, req)
}

// Update godoc
// @Summary     Update request fields
// @Tags        requests
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Request ID"
// @Param       body body updateRequestDTO true "Fields to update"
// @Success     200 {object} domain.Request
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /requests/{id} [patch]
func (h *requestHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var dto updateRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}

	req.InspectorID = dto.InspectorID
	req.ObjectType = dto.ObjectType
	req.Address = dto.Address

	updated, err := h.svc.Update(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update request")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// ChangeStatus godoc
// @Summary     Change request status
// @Tags        requests
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Request ID"
// @Param       body body changeStatusDTO true "New status"
// @Success     200 {object} domain.Request
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     422 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /requests/{id}/status [patch]
func (h *requestHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var dto changeStatusDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req, err := h.svc.ChangeStatus(r.Context(), id, dto.Status)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			respondError(w, http.StatusUnprocessableEntity, "invalid status transition")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to change status")
		return
	}

	respondJSON(w, http.StatusOK, req)
}

// ListByClientID godoc
// @Summary     List requests by client ID
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       client_id query string false "Client ID (appraiser/admin only)"
// @Success     200 {array} domain.Request
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /requests [get]
func (h *requestHandler) ListByClientID(w http.ResponseWriter, r *http.Request) {
	if middleware.HasRole(r.Context(), "client") {
		userID, _ := middleware.UserIDFromContext(r.Context())
		requests, err := h.svc.ListByClientID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list requests")
			return
		}
		respondJSON(w, http.StatusOK, requests)
		return
	}

	clientID, err := uuid.Parse(r.URL.Query().Get("client_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing or invalid client_id query param")
		return
	}

	requests, err := h.svc.ListByClientID(r.Context(), clientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	respondJSON(w, http.StatusOK, requests)
}
