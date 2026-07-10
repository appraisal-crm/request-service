# ADR-007: Reliable Event Publishing via Transactional Outbox

> English version · [Русская версия](i18n/ru/ADR-007-event-delivery-outbox.md)

**Status:** Proposed  
**Date:** 2026-07-10

Refines [ADR-001](ADR-001-kafka.md) — that ADR chose Kafka as the broker; this one
decides *how* events are published reliably, how topics are laid out, and what
guarantees consumers can rely on.

## Context

A service must do two things when domain state changes (e.g. a request status
transition): persist the change in PostgreSQL **and** publish an event to Kafka.
These are two independent systems with no shared transaction, so the process can
crash between them — a **dual-write problem**:

- Publish first, then commit → crash loses the DB write → an event announces a
  state that never persisted (phantom event).
- Commit first, then publish → crash loses the event → state changed but no one
  was notified and downstream services never react (lost event).

Neither order is safe, and no amount of retry logic in a handler fixes it,
because two separate systems cannot be written atomically. We also need to fix
the topic layout, wire format, delivery semantics, and broker topology before
building producers and consumers, since these are hard to change later.

## Decision

**1. Transactional Outbox for publishing.** Handlers never publish to Kafka
directly. The event is written to an `outbox` table in the **same DB transaction**
as the state change, so both commit atomically or not at all. A background relay
worker (polling publisher) reads unpublished rows, publishes them to Kafka, and
marks them sent. Kafka being down only delays delivery; nothing is lost. We use
polling rather than CDC (Debezium) to avoid extra infrastructure.

**2. One topic per producing service.** Events are differentiated by an
`event_type` field, not by a topic-per-event:

| Topic            | Producer        | Event types                                  |
|------------------|-----------------|----------------------------------------------|
| `request.events` | request-service | `request.created`, `request.status_changed`  |
| `inspect.events` | inspect-service | `inspect.completed`                          |
| `review.events`  | review-service  | `report.ready`                               |

The **message key** is the aggregate id (e.g. `request_id`), so all events of one
aggregate land in the same partition and stay ordered relative to each other.

**3. JSON envelope.** No Schema Registry on MVP:

```json
{
  "event_id":    "uuid",
  "event_type":  "request.status_changed",
  "version":     1,
  "occurred_at": "2026-07-10T12:34:56Z",
  "request_id":  "uuid",
  "data":        { "old_status": "Appraisal", "new_status": "Report Sent", "...": "..." }
}
```

Schema evolution is additive only; breaking changes bump `version`.

**4. At-least-once delivery.** Consumers process the event, then commit the Kafka
offset. Combined with outbox retries, this means duplicates are possible, so every
consumer MUST be idempotent — deduplicating by `event_id` (e.g. Redis `SET NX EX`,
or an `ON CONFLICT DO NOTHING` table).

**5. KRaft mode, no Zookeeper.** The broker runs in KRaft mode — one fewer
component to deploy and operate, and the supported topology going forward.

## Consequences

**Positive:**
- No lost events and no phantom events — DB state and the outbox row are atomic.
- New consumers (analytics, audit) subscribe without touching producers.
- Per-aggregate ordering is guaranteed by keying on the aggregate id.
- Minimal infrastructure — no Zookeeper, no Schema Registry, no CDC pipeline.

**Negative:**
- An `outbox` table and a relay worker must be built, run, and monitored in every
  producing service.
- At-least-once means duplicates are normal, not exceptional — idempotency is
  mandatory in every consumer, not optional.
- The polling relay adds latency bounded by its poll interval (mitigated by a
  short interval; acceptable at current volume).
- JSON is larger and less strictly enforced than Avro/Protobuf; migrating to a
  Schema Registry is deferred until volume or contract strictness demands it.
