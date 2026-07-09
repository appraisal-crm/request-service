---
description: Adds a Kafka producer or consumer to a service. Use when wiring a service into the event bus.
disable-model-invocation: true
argument-hint: <event-name> out|in <service-name> e.g. inspect.completed out inspect-service
---

# Kafka integration: $ARGUMENTS

## Event map

| Event                    | Producer        | Consumers                              |
|--------------------------|-----------------|----------------------------------------|
| `request.created`        | request-service | notification-service                   |
| `request.status_changed` | request-service | notification-service, inspect-service  |
| `inspect.completed`      | inspect-service | request-service, notification-service  |
| `report.ready`           | review-service  | notification-service                   |

---

## If PRODUCER (out)

**1. `internal/domain/events.go`** — define the event type:
```go
const TopicInspectCompleted = "inspect.completed"

type InspectCompletedEvent struct {
    RequestID   string    `json:"request_id"`
    InspectorID string    `json:"inspector_id"`
    CompletedAt time.Time `json:"completed_at"`
    PhotoCount  int       `json:"photo_count"`
}
```

**2. `internal/kafka/producer.go`** — thin wrapper over `segmentio/kafka-go`:
```go
type Producer struct{ writer *kafka.Writer }

func (p *Producer) PublishInspectCompleted(ctx context.Context, e domain.InspectCompletedEvent) error
```

**3. Publish AFTER transaction commit** — never inside it:
```go
if err := tx.Commit(ctx); err != nil { return err }
_ = s.producer.PublishInspectCompleted(ctx, event) // at-least-once; consumers must be idempotent
```

---

## If CONSUMER (in)

**1. `internal/kafka/consumer.go`** — consumer group:
```go
type Consumer struct {
    reader  *kafka.Reader
    service *Service
}
func (c *Consumer) Run(ctx context.Context) error // blocking loop, exits on ctx cancel
```

**2. Idempotency** — track processed events:
```sql
CREATE TABLE processed_events (
    event_id    TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Check before processing; skip if already seen.

**3. Wire into main.go** — run in a goroutine with graceful shutdown via context cancellation.

---

## ENV variables to add in config.go

```go
KafkaBrokers: os.Getenv("KAFKA_BROKERS"), // e.g. "kafka:9092"
KafkaGroupID: os.Getenv("KAFKA_GROUP_ID"), // e.g. "inspect-service-group"
```

## Tests

Define `EventPublisher` as an interface; mock it in service unit tests.
