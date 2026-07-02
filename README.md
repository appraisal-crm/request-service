# Appraisal CRM

> English version · [Русская версия](i18n/ru/README.md)

CRM for a property appraisal company (apartments, houses, land, vehicles, commercial real estate).
Digitizes the full cycle: client submits a request → inspector visits the property → appraiser evaluates → client receives the report.

Commercial project built for a real client. Code goes to production.

> **New to the project?** Start with the [Developer Onboarding Guide](docs/onboarding.md).

## Repository map

| Path | What lives there |
|------|------------------|
| `services/request-service/` | First business service (Go): appraisal requests CRUD + lifecycle state machine. **The reference implementation for all future services.** |
| `infra/docker-compose.yml` | Local infrastructure: PostgreSQL 17, Redis 7, Keycloak 26 |
| `docs/brd/` | Business Requirements Document (en/ru) |
| `docs/architecture/` | C4 diagrams in Structurizr DSL (en/ru) |
| `docs/adr/` | Architecture Decision Records (en/ru) |
| `docs/onboarding.md` | Developer onboarding guide — start here |
| `CLAUDE.md` | Project conventions and hard rules (read it even if you don't use Claude) |

## Architecture at a glance

Microservices in Go, one PostgreSQL database per service, async communication via Kafka events only (no direct service-to-service HTTP calls), Keycloak for OAuth2/OIDC, four React SPAs per role behind an API Gateway.

**Implemented today:** infrastructure compose + `request-service` (CRUD, state machine, JWT/RBAC, Swagger, unit tests).
**Not yet:** API Gateway, inspect-service, review-service (blocked on client formulas), notification-service, frontends, Kafka wiring.

See [C4 diagrams](docs/architecture/README.md) for the target picture and [ADRs](docs/adr/README.md) for the reasoning behind the stack.

## Quickstart

```bash
# 1. Infrastructure (PostgreSQL :5433, Redis :6380, Keycloak :8180)
docker compose -f infra/docker-compose.yml up -d

# 2. Keycloak bootstrap — the compose starts Keycloak EMPTY, one-time setup required:
#    realm `appraisal`, roles, a public client, test users.
#    Follow docs/onboarding.md § "Keycloak setup" (5 minutes, copy-paste commands).

# 3. Migrations + run the service
cd services/request-service
make migrate-up
make run          # generates Swagger docs, starts on :8080
```

Then open Swagger UI at http://localhost:8080/swagger/index.html.
Getting a token and calling the API: see the [request-service README](services/request-service/README.md).

## Development workflow

- Branch from `dev`: `feature/<scope>` or `fix/<scope>` (see branch history for examples)
- Conventional commits: `feat(requests): ...`, `fix(server): ...`; reference the Jira issue key (`ACRM-...`) in the subject
- Tasks are tracked in Jira, project **ACRM**
- PR into `dev`; CI-less for now — run `go build ./... && go vet ./... && go test ./...` before pushing
- Read the **Hard rules** section of [CLAUDE.md](CLAUDE.md) before your first PR — they are non-negotiable (no sync cross-service calls, no cross-DB joins, never edit applied migrations, etc.)

## Request lifecycle (the core domain rule)

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

Strictly linear, one step at a time, no going back. Enforced in the service layer; violations return HTTP 422.
