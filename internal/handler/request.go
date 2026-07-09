package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/appraisal-crm/request-service/internal/middleware"
	"github.com/appraisal-crm/request-service/internal/service"
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var dto createRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(dto); err != nil {
		respondError(w, http.StatusBadRequest, firstValidationError(err))
		return
	}

	req, err := h.svc.Create(r.Context(), service.CreateInput{
		ClientID:    clientID,
		Email:       dto.Email,
		PhoneNumber: dto.PhoneNumber,
		ObjectType:  dto.ObjectType,
		Address:     dto.Address,
	})
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
		if errors.Is(err, service.ErrNotFound) {
			respondError(w, http.StatusNotFound, "request not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get request")
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
// @Failure     409 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /requests/{id} [patch]
func (h *requestHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var dto updateRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(dto); err != nil {
		respondError(w, http.StatusBadRequest, firstValidationError(err))
		return
	}

	updated, err := h.svc.Update(r.Context(), id, service.UpdateInput{
		InspectorID: dto.InspectorID,
		ObjectType:  dto.ObjectType,
		Address:     dto.Address,
	})
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(w, http.StatusNotFound, "request not found")
			return
		}
		if errors.Is(err, service.ErrConflict) {
			respondError(w, http.StatusConflict, "request was modified concurrently, please retry")
			return
		}
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var dto changeStatusDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(dto); err != nil {
		respondError(w, http.StatusBadRequest, firstValidationError(err))
		return
	}

	req, err := h.svc.ChangeStatus(r.Context(), id, dto.Status)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(w, http.StatusNotFound, "request not found")
			return
		}
		if errors.Is(err, service.ErrConflict) {
			respondError(w, http.StatusConflict, "request was modified concurrently, please retry")
			return
		}
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			respondError(w, http.StatusUnprocessableEntity, "invalid status transition")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to change status")
		return
	}

	respondJSON(w, http.StatusOK, req)
}

// ListRequests godoc
// @Summary     List requests
// @Description Clients see their own requests. Appraiser/admin with client_id param filter by client. Appraiser/admin without client_id return all requests (paginated).
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       client_id query string false "Filter by client ID (appraiser/admin only)"
// @Param       page      query int    false "Page number (default 1, appraiser/admin list-all only)"
// @Param       limit     query int    false "Page size (default 20, max 100, appraiser/admin list-all only)"
// @Success     200 {array}  domain.Request
// @Success     200 {object} listAllResponse
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

	rawClientID := r.URL.Query().Get("client_id")
	if rawClientID != "" {
		clientID, err := uuid.Parse(rawClientID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid client_id query param")
			return
		}
		requests, err := h.svc.ListByClientID(r.Context(), clientID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list requests")
			return
		}
		respondJSON(w, http.StatusOK, requests)
		return
	}

	page := parseIntParam(r.URL.Query().Get("page"), 1)
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	requests, err := h.svc.ListAll(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	respondJSON(w, http.StatusOK, listAllResponse{
		Data:  requests,
		Page:  page,
		Limit: limit,
	})
}

func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
