package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("concurrent modification conflict")

type RequestRepository interface {
	Create(ctx context.Context, req *domain.Request) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error)
	Update(ctx context.Context, req *domain.Request) error
	ChangeStatus(ctx context.Context, id uuid.UUID, oldStatus, newStatus domain.Status, updatedAt time.Time) error
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error)
	ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error)
}
