package service

import (
	"context"
	"testing"
	"time"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) Create(ctx context.Context, req *domain.Request) error {
	return m.Called(ctx, req).Error(0)
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Request), args.Error(1)
}

func (m *mockRepository) Update(ctx context.Context, req *domain.Request) error {
	return m.Called(ctx, req).Error(0)
}

func (m *mockRepository) ChangeStatus(ctx context.Context, id uuid.UUID, oldStatus, newStatus domain.Status, updatedAt time.Time) error {
	return m.Called(ctx, id, oldStatus, newStatus, updatedAt).Error(0)
}

func (m *mockRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Request), args.Error(1)
}

func (m *mockRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Request), args.Error(1)
}

func TestCreate_SetsStatusNewAndGeneratesID(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	clientID := uuid.New()
	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Request) bool {
		return r.ClientID == clientID && r.Status == domain.StatusNew && r.ID != uuid.Nil &&
			r.Email == "client@example.com" && r.PhoneNumber == "+71234567890"
	})).Return(nil)

	req, err := svc.Create(context.Background(), CreateInput{
		ClientID:    clientID,
		Email:       "client@example.com",
		PhoneNumber: "+71234567890",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusNew, req.Status)
	assert.Equal(t, clientID, req.ClientID)
	assert.Equal(t, "client@example.com", req.Email)
	assert.Equal(t, "+71234567890", req.PhoneNumber)
	assert.NotEqual(t, uuid.Nil, req.ID)
	repo.AssertExpectations(t)
}

func TestChangeStatus_ValidTransition(t *testing.T) {
	transitions := []struct {
		from domain.Status
		to   domain.Status
	}{
		{domain.StatusNew, domain.StatusInProgress},
		{domain.StatusInProgress, domain.StatusInspectionScheduled},
		{domain.StatusInspectionScheduled, domain.StatusInspectionCompleted},
		{domain.StatusInspectionCompleted, domain.StatusAppraisal},
		{domain.StatusAppraisal, domain.StatusReportSent},
		{domain.StatusReportSent, domain.StatusClosed},
	}

	for _, tt := range transitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			repo := &mockRepository{}
			svc := NewRequestService(repo)

			id := uuid.New()
			existing := &domain.Request{ID: id, Status: tt.from}

			repo.On("GetByID", mock.Anything, id).Return(existing, nil)
			repo.On("ChangeStatus", mock.Anything, id, tt.from, tt.to, mock.AnythingOfType("time.Time")).Return(nil)

			req, err := svc.ChangeStatus(context.Background(), id, tt.to)

			assert.NoError(t, err)
			assert.Equal(t, tt.to, req.Status)
			repo.AssertExpectations(t)
		})
	}
}

func TestChangeStatus_InvalidTransition(t *testing.T) {
	cases := []struct {
		from domain.Status
		to   domain.Status
	}{
		{domain.StatusNew, domain.StatusClosed},
		{domain.StatusNew, domain.StatusAppraisal},
		{domain.StatusInProgress, domain.StatusNew},
		{domain.StatusClosed, domain.StatusNew},
	}

	for _, tt := range cases {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			repo := &mockRepository{}
			svc := NewRequestService(repo)

			id := uuid.New()
			repo.On("GetByID", mock.Anything, id).Return(&domain.Request{ID: id, Status: tt.from}, nil)

			_, err := svc.ChangeStatus(context.Background(), id, tt.to)

			assert.ErrorIs(t, err, ErrInvalidStatusTransition)
			repo.AssertExpectations(t)
		})
	}
}

func TestCreate_OptionalFieldsCanBeNil(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Request) bool {
		return r.ObjectType == nil && r.Address == nil
	})).Return(nil)

	req, err := svc.Create(context.Background(), CreateInput{
		ClientID:    uuid.New(),
		Email:       "client@example.com",
		PhoneNumber: "+71234567890",
	})

	assert.NoError(t, err)
	assert.Nil(t, req.ObjectType)
	assert.Nil(t, req.Address)
	repo.AssertExpectations(t)
}
