package service

import (
	"context"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/google/uuid"
)

type CreateInput struct {
	ClientID    uuid.UUID
	Email       string
	PhoneNumber string
	ObjectType  *domain.ObjectType
	Address     *string
}

type UpdateInput struct {
	InspectorID *uuid.UUID
	ObjectType  *domain.ObjectType
	Address     *string
}

type RequestService interface {
	Create(ctx context.Context, in CreateInput) (*domain.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*domain.Request, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error)
	ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error)
}
