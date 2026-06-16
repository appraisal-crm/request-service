package handler

import (
	"encoding/json"
	"errors"
	"net/http"

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

func (h *requestHandler) Create(w http.ResponseWriter, r *http.Request) {
	clientID, err := uuid.Parse(r.Header.Get("X-Client-ID"))
	if err != nil {
		http.Error(w, "missing or invalid X-Client-ID header", http.StatusBadRequest)
		return
	}

	var dto createRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req, err := h.svc.Create(r.Context(), clientID, dto.ObjectType, dto.Address)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *requestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	req, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func (h *requestHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var dto updateRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	req.InspectorID = dto.InspectorID
	req.ObjectType = dto.ObjectType
	req.Address = dto.Address

	updated, err := h.svc.Update(r.Context(), req)
	if err != nil {
		http.Error(w, "failed to update request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *requestHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var dto changeStatusDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req, err := h.svc.ChangeStatus(r.Context(), id, dto.Status)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "invalid status transition", http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, "failed to change status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func (h *requestHandler) ListByClientID(w http.ResponseWriter, r *http.Request) {
	clientID, err := uuid.Parse(r.URL.Query().Get("client_id"))
	if err != nil {
		http.Error(w, "missing or invalid client_id query param", http.StatusBadRequest)
		return
	}

	requests, err := h.svc.ListByClientID(r.Context(), clientID)
	if err != nil {
		http.Error(w, "failed to list requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}
