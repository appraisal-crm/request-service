package domain

import (
	"time"

	"github.com/google/uuid"
)

// TopicRequestEvents carries every domain event produced by request-service.
// Events are told apart by EventType, not by topic (ADR-007).
const TopicRequestEvents = "request.events"

// Event types on TopicRequestEvents.
const (
	EventTypeRequestCreated       = "request.created"
	EventTypeRequestStatusChanged = "request.status_changed"
)

// EventVersion is the current envelope schema version. Additive changes keep it;
// breaking changes bump it (ADR-007).
const EventVersion = 1

// EventEnvelope is the JSON wire format for every event on TopicRequestEvents.
// Data holds the event-type-specific payload.
type EventEnvelope struct {
	EventID    uuid.UUID `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	RequestID  uuid.UUID `json:"request_id"`
	Data       any       `json:"data"`
}

// RequestCreatedData is the payload for EventTypeRequestCreated.
type RequestCreatedData struct {
	ClientID    uuid.UUID   `json:"client_id"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phone_number"`
	ObjectType  *ObjectType `json:"object_type,omitempty"`
	Address     *string     `json:"address,omitempty"`
	Status      Status      `json:"status"`
}

// RequestStatusChangedData is the payload for EventTypeRequestStatusChanged.
type RequestStatusChangedData struct {
	OldStatus Status `json:"old_status"`
	NewStatus Status `json:"new_status"`
}

// NewRequestCreatedEvent builds the envelope for a newly created request.
func NewRequestCreatedEvent(r Request) EventEnvelope {
	return EventEnvelope{
		EventID:    uuid.New(),
		EventType:  EventTypeRequestCreated,
		Version:    EventVersion,
		OccurredAt: time.Now().UTC(),
		RequestID:  r.ID,
		Data: RequestCreatedData{
			ClientID:    r.ClientID,
			Email:       r.Email,
			PhoneNumber: r.PhoneNumber,
			ObjectType:  r.ObjectType,
			Address:     r.Address,
			Status:      r.Status,
		},
	}
}

// NewRequestStatusChangedEvent builds the envelope for a status transition.
func NewRequestStatusChangedEvent(requestID uuid.UUID, oldStatus, newStatus Status) EventEnvelope {
	return EventEnvelope{
		EventID:    uuid.New(),
		EventType:  EventTypeRequestStatusChanged,
		Version:    EventVersion,
		OccurredAt: time.Now().UTC(),
		RequestID:  requestID,
		Data: RequestStatusChangedData{
			OldStatus: oldStatus,
			NewStatus: newStatus,
		},
	}
}
