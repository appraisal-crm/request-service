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

**Done (merged to dev):**
- `infra/docker-compose.yml` — PostgreSQL 17, Redis 7, Keycloak 26 (Kafka not yet in compose)
- `services/request-service` — mvp working: CRUD, state machine, JWT auth, RBAC, Swagger, unit tests, optimistic locking on both PATCH endpoints (CAS, no version column), graceful shutdown

**Keycloak note:** the compose starts Keycloak with an empty database — the `appraisal` realm, roles, client, and test users must be bootstrapped manually (see `docs/onboarding.md` § Keycloak setup for copy-paste commands).

**Not yet implemented:**
- API Gateway
- Inspect Service
- Review Service (blocked — appraisal formulas not yet formalized by client)
- Notification Service
- Frontend (all 4 SPAs)
- Kafka integration in services

## Go module paths

Each service is an independent module:
```
github.com/Meidorislav/appraisal-crm/services/request-service
github.com/Meidorislav/appraisal-crm/services/<name>-service   # pattern for new services
```

## Go service structure (follow request-service as the reference)

```
services/<name>-service/
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
  docs/                # Swagger (swaggo/swag)
```

## Code rules

- No magic frameworks — chi, pgx, playground/validator only
- Config via `os.Getenv` only — no viper, no cobra
- Migration files: `000001_<description>.up.sql` / `000001_<description>.down.sql` — sequential numbering, always provide both up and down
- JWT validation via Keycloak JWKS: use `MicahParks/keyfunc` + `JWKS_URL` env var (see request-service config/middleware as reference)
- Domain errors in `domain/errors.go` for new services; in request-service they live in `service/request.go` (legacy — follow the new pattern going forward)
- HTTP status codes mapped in handler layer only
- Each Kafka event is a distinct type in `domain/events.go`
- Optimistic locking for concurrent ChangeStatus (see request-service pattern)
- Swagger annotations required for all public endpoints
- Unit tests for business logic in `service/` (not integration tests)

## Kafka events

| Event                    | Producer        | Consumers                              |
|--------------------------|-----------------|----------------------------------------|
| `request.created`        | request-service | notification-service                   |
| `request.status_changed` | request-service | notification-service, inspect-service  |
| `inspect.completed`      | inspect-service | request-service, notification-service  |
| `report.ready`           | review-service  | notification-service                   |

## Commands

```bash
# Start infrastructure
docker compose -f infra/docker-compose.yml up -d

# Run a service
cd services/request-service && go run cmd/server/main.go

# Tests
cd services/request-service && go test ./...

# Swagger — NOTE: docs/ is gitignored, so `go build ./...` FAILS on a fresh
# checkout until docs are generated (`make generate` or `make build`)
swag init -g cmd/server/main.go -o docs

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
- Conventional commits with the Jira key: `fix(requests): ... (ACRM-84)`

## Documentation

- Developer onboarding: `docs/onboarding.md`
- BRD: `docs/brd/`
- Architecture (C4 / Structurizr): `docs/architecture/`
- ADRs: `docs/adr/`
- API contracts: `services/*/docs/swagger.json` (generated, gitignored — run `make generate`)
