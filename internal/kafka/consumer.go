package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/appraisal-crm/request-service/internal/service"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const (
	EventTypeInspectCompleted = "inspect.completed"
	EventTypeReportReady      = "report.ready"
)

// Deduplicator marks event ids so a redelivered message is processed once.
type Deduplicator interface {
	Seen(ctx context.Context, eventID string) (bool, error)
	Forget(ctx context.Context, eventID string) error
}

// RequestStatusChanger changes status of requests on domain events.
type RequestStatusChanger interface {
	ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error)
}

// inboundEnvelope is the standard event envelope.
type inboundEnvelope struct {
	EventID   uuid.UUID       `json:"event_id"`
	EventType string          `json:"event_type"`
	RequestID uuid.UUID       `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

// Consumer reads Kafka events and advances request state machine.
type Consumer struct {
	reader *kafka.Reader
	dedup  Deduplicator
	svc    RequestStatusChanger
}

func NewConsumer(brokers []string, groupID, topic string, dedup Deduplicator, svc RequestStatusChanger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			GroupID: groupID,
			Topic:   topic,
		}),
		dedup: dedup,
		svc:   svc,
	}
}

// Run consumes messages until context is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // shutting down
			}
			return err
		}
		if err := c.process(ctx, m); err != nil {
			slog.ErrorContext(ctx, "consumer stopping after processing error", "error", err, "topic", m.Topic)
			return err
		}
		if err := c.reader.CommitMessages(ctx, m); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

func (c *Consumer) process(ctx context.Context, m kafka.Message) error {
	var env inboundEnvelope
	if err := json.Unmarshal(m.Value, &env); err != nil {
		slog.WarnContext(ctx, "skipping malformed event", "error", err, "topic", m.Topic, "offset", m.Offset)
		return nil
	}
	if env.EventID == uuid.Nil {
		slog.WarnContext(ctx, "skipping event without event_id", "topic", m.Topic, "offset", m.Offset)
		return nil
	}
	if env.RequestID == uuid.Nil {
		slog.WarnContext(ctx, "skipping event without request_id", "topic", m.Topic, "offset", m.Offset)
		return nil
	}

	var targetStatus domain.Status
	switch env.EventType {
	case EventTypeInspectCompleted:
		targetStatus = domain.StatusInspectionCompleted
	case EventTypeReportReady:
		targetStatus = domain.StatusReportSent
	default:
		// Ignore irrelevant event types on this topic
		return nil
	}

	dup, err := c.dedup.Seen(ctx, env.EventID.String())
	if err != nil {
		return fmt.Errorf("dedup check: %w", err)
	}
	if dup {
		slog.InfoContext(ctx, "duplicate event, skipping", "event_id", env.EventID, "event_type", env.EventType)
		return nil
	}

	_, err = c.svc.ChangeStatus(ctx, env.RequestID, targetStatus)
	if err != nil {
		// If status transition is invalid (e.g. already advanced or irrelevant), or request not found, log and do not retry
		if errors.Is(err, service.ErrInvalidStatusTransition) || errors.Is(err, service.ErrNotFound) {
			slog.WarnContext(ctx, "cannot apply event status transition", "error", err, "event_id", env.EventID, "request_id", env.RequestID, "target_status", targetStatus)
			return nil
		}
		// Undo dedup mark on transient failure so redelivery can retry
		if ferr := c.dedup.Forget(ctx, env.EventID.String()); ferr != nil {
			slog.ErrorContext(ctx, "failed to forget dedup key", "error", ferr, "event_id", env.EventID)
		}
		return fmt.Errorf("change status: %w", err)
	}

	slog.InfoContext(ctx, "processed Kafka event and updated request status", "event_type", env.EventType, "request_id", env.RequestID, "new_status", targetStatus)
	return nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
