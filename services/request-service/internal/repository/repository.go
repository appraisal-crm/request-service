package repository

import (
	"context"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
)

type RequestRepository interface {
	Create(ctx context.Context, req *domain.Request) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error)
	Update(ctx context.Context, req *domain.Request) error
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error)
}
