# Request Service

Manages appraisal requests lifecycle from creation to closure.

## Requirements

- Go 1.22+
- Docker & Docker Compose
- [migrate CLI](https://github.com/golang-migrate/migrate)
- [swag CLI](https://github.com/swaggo/swag)

## Setup

**1. Start infrastructure:**
```bash
cd ../../infra
cp .env.example .env
docker compose up -d
```

**2. Apply migrations:**
```bash
cd ../services/request-service
make migrate-up
```

**3. Run the service:**
```bash
make run
```

The service starts on port `8080` by default.

## Environment variables

| Variable       | Default                                                                    | Description               |
|----------------|----------------------------------------------------------------------------|---------------------------|
| `SERVER_PORT`  | `8080`                                                                     | HTTP server port          |
| `DATABASE_URL` | `postgres://appraisal:appraisal@localhost:5433/request_db?sslmode=disable` | PostgreSQL connection URL |
| `JWKS_URL`     | `http://localhost:8180/realms/appraisal/protocol/openid-connect/certs`     | Keycloak JWKS endpoint    |
| `ALLOWED_ORIGINS` | `*`                                                                   | Comma-separated CORS origins (`*` for local dev only) |

## Make targets

| Command             | Description                        |
|---------------------|------------------------------------|
| `make run`          | Generate docs, run the service     |
| `make build`        | Generate docs, build the binary    |
| `make generate`     | Regenerate Swagger docs            |
| `make test`         | Run unit tests                     |
| `make migrate-up`   | Apply all pending migrations       |
| `make migrate-down` | Roll back the last migration       |

---

## QA Testing Guide

This section is for testers who are new to API testing.

### What is a token and why do I need it?

The API is protected. Before you can make any request (except `/health`), you need to prove who you are. You do this by getting a **token** from Keycloak (our authentication service) and attaching it to every request.

Think of it like a badge — you get it at the entrance, then show it every time you go through a door.

A token looks like a long random string:
```
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0...
```

Tokens expire after **5 minutes**. If you get a `401 Unauthorized` error, just get a new token.

---

### Step 1 — Create a test user in Keycloak

1. Open [http://localhost:8180](http://localhost:8180) in your browser
2. Log in with `admin` / `admin`
3. Make sure the `appraisal` realm is selected (top left dropdown)
4. Go to **Users** → **Create user**
5. Fill in username, email, first name, last name → **Create**
6. Go to the **Credentials** tab → **Set password** → enter a password, turn off **Temporary** → **Save password**
7. Go to the **Role mapping** tab → **Assign role** → switch filter to **Filter by realm roles** → select a role (e.g. `client`) → **Assign**

---

### Step 2 — Get a token

#### Using curl

Run this in the terminal (replace `<username>` and `<password>` with your test user's credentials):

```bash
curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=appraisal-frontend&username=<username>&password=<password>" \
  | jq -r .access_token
```

Copy the output — that's your token.

#### Using Postman

1. Create a new request → method `POST`
2. URL: `http://localhost:8180/realms/appraisal/protocol/openid-connect/token`
3. Go to the **Body** tab → select `x-www-form-urlencoded`
4. Add these fields:

| Key            | Value                  |
|----------------|------------------------|
| `grant_type`   | `password`             |
| `client_id`    | `appraisal-frontend`   |
| `username`     | your test username     |
| `password`     | your test password     |

5. Click **Send** — copy the `access_token` value from the response

---

### Step 3 — Make API requests

#### Using Swagger UI

1. Open [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
2. Click **Authorize** (top right, lock icon)
3. In the `BearerAuth` field enter your token (without the word "Bearer")
4. Click **Authorize** → **Close**
5. Now you can expand any endpoint and click **Try it out** → **Execute**

#### Using curl

Attach the token with `-H "Authorization: Bearer <token>"`:

```bash
# Get token and save it to a variable
TOKEN=$(curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=appraisal-frontend&username=<username>&password=<password>" \
  | jq -r .access_token)

# Create a request
curl -s -X POST http://localhost:8080/requests \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"object_type": "apartment", "address": "Moscow, Lenina 1"}'
```

#### Using Postman

1. Create a new request
2. Go to the **Authorization** tab → Type: `Bearer Token`
3. Paste your token in the **Token** field
4. Set the method, URL, and body as needed → **Send**

---

### What to test

#### Roles and access control

| Scenario | Expected result |
|---|---|
| Request without a token | `401 Unauthorized` |
| `client` creates a request | `201 Created` |
| `appraiser` tries to create a request | `403 Forbidden` |
| `client` views their own request | `200 OK` |
| `client` tries to view someone else's request | `403 Forbidden` |
| `appraiser` views any request | `200 OK` |
| `appraiser` changes request status | `200 OK` |
| `client` tries to change status | `403 Forbidden` |

#### Status flow

Statuses must change strictly in order — skipping is not allowed:

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

| Scenario | Expected result |
|---|---|
| Move from `new` to `in_progress` | `200 OK` |
| Move from `new` to `closed` (skip) | `422 Unprocessable Entity` |
| Move backwards (e.g. `in_progress` to `new`) | `422 Unprocessable Entity` |

#### Optional fields

When creating a request, `object_type` and `address` are optional — the appraiser can fill them in later.

| Scenario | Expected result |
|---|---|
| Create with no body `{}` | `201 Created` |
| Create with `object_type` and `address` | `201 Created` |

---

## Authentication (developer reference)

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

---

## API Endpoints

| Method  | Path                    | Allowed roles                  | Description           |
|---------|-------------------------|--------------------------------|-----------------------|
| `GET`   | `/health`               | —                              | Health check          |
| `POST`  | `/requests`             | `client`                       | Create a request      |
| `GET`   | `/requests`             | `client`, `appraiser`, `admin` | List requests         |
| `GET`   | `/requests/{id}`        | `client`, `appraiser`, `admin` | Get request by ID     |
| `PATCH` | `/requests/{id}`        | `appraiser`, `admin`           | Update request fields |
| `PATCH` | `/requests/{id}/status` | `appraiser`                    | Change status         |

## Status flow

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

Status can only move forward one step at a time. Skipping steps returns `422`.
