---
paths:
  - "services/**/*.go"
---

# Go service conventions

## Error handling

Define domain errors in `internal/domain/errors.go` as typed sentinel errors.
Map them to HTTP status codes in the handler layer only — never in service or repository.

```go
// domain/errors.go
var (
    ErrNotFound      = errors.New("not found")
    ErrForbidden     = errors.New("forbidden")
    ErrInvalidStatus = errors.New("invalid status transition")
    ErrConflict      = errors.New("conflict") // optimistic locking
)
```

## Handler structure

```go
func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
    // 1. Extract path/query params
    // 2. Extract JWT claims from context (middleware already validated)
    // 3. Call service
    // 4. Map domain error → HTTP status
    // 5. httputil.WriteJSON(w, http.StatusOK, resp)
}
```

## Middleware

- JWT validation: via Keycloak JWKS endpoint only — no shared secrets in services
- Roles from JWT claims: `realm_access.roles`
- `RequireRoles("appraiser", "admin")` — variadic, passes if user has any of the listed roles

## Tests

- Test the service layer using a mock repository (interface-based)
- Table-driven tests for state machine transitions — mandatory
- Do not test handlers directly unless there is a strong reason

## Kafka

```go
// Publish event AFTER the transaction commits, never inside it.
// Use at-least-once semantics — consumers must be idempotent.
if err := tx.Commit(ctx); err != nil { return err }
_ = s.producer.Publish(ctx, event)
```
