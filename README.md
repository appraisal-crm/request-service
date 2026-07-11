# Request Service

> English version · [Русская версия](i18n/ru/README.md)

First business service of the **Appraisal CRM** platform (Go): appraisal requests CRUD + lifecycle
state machine. The reference implementation for all other services in the platform.

CRM for a property appraisal company (apartments, houses, land, vehicles, commercial real estate).
Digitizes the full cycle: client submits a request → inspector visits the property → appraiser evaluates → client receives the report.

Commercial project built for a real client. Code goes to production.

> **New to the project?** Start with the [Developer Onboarding Guide](docs/onboarding.md).

## Repository map

| Path | What lives there |
|------|------------------|
| `cmd/server/` | Entry point, dependency wiring |
| `internal/` | Domain, repository, service (state machine), handlers, middleware |
| `config/` | ENV configuration (`os.Getenv` only) |
| `migrations/` | SQL migrations (golang-migrate up/down) |
| `api/` | Generated Swagger docs (swaggo/swag — gitignored, run `make generate`) |
| `docker-compose.yml` | Local infrastructure: PostgreSQL 17, Redis 7, Keycloak 26, Kafka 4 (KRaft), Kafka UI; the service itself behind the `app` profile |
| `Dockerfile` | Service image (multi-stage; generates Swagger docs during build) |
| `docs/brd/` | Business Requirements Document (en/ru) |
| `docs/architecture/` | C4 diagrams in Structurizr DSL (en/ru) |
| `docs/adr/` | Architecture Decision Records (en/ru) |
| `docs/onboarding.md` | Developer onboarding guide — start here |
| `CLAUDE.md` | Project conventions and hard rules (read it even if you don't use Claude) |

## Architecture at a glance

Microservices in Go, one PostgreSQL database per service, async communication via Kafka events only (no direct service-to-service HTTP calls), Keycloak for OAuth2/OIDC, four React SPAs per role behind an API Gateway. Each service lives in its own repository; this one is `request-service`.

**Implemented today:** infrastructure compose + `request-service` (CRUD, state machine, JWT/RBAC, Swagger, unit tests).
**Not yet:** API Gateway, inspect-service, review-service (blocked on client formulas), notification-service, frontends, Kafka wiring.

See [C4 diagrams](docs/architecture/README.md) for the target picture and [ADRs](docs/adr/README.md) for the reasoning behind the stack.

## Requirements

- Go 1.22+
- Docker & Docker Compose
- [migrate CLI](https://github.com/golang-migrate/migrate)
- [swag CLI](https://github.com/swaggo/swag)

## Quickstart

```bash
# 1a. Shared infra — Keycloak :8180, Kafka :9094, Kafka UI :8090 (repo-root ../infra)
docker compose -f ../infra/docker-compose.yml up -d
# 1b. This service's data infra — PostgreSQL :5433, Redis :6380
docker compose up -d

# 2. Keycloak bootstrap — the shared infra starts Keycloak EMPTY, one-time setup required:
#    realm `appraisal`, roles, a public client, test users.
#    Follow docs/onboarding.md § "Keycloak setup" (5 minutes, copy-paste commands).

# 3. Migrations + run the service (from the repo root)
make migrate-up
make run          # generates Swagger docs, starts on :8080
```

Alternatively, run the whole stack in containers — the service is built from the
`Dockerfile` and migrations apply automatically (shared `../infra` must be up first):

```bash
docker compose -f ../infra/docker-compose.yml up -d
docker compose --profile app up -d --build
```

Then open Swagger UI at http://localhost:8080/swagger/index.html.

## Environment variables

| Variable          | Default                                                                    | Description               |
|-------------------|----------------------------------------------------------------------------|---------------------------|
| `SERVER_PORT`     | `8080`                                                                      | HTTP server port          |
| `DATABASE_URL`    | — **(required)**, e.g. `postgres://appraisal:appraisal@localhost:5433/request_db?sslmode=disable` | PostgreSQL connection URL |
| `JWKS_URL`        | `http://localhost:8180/realms/appraisal/protocol/openid-connect/certs`     | Keycloak JWKS endpoint    |
| `ALLOWED_ORIGINS` | `*`                                                                        | Comma-separated CORS origins (`*` for local dev only) |

## Make targets

| Command             | Description                        |
|---------------------|------------------------------------|
| `make run`          | Generate docs, run the service     |
| `make build`        | Generate docs, build the binary    |
| `make generate`     | Regenerate Swagger docs            |
| `make test`         | Run unit tests                     |
| `make migrate-up`   | Apply all pending migrations       |
| `make migrate-down` | Roll back the last migration       |
| `make up`           | Start infrastructure containers    |
| `make up-all`       | Start infra + the service in containers |
| `make down`         | Stop the whole stack               |

## Authentication

All `/requests` endpoints require a valid JWT token from Keycloak passed as:
```
Authorization: Bearer <token>
```

### Roles

| Role        | Permissions                                             |
|-------------|---------------------------------------------------------|
| `client`    | Create requests, view own requests only                 |
| `appraiser` | View all requests, update fields, change status         |
| `admin`     | View all requests, update fields                        |

## API Endpoints

| Method  | Path                    | Allowed roles                  | Description           |
|---------|-------------------------|--------------------------------|-----------------------|
| `GET`   | `/health`               | —                              | Health check          |
| `POST`  | `/requests`             | `client`                       | Create a request      |
| `GET`   | `/requests`             | `client`, `appraiser`, `admin` | List requests         |
| `GET`   | `/requests/{id}`        | `client`, `appraiser`, `admin` | Get request by ID     |
| `PATCH` | `/requests/{id}`        | `appraiser`, `admin`           | Update request fields |
| `PATCH` | `/requests/{id}/status` | `appraiser`                    | Change status         |

## Request lifecycle (the core domain rule)

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

Strictly linear, one step at a time, no going back. Enforced in the service layer; violations return HTTP 422.

## QA Testing Guide

A step-by-step guide for testers new to API testing (getting a Keycloak token, calling the API via
Swagger/curl/Postman, and the scenarios to cover) lives in [docs/qa-testing.md](docs/qa-testing.md).

## Development workflow

- Branch from `dev`: `feature/<scope>` or `fix/<scope>` (see branch history for examples)
- Conventional commits: `feat(requests): ...`, `fix(server): ...`; reference the Jira issue key (`ACRM-...`) in the subject
- Tasks are tracked in Jira, project **ACRM**
- PR into `dev`; CI-less for now — run `go build ./... && go vet ./... && go test ./...` before pushing
- Read the **Hard rules** section of [CLAUDE.md](CLAUDE.md) before your first PR — they are non-negotiable (no sync cross-service calls, no cross-DB joins, never edit applied migrations, etc.)
