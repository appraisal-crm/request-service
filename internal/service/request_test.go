package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/appraisal-crm/request-service/internal/repository"
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

func (m *mockRepository) Update(ctx context.Context, req *domain.Request, prevUpdatedAt time.Time) error {
	return m.Called(ctx, req, prevUpdatedAt).Error(0)
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

func TestUpdate_MergesOnlyProvidedFields(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	oldAddress := "Old street 1"
	oldType := domain.ObjectTypeApartment
	prevUpdatedAt := time.Now().Add(-time.Hour)
	existing := &domain.Request{
		ID:         id,
		Status:     domain.StatusInProgress,
		ObjectType: &oldType,
		Address:    &oldAddress,
		UpdatedAt:  prevUpdatedAt,
	}

	inspectorID := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(r *domain.Request) bool {
		return r.InspectorID != nil && *r.InspectorID == inspectorID &&
			r.ObjectType == &oldType && r.Address == &oldAddress &&
			r.Status == domain.StatusInProgress
	}), prevUpdatedAt).Return(nil)

	req, err := svc.Update(context.Background(), id, UpdateInput{InspectorID: &inspectorID})

	assert.NoError(t, err)
	assert.Equal(t, &inspectorID, req.InspectorID)
	assert.Equal(t, &oldAddress, req.Address)
	assert.Equal(t, domain.StatusInProgress, req.Status)
	assert.True(t, req.UpdatedAt.After(prevUpdatedAt))
	repo.AssertExpectations(t)
}

func TestUpdate_ConflictOnConcurrentModification(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(&domain.Request{ID: id, Status: domain.StatusNew}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(repository.ErrConflict)

	_, err := svc.Update(context.Background(), id, UpdateInput{})

	assert.ErrorIs(t, err, ErrConflict)
	repo.AssertExpectations(t)
}

func TestUpdate_NotFound(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, repository.ErrNotFound)

	_, err := svc.Update(context.Background(), id, UpdateInput{})

	assert.ErrorIs(t, err, ErrNotFound)
	repo.AssertExpectations(t)
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

// Empty UpdateInput changes no fields but still bumps updated_at and persists.
// This documents current behavior (the handler maps it to 200; see ACRM-77 TC-VAL-16).
func TestUpdate_EmptyInputBumpsUpdatedAtWithoutChangingFields(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	oldAddress := "Old street 1"
	oldType := domain.ObjectTypeApartment
	inspectorID := uuid.New()
	prevUpdatedAt := time.Now().Add(-time.Hour)
	existing := &domain.Request{
		ID:          id,
		Status:      domain.StatusInProgress,
		InspectorID: &inspectorID,
		ObjectType:  &oldType,
		Address:     &oldAddress,
		UpdatedAt:   prevUpdatedAt,
	}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(r *domain.Request) bool {
		// fields untouched, updated_at moved forward, and the compare-and-set
		// still uses the previous updated_at as the guard.
		return r.InspectorID == &inspectorID && r.ObjectType == &oldType &&
			r.Address == &oldAddress && r.Status == domain.StatusInProgress &&
			r.UpdatedAt.After(prevUpdatedAt)
	}), prevUpdatedAt).Return(nil)

	req, err := svc.Update(context.Background(), id, UpdateInput{})

	assert.NoError(t, err)
	assert.Equal(t, &inspectorID, req.InspectorID)
	assert.Equal(t, &oldType, req.ObjectType)
	assert.Equal(t, &oldAddress, req.Address)
	assert.Equal(t, domain.StatusInProgress, req.Status)
	assert.True(t, req.UpdatedAt.After(prevUpdatedAt))
	repo.AssertExpectations(t)
}

// An unexpected repository error from Update is propagated unchanged (not masked
// as ErrNotFound or ErrConflict).
func TestUpdate_PropagatesUnknownRepoError(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	repoErr := errors.New("db exploded")
	repo.On("GetByID", mock.Anything, id).Return(&domain.Request{ID: id, Status: domain.StatusNew}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(repoErr)

	_, err := svc.Update(context.Background(), id, UpdateInput{})

	assert.ErrorIs(t, err, repoErr)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NotErrorIs(t, err, ErrConflict)
	repo.AssertExpectations(t)
}

// An unexpected repository error from the initial GetByID is propagated unchanged.
func TestUpdate_PropagatesUnknownRepoErrorFromGetByID(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	id := uuid.New()
	repoErr := errors.New("db exploded")
	repo.On("GetByID", mock.Anything, id).Return(nil, repoErr)

	_, err := svc.Update(context.Background(), id, UpdateInput{})

	assert.ErrorIs(t, err, repoErr)
	assert.NotErrorIs(t, err, ErrNotFound)
	repo.AssertExpectations(t)
}

func TestListByClientID_ReturnsRequests(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	clientID := uuid.New()
	expected := []*domain.Request{
		{ID: uuid.New(), ClientID: clientID, Status: domain.StatusNew},
		{ID: uuid.New(), ClientID: clientID, Status: domain.StatusInProgress},
	}
	repo.On("ListByClientID", mock.Anything, clientID).Return(expected, nil)

	got, err := svc.ListByClientID(context.Background(), clientID)

	assert.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}

// A client with no requests yields an empty (non-nil) slice, not null.
func TestListByClientID_EmptyReturnsEmptySlice(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	clientID := uuid.New()
	repo.On("ListByClientID", mock.Anything, clientID).Return([]*domain.Request{}, nil)

	got, err := svc.ListByClientID(context.Background(), clientID)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
	repo.AssertExpectations(t)
}

func TestListByClientID_PropagatesRepoError(t *testing.T) {
	repo := &mockRepository{}
	svc := NewRequestService(repo)

	clientID := uuid.New()
	repoErr := errors.New("db exploded")
	repo.On("ListByClientID", mock.Anything, clientID).Return(nil, repoErr)

	got, err := svc.ListByClientID(context.Background(), clientID)

	assert.ErrorIs(t, err, repoErr)
	assert.Nil(t, got)
	repo.AssertExpectations(t)
}
