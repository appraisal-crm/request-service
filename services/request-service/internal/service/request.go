package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidStatusTransition = errors.New("invalid status transition")

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

func (s *requestService) Create(ctx context.Context, clientID uuid.UUID, objectType *domain.ObjectType, address *string) (*domain.Request, error) {
	now := time.Now()
	req := &domain.Request{
		ID:         uuid.New(),
		ClientID:   clientID,
		ObjectType: objectType,
		Address:    address,
		Status:     domain.StatusNew,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to create request", "error", err, "client_id", clientID)
		return nil, err
	}
	slog.InfoContext(ctx, "request created", "request_id", req.ID, "client_id", clientID)
	return req, nil
}

func (s *requestService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *requestService) Update(ctx context.Context, req *domain.Request) (*domain.Request, error) {
	req.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to update request", "error", err, "request_id", req.ID)
		return nil, err
	}
	slog.InfoContext(ctx, "request updated", "request_id", req.ID)
	return req, nil
}

func (s *requestService) ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	allowed, ok := allowedTransitions[req.Status]
	if !ok || allowed != newStatus {
		slog.WarnContext(ctx, "invalid status transition", "request_id", id, "from", req.Status, "to", newStatus)
		return nil, ErrInvalidStatusTransition
	}

	req.Status = newStatus
	req.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to update request status", "error", err, "request_id", id)
		return nil, err
	}
	slog.InfoContext(ctx, "request status changed", "request_id", id, "from", allowed, "to", newStatus)
	return req, nil
}

func (s *requestService) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	return s.repo.ListByClientID(ctx, clientID)
}
