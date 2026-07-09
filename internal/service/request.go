package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/appraisal-crm/request-service/internal/repository"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")
var ErrInvalidStatusTransition = errors.New("invalid status transition")
var ErrConflict = errors.New("concurrent modification conflict")

var allowedTransitions = map[domain.Status]domain.Status{
	domain.StatusNew:                 domain.StatusInProgress,
	domain.StatusInProgress:          domain.StatusInspectionScheduled,
	domain.StatusInspectionScheduled: domain.StatusInspectionCompleted,
	domain.StatusInspectionCompleted: domain.StatusAppraisal,
	domain.StatusAppraisal:           domain.StatusReportSent,
	domain.StatusReportSent:          domain.StatusClosed,
}

type requestService struct {
	repo repository.RequestRepository
}

func NewRequestService(repo repository.RequestRepository) RequestService {
	return &requestService{repo: repo}
}

func (s *requestService) Create(ctx context.Context, in CreateInput) (*domain.Request, error) {
	now := time.Now()
	req := &domain.Request{
		ID:          uuid.New(),
		ClientID:    in.ClientID,
		Email:       in.Email,
		PhoneNumber: in.PhoneNumber,
		ObjectType:  in.ObjectType,
		Address:     in.Address,
		Status:      domain.StatusNew,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to create request", "error", err, "client_id", in.ClientID)
		return nil, err
	}
	slog.InfoContext(ctx, "request created", "request_id", req.ID, "client_id", in.ClientID)
	return req, nil
}

func (s *requestService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		slog.ErrorContext(ctx, "failed to get request", "error", err, "request_id", id)
		return nil, err
	}
	return req, nil
}

func (s *requestService) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*domain.Request, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if in.InspectorID != nil {
		req.InspectorID = in.InspectorID
	}
	if in.ObjectType != nil {
		req.ObjectType = in.ObjectType
	}
	if in.Address != nil {
		req.Address = in.Address
	}

	prevUpdatedAt := req.UpdatedAt
	req.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, req, prevUpdatedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			slog.WarnContext(ctx, "concurrent update detected", "request_id", id)
			return nil, ErrConflict
		}
		slog.ErrorContext(ctx, "failed to update request", "error", err, "request_id", id)
		return nil, err
	}
	slog.InfoContext(ctx, "request updated", "request_id", id)
	return req, nil
}

func (s *requestService) ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	allowed, ok := allowedTransitions[req.Status]
	if !ok || allowed != newStatus {
		slog.WarnContext(ctx, "invalid status transition", "request_id", id, "from", req.Status, "to", newStatus)
		return nil, ErrInvalidStatusTransition
	}

	oldStatus := req.Status
	updatedAt := time.Now()

	if err := s.repo.ChangeStatus(ctx, id, oldStatus, newStatus, updatedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			slog.WarnContext(ctx, "concurrent status change detected", "request_id", id, "from", oldStatus, "to", newStatus)
			return nil, ErrConflict
		}
		slog.ErrorContext(ctx, "failed to update request status", "error", err, "request_id", id)
		return nil, err
	}

	req.Status = newStatus
	req.UpdatedAt = updatedAt

	slog.InfoContext(ctx, "request status changed", "request_id", id, "from", oldStatus, "to", newStatus)
	return req, nil
}

func (s *requestService) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	requests, err := s.repo.ListByClientID(ctx, clientID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list requests", "error", err, "client_id", clientID)
		return nil, err
	}
	return requests, nil
}

func (s *requestService) ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error) {
	requests, err := s.repo.ListAll(ctx, limit, offset)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list all requests", "error", err)
		return nil, err
	}
	return requests, nil
}
