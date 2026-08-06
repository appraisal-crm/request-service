package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/appraisal-crm/request-service/internal/service"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDedup struct {
	mock.Mock
}

func (m *mockDedup) Seen(ctx context.Context, eventID string) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}

func (m *mockDedup) Forget(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

type mockChanger struct {
	mock.Mock
}

func (m *mockChanger) ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error) {
	args := m.Called(ctx, id, newStatus)
	if req, ok := args.Get(0).(*domain.Request); ok {
		return req, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestProcess_InspectCompleted_Success(t *testing.T) {
	dedup := new(mockDedup)
	changer := new(mockChanger)
	consumer := &Consumer{dedup: dedup, svc: changer}

	eventID := uuid.New()
	reqID := uuid.New()

	env := inboundEnvelope{
		EventID:   eventID,
		EventType: EventTypeInspectCompleted,
		RequestID: reqID,
		Data:      json.RawMessage(`{}`),
	}
	bytes, _ := json.Marshal(env)

	dedup.On("Seen", mock.Anything, eventID.String()).Return(false, nil)
	changer.On("ChangeStatus", mock.Anything, reqID, domain.StatusInspectionCompleted).Return(&domain.Request{ID: reqID, Status: domain.StatusInspectionCompleted}, nil)

	err := consumer.process(context.Background(), kafka.Message{Value: bytes})
	assert.NoError(t, err)
	dedup.AssertExpectations(t)
	changer.AssertExpectations(t)
}

func TestProcess_ReportReady_Success(t *testing.T) {
	dedup := new(mockDedup)
	changer := new(mockChanger)
	consumer := &Consumer{dedup: dedup, svc: changer}

	eventID := uuid.New()
	reqID := uuid.New()

	env := inboundEnvelope{
		EventID:   eventID,
		EventType: EventTypeReportReady,
		RequestID: reqID,
		Data:      json.RawMessage(`{}`),
	}
	bytes, _ := json.Marshal(env)

	dedup.On("Seen", mock.Anything, eventID.String()).Return(false, nil)
	changer.On("ChangeStatus", mock.Anything, reqID, domain.StatusReportSent).Return(&domain.Request{ID: reqID, Status: domain.StatusReportSent}, nil)

	err := consumer.process(context.Background(), kafka.Message{Value: bytes})
	assert.NoError(t, err)
	dedup.AssertExpectations(t)
	changer.AssertExpectations(t)
}

func TestProcess_Duplicate_Skipped(t *testing.T) {
	dedup := new(mockDedup)
	changer := new(mockChanger)
	consumer := &Consumer{dedup: dedup, svc: changer}

	eventID := uuid.New()
	reqID := uuid.New()

	env := inboundEnvelope{
		EventID:   eventID,
		EventType: EventTypeInspectCompleted,
		RequestID: reqID,
	}
	bytes, _ := json.Marshal(env)

	dedup.On("Seen", mock.Anything, eventID.String()).Return(true, nil)

	err := consumer.process(context.Background(), kafka.Message{Value: bytes})
	assert.NoError(t, err)
	dedup.AssertExpectations(t)
	changer.AssertNotCalled(t, "ChangeStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcess_InvalidTransition_LoggedAndNotFailed(t *testing.T) {
	dedup := new(mockDedup)
	changer := new(mockChanger)
	consumer := &Consumer{dedup: dedup, svc: changer}

	eventID := uuid.New()
	reqID := uuid.New()

	env := inboundEnvelope{
		EventID:   eventID,
		EventType: EventTypeInspectCompleted,
		RequestID: reqID,
	}
	bytes, _ := json.Marshal(env)

	dedup.On("Seen", mock.Anything, eventID.String()).Return(false, nil)
	changer.On("ChangeStatus", mock.Anything, reqID, domain.StatusInspectionCompleted).Return(nil, service.ErrInvalidStatusTransition)

	err := consumer.process(context.Background(), kafka.Message{Value: bytes})
	assert.NoError(t, err)
	dedup.AssertExpectations(t)
	changer.AssertExpectations(t)
}
