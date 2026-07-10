# Appraisal CRM

Commercial project built for a real client. Code goes to production — treat it accordingly.

## What it does

CRM for a property appraisal company (apartments, houses, land, vehicles, commercial real estate).
Digitizes the full cycle: client submits request → inspector visits the property → appraiser evaluates → client receives report.

## Roles

| Role          | What they do in the system                                                         |
|---------------|------------------------------------------------------------------------------------|
| Client        | Submits a request, tracks status, downloads the final report                       |
| Appraiser     | Accepts requests, assigns an inspector, conducts appraisal, sends the report       |
| Inspector     | Receives field assignments, uploads photos and property data                       |
| Administrator | Manages users, monitors the system                                                 |

## Request lifecycle (strictly linear — no going back)

```
New → In Progress → Inspection Scheduled → Inspection Completed → Appraisal → Report Sent → Closed
```

Transitions are validated in the service layer. Skipping a step is not allowed.

## Stack

| Layer              | Technology                               |
|--------------------|------------------------------------------|
| Backend services   | Go (chi, pgx, golang-migrate)            |
| Database per svc   | PostgreSQL (Database-per-Service pattern)|
| API Gateway        | Go (not yet implemented)                 |
| Auth               | Keycloak 26 (OAuth2/OIDC)               |
| Events             | Apache Kafka                             |
| Cache / Sessions   | Redis                                    |
| Object storage     | S3 Yandex Cloud                          |
| Frontend (4 SPAs)  | React + TypeScript                       |
| Architecture docs  | Structurizr DSL (C4)                     |

## Current state

**Done (in `dev`, released to `main`):**
- `docker-compose.yml` (repo root) — PostgreSQL 17, Redis 7, Keycloak 26, Kafka 4 (KRaft) + Kafka UI; the service container + migrations run under the `app` profile (`docker compose --profile app up -d --build`), plain `docker compose up -d` starts infra only
- `request-service` (this repo) — mvp working: CRUD, state machine, JWT auth, RBAC, Swagger, unit tests, optimistic locking on both PATCH endpoints (CAS, no version column), graceful shutdown

**Keycloak note:** the compose starts Keycloak with an empty database — the `appraisal` realm, roles, client, and test users must be bootstrapped manually (see `docs/onboarding.md` § Keycloak setup for copy-paste commands).

**Not yet implemented:**
- API Gateway
- Inspect Service
- Review Service (blocked — appraisal formulas not yet formalized by client)
- Notification Service
- Frontend (all 4 SPAs)
- Kafka integration in services

## Go module path

Not a monorepo. Every service — and each of the 4 frontend SPAs — is a separate
repository under the `appraisal-crm` GitHub organization. Each Go service is an
independent module:
```
github.com/appraisal-crm/request-service
github.com/appraisal-crm/<name>-service   # pattern for new services
```

## Go service structure (this repo is the reference layout)

```
<name>-service/
  cmd/server/          # entry point, wire DI
  internal/
    domain/            # entities, domain errors
    repository/        # interface + PostgreSQL implementation
    service/           # business logic, state machine
    handler/           # HTTP (chi router), request/response DTOs
    middleware/        # JWT auth, role-based access
    httputil/          # shared response helpers
  config/              # ENV config (os.Getenv only)
  migrations/          # SQL files (golang-migrate up/down)
  api/                 # Swagger (swaggo/swag, generated)
```

## Code rules

- No magic frameworks — chi, pgx, playground/validator only
- Config via `os.Getenv` only — no viper, no cobra
- Migration files: `000001_<description>.up.sql` / `000001_<description>.down.sql` — sequential numbering, always provide both up and down
- JWT validation via Keycloak JWKS: use `MicahParks/keyfunc` + `JWKS_URL` env var (see request-service config/middleware as reference)
- Domain errors in `domain/errors.go` for new services; in request-service they live in `service/request.go` (legacy — follow the new pattern going forward)
- HTTP status codes mapped in handler layer only
- Each Kafka event is a distinct type in `domain/events.go`
- Publish events via transactional outbox — write to the `outbox` table in the same tx as the state change; never publish from a handler
- Optimistic locking for concurrent ChangeStatus (see request-service pattern)
- Swagger annotations required for all public endpoints
- Unit tests for business logic in `service/` (not integration tests)

## Kafka events

**One topic per producing service** — events are told apart by `event_type`, NOT by a
topic-per-event. All events of one aggregate share a topic so they stay ordered per key.

| event_type               | Topic            | Producer        | Consumers                              |
|--------------------------|------------------|-----------------|----------------------------------------|
| `request.created`        | `request.events` | request-service | notification-service                   |
| `request.status_changed` | `request.events` | request-service | notification-service, inspect-service  |
| `inspect.completed`      | `inspect.events` | inspect-service | request-service, notification-service  |
| `report.ready`           | `review.events`  | review-service  | notification-service                   |

**Event conventions:**
- **Message key** = aggregate id (e.g. `request_id`) → per-aggregate ordering within a partition
- **Format** = JSON envelope: `event_id`, `event_type`, `version`, `occurred_at`, `request_id`, `data{}` (no Schema Registry on MVP)
- **Delivery** = at-least-once → consumers MUST be idempotent (dedup by `event_id`, e.g. Redis `SET NX EX`)
- **Publishing** = transactional outbox (see Code rules) — never publish straight from a handler
- **Schema evolution** = additive only; bump `version` for breaking changes
- **Broker** = KRaft mode (no Zookeeper)

## Commands

```bash
# Start infrastructure (compose lives at the repo root)
docker compose up -d

# Run the service (from the repo root)
go run cmd/server/main.go

# Tests
go test ./...

# Swagger — NOTE: api/ is gitignored, so `go build ./...` FAILS on a fresh
# checkout until docs are generated (`make generate` or `make build`)
swag init -g cmd/server/main.go -o api

# Migrations
migrate -path migrations/ -database "postgres://..." up
```

## Hard rules — do not break

- No synchronous cross-service calls between business services — events via Kafka only
- No cross-database JOINs between services
- Do not implement Review Service until appraisal formulas are formalized
- Do not switch config from `os.Getenv` to viper without explicit agreement
- Never modify already-applied migrations — new additive migrations only

## Workflow

- Tasks tracked in Jira, project **ACRM** (mdrslv.atlassian.net)
- Branch from `dev`: `feature/<scope>` / `fix/<scope>`; PR into `dev`
- `main` is the release branch — updated by merging `dev` → `main` on release; never PR features directly into `main`
- Conventional commits with the Jira key: `fix(requests): ... (ACRM-84)`

## Documentation

- Developer onboarding: `docs/onboarding.md`
- BRD: `docs/brd/`
- Architecture (C4 / Structurizr): `docs/architecture/`
- ADRs: `docs/adr/`
- API contracts: `api/swagger.json` (generated, gitignored — run `make generate`)
