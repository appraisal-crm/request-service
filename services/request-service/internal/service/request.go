package service

import (
	"context"
	"errors"
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
		return nil, err
	}
	return req, nil
}

func (s *requestService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *requestService) Update(ctx context.Context, req *domain.Request) (*domain.Request, error) {
	req.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *requestService) ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	allowed, ok := allowedTransitions[req.Status]
	if !ok || allowed != newStatus {
		return nil, ErrInvalidStatusTransition
	}

	req.Status = newStatus
	req.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *requestService) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	return s.repo.ListByClientID(ctx, clientID)
}
