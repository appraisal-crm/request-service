package service

import (
	"context"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
)

type RequestService interface {
	Create(ctx context.Context, clientID uuid.UUID, objectType *domain.ObjectType, address *string) (*domain.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error)
	Update(ctx context.Context, req *domain.Request) (*domain.Request, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error)
}
