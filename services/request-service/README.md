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
make migrate-up
```

**3. Run the service:**
```bash
make run
```

The service starts on port `8080` by default.

## Environment variables

| Variable       | Default                                                              | Description              |
|----------------|----------------------------------------------------------------------|--------------------------|
| `SERVER_PORT`  | `8080`                                                               | HTTP server port         |
| `DATABASE_URL` | `postgres://appraisal:appraisal@localhost:5433/request_db?sslmode=disable` | PostgreSQL connection URL |

## Make targets

| Command            | Description                        |
|--------------------|------------------------------------|
| `make run`         | Generate docs, run the service     |
| `make build`       | Generate docs, build the binary    |
| `make generate`    | Regenerate Swagger docs            |
| `make migrate-up`  | Apply all pending migrations       |
| `make migrate-down`| Roll back the last migration       |

## API

Swagger UI is available at `http://localhost:8080/swagger/index.html` after starting the service.

### Endpoints

| Method  | Path                        | Description              |
|---------|-----------------------------|--------------------------|
| `GET`   | `/health`                   | Health check             |
| `POST`  | `/requests`                 | Create a request         |
| `GET`   | `/requests?client_id=`      | List requests by client  |
| `GET`   | `/requests/{id}`            | Get request by ID        |
| `PATCH` | `/requests/{id}`            | Update request fields    |
| `PATCH` | `/requests/{id}/status`     | Change request status    |

### Status flow

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

Status can only move forward one step at a time. Skipping steps returns `422`.
