# Developer Onboarding Guide

> English version · [Русская версия](i18n/ru/onboarding.md)

For a developer joining the project who knows Go but has never seen this codebase.
Read top to bottom once; after that you'll mostly come back for the recipes and gotchas.

## Table of contents

1. [The big picture](#1-the-big-picture)
2. [Local environment setup](#2-local-environment-setup)
3. [Keycloak setup](#3-keycloak-setup)
4. [Codebase tour: request-service](#4-codebase-tour-request-service)
5. [Patterns you must follow](#5-patterns-you-must-follow)
6. [HTTP status code conventions](#6-http-status-code-conventions)
7. [Recipes](#7-recipes)
8. [Gotchas](#8-gotchas)
9. [Workflow and process](#9-workflow-and-process)

---

## 1. The big picture

The system automates a property appraisal company. Business flow:

1. **Client** submits an appraisal request (apartment / house / land / commercial / car)
2. **Appraiser** accepts it and assigns an **inspector**
3. Inspector visits the property, uploads photos and data
4. Appraiser produces the report; client downloads it

Architecture principles (see [ADRs](adr/README.md) for the "why"):

- **Microservices in Go** — chi, pgx, golang-migrate; no frameworks beyond that
- **Database per service** — each service owns its PostgreSQL database; cross-database JOINs are forbidden
- **Events over Kafka only** — business services never call each other synchronously
- **Keycloak** — OAuth2/OIDC; services validate JWTs against Keycloak's JWKS endpoint, roles come from token claims
- **Four React SPAs** (client / appraiser / inspector / admin) behind an API Gateway

What exists vs the target picture ([C4 diagrams](architecture/README.md)):

| Component | Status |
|-----------|--------|
| `request-service` | ✅ working MVP — **the reference for everything else** |
| Infrastructure compose (PostgreSQL, Redis, Keycloak) | ✅ |
| Kafka + event wiring | ❌ not yet (next milestone) |
| API Gateway | ❌ |
| inspect-service, notification-service | ❌ |
| review-service | ❌ **blocked** — appraisal formulas not formalized by the client; do not start it |
| Frontends (4 SPAs) | ❌ |

The core domain rule — the request lifecycle is strictly linear, enforced in code:

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

One step at a time, no skipping, no going back. Ever.

## 2. Local environment setup

Requirements: Go 1.22+, Docker, [migrate CLI](https://github.com/golang-migrate/migrate), [swag CLI](https://github.com/swaggo/swag).

```bash
# 1. Infrastructure
docker compose -f infra/docker-compose.yml up -d
# PostgreSQL → localhost:5433 (user/pass: appraisal/appraisal)
# Redis      → localhost:6380
# Keycloak   → localhost:8180 (admin/admin)

# 2. Keycloak bootstrap — see section 3, required once per fresh volume

# 3. Migrations
cd services/request-service
make migrate-up

# 4. Run
make run     # swag generate + go run, listens on :8080
```

Verify: `curl localhost:8080/health` → `{"status":"ok"}`, Swagger UI at http://localhost:8080/swagger/index.html.

## 3. Keycloak setup

**The compose starts Keycloak with an empty database** — the `appraisal` realm, roles, and users must be created once (they persist in the `postgres_data` volume afterwards). Copy-paste bootstrap using `kcadm` inside the container:

```bash
KC="docker exec appraisal-keycloak /opt/keycloak/bin/kcadm.sh"

# login
$KC config credentials --server http://localhost:8080 --realm master --user admin --password admin

# realm + roles
$KC create realms -s realm=appraisal -s enabled=true
for r in client appraiser inspector admin; do $KC create roles -r appraisal -s name=$r; done

# public client with password grant (for local tokens / Postman / frontends)
$KC create clients -r appraisal -s clientId=appraisal-frontend \
  -s publicClient=true -s directAccessGrantsEnabled=true -s enabled=true

# test users (email/firstName/lastName required, otherwise token requests fail
# with "Account is not fully set up")
for u in test-client test-appraiser; do
  $KC create users -r appraisal -s username=$u -s enabled=true \
    -s email=$u@example.com -s firstName=Test -s lastName=User -s emailVerified=true
  $KC set-password -r appraisal --username $u --new-password test123
done
$KC add-roles -r appraisal --uusername test-client --rolename client
$KC add-roles -r appraisal --uusername test-appraiser --rolename appraiser
```

Get a token (expires in 5 minutes):

```bash
TOKEN=$(curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -d "grant_type=password&client_id=appraisal-frontend&username=test-client&password=test123" \
  | jq -r .access_token)

curl -s -X POST localhost:8080/requests \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","phone_number":"+79161234567","object_type":"apartment","address":"Lenina 1"}'
```

## 4. Codebase tour: request-service

Every future service copies this structure — internalize it once.

```
services/request-service/
  cmd/server/main.go       ← entry point: config, pgx pool, JWKS, DI wiring, http.Server
  config/config.go         ← ENV config, os.Getenv only (DATABASE_URL is required)
  internal/
    domain/request.go      ← Request entity, Status and ObjectType enums
    repository/
      repository.go        ← RequestRepository interface + sentinel errors (ErrNotFound, ErrConflict)
      postgres.go          ← pgx implementation, raw SQL
    service/
      service.go           ← RequestService interface, CreateInput/UpdateInput DTOs
      request.go           ← business logic: state machine, optimistic locking, domain errors
      request_test.go      ← unit tests with a testify/mock repository
    handler/
      route.go             ← chi router: middleware chain + RBAC per route
      request.go           ← HTTP handlers: parse → validate → call service → map error → respond
      dto.go               ← request DTOs with `validate:` tags
      validate.go          ← go-playground/validator instance
      health.go, response.go
    middleware/auth.go     ← JWT via Keycloak JWKS (MicahParks/keyfunc), roles from realm_access.roles
    httputil/response.go   ← shared RespondJSON / RespondError
  migrations/              ← golang-migrate SQL, sequential numbering, always up+down pairs
  docs/                    ← swag-generated; GITIGNORED — regenerate with `make generate`
```

**Request flow through the layers**, using `PATCH /requests/{id}/status` as the example:

1. `middleware.Auth` validates the JWT against Keycloak JWKS, puts user ID + roles into context
2. `middleware.RequireRoles("appraiser")` gates the route (403 otherwise)
3. Handler parses the UUID, decodes the body (1 MB limit), runs validator tags → 400 on failure
4. Service loads the request, checks `allowedTransitions` map → `ErrInvalidStatusTransition` if the step is illegal
5. Repository runs a **compare-and-set** UPDATE: `... SET status=$new WHERE id=$id AND status=$old`. Zero rows affected + row exists ⇒ someone raced us ⇒ `ErrConflict`
6. Handler maps the domain error to HTTP: 404 / 409 / 422 / 500 — **status codes are chosen in the handler layer only**

**Optimistic locking** (no `version` column):
- `ChangeStatus` — CAS on the current `status` value
- `Update` (field patch) — CAS on `updated_at`; deliberately does **not** write `status`, so a concurrent status change can never be rolled back by a stale field update
- Both surface as `ErrConflict` → HTTP 409; the client retries

**RBAC snapshot** (from `route.go` — the source of truth):

| Endpoint | client | appraiser | admin | inspector |
|----------|--------|-----------|-------|-----------|
| POST /requests | ✅ | — | — | — |
| GET /requests (list) | own only | all + pagination | all | — |
| GET /requests/{id} | own only | ✅ | ✅ | — |
| PATCH /requests/{id} | — | ✅ | ✅ | — |
| PATCH /requests/{id}/status | — | ✅ | — | — |

Admin intentionally cannot create requests or drive the lifecycle; inspector gets access when inspect-service arrives. If that surprises you — it's a product decision, not an oversight.

## 5. Patterns you must follow

These come from [CLAUDE.md](../CLAUDE.md) (hard rules) and `.claude/rules/go-services.md`:

- **No frameworks** beyond chi, pgx, playground/validator. Config via `os.Getenv` only — no viper/cobra.
- **Domain errors** are typed sentinels mapped to HTTP codes **in the handler layer only**. New services put them in `internal/domain/errors.go` (request-service keeps them in `service/request.go` — legacy, don't copy that).
- **Migrations**: `00000N_description.up.sql` + `.down.sql`, sequential, both directions always. **Never modify an applied migration** — only add new ones.
- **Events**: each Kafka event is a distinct type in `domain/events.go`; publish **after** the DB transaction commits; consumers must be idempotent (at-least-once).
- **No synchronous calls between business services**, no cross-database JOINs.
- **Swagger annotations** on every public endpoint (`make generate` before committing handler changes).
- **Unit tests** for business logic live in `service/` with a mocked repository; state-machine transitions get table-driven tests — mandatory.

## 6. HTTP status code conventions

| Code | Meaning here | Source |
|------|--------------|--------|
| 400 | Malformed JSON, failed validation, bad UUID in path | handler |
| 401 | Missing/invalid/expired JWT | middleware |
| 403 | Role not allowed, or client touching someone else's request | middleware / handler |
| 404 | Request not found (`ErrNotFound`) | service → handler |
| 409 | Optimistic-lock conflict (`ErrConflict`) — retry | service → handler |
| 422 | Illegal state-machine transition (`ErrInvalidStatusTransition`) | service → handler |
| 500 | Anything unexpected; generic message, details only in logs | handler |

## 7. Recipes

**Add an endpoint:** DTO with validate tags in `dto.go` → handler method with Swagger annotations → route + `RequireRoles` in `route.go` → service method on the interface + implementation → repository if needed → unit tests for the service logic → `make generate`.

**Add a migration:** next sequential number, both `.up.sql` and `.down.sql`, additive only. Apply with `make migrate-up`, verify rollback with `make migrate-down`.

**Add a new service:** copy the request-service layout (structure above), module path `github.com/Meidorislav/appraisal-crm/services/<name>-service`. Domain errors go in `domain/errors.go`. Add its database to `infra/postgres/init/01-create-databases.sql` (fresh volumes only) and wire config via ENV.

**Run everything before pushing:**
```bash
cd services/request-service
make generate && go build ./... && go vet ./... && go test ./...
```

## 8. Gotchas

- **`go build ./...` fails on a fresh clone** with `no required module provides package .../docs` — the Swagger `docs/` package is gitignored. Run `make generate` (or `make build`, which does it for you) first.
- **Keycloak is empty after `docker compose up`** — no realm, no users. Section 3 fixes it; the state persists in the `postgres_data` volume until you `docker compose down -v`.
- **Token requests fail with "Account is not fully set up"** — the Keycloak user is missing email/first/last name or has pending required actions. The bootstrap in section 3 sets them.
- **Tokens expire in 5 minutes** — a sudden 401 in Postman usually means "get a new token", not "auth is broken".
- **Non-standard ports** to avoid collisions: PostgreSQL **5433**, Redis **6380**, Keycloak **8180**.
- **Logs are mixed-format** for now: chi's text request log + slog JSON. Known issue, don't be surprised.
- **`DATABASE_URL` is required** — the service exits immediately without it. Other ENV vars have dev defaults (`config/config.go`).
- **PATCH returning 409** is not an error in your code — it's optimistic locking telling you to re-read and retry.

## 9. Workflow and process

- Tasks live in Jira, project **ACRM**. Take a card, move it to In Progress.
- Branch from `dev`: `feature/<scope>` or `fix/<scope>`.
- Conventional commits with the Jira key: `fix(requests): ... (ACRM-84)`.
- PR into `dev`. No CI yet — the pre-push command in section 7 is your gate.
- Business docs: [BRD](brd/README.md). Architecture: [C4](architecture/README.md), [ADRs](adr/README.md). Conventions: [CLAUDE.md](../CLAUDE.md).
- review-service is **blocked** by the client (no formalized appraisal formulas) — don't pick it up, whatever the backlog says.
