package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) RequestRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, req *domain.Request, event domain.EventEnvelope) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO requests (id, client_id, email, phone_number, inspector_id, object_type, address, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if _, err := tx.Exec(ctx, query,
		req.ID,
		req.ClientID,
		req.Email,
		req.PhoneNumber,
		req.InspectorID,
		req.ObjectType,
		req.Address,
		req.Status,
		req.CreatedAt,
		req.UpdatedAt,
	); err != nil {
		return err
	}

	if err := insertOutbox(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertOutbox writes the event into the outbox within the caller's tx, so the
// event and the state change commit atomically (ADR-007).
func insertOutbox(ctx context.Context, tx pgx.Tx, event domain.EventEnvelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO outbox (event_id, topic, event_type, aggregate_id, payload)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(ctx, query, event.EventID, domain.TopicRequestEvents, event.EventType, event.RequestID, payload)
	return err
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	query := `
		SELECT id, client_id, email, phone_number, inspector_id, object_type, address, status, created_at, updated_at
		FROM requests
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)

	var req domain.Request
	err := row.Scan(
		&req.ID,
		&req.ClientID,
		&req.Email,
		&req.PhoneNumber,
		&req.InspectorID,
		&req.ObjectType,
		&req.Address,
		&req.Status,
		&req.CreatedAt,
		&req.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &req, nil
}

// Update deliberately does not touch status — status changes go through ChangeStatus only.
func (r *postgresRepository) Update(ctx context.Context, req *domain.Request, prevUpdatedAt time.Time) error {
	query := `
		UPDATE requests
		SET inspector_id = $1, object_type = $2, address = $3, updated_at = $4
		WHERE id = $5 AND updated_at = $6
	`
	tag, err := r.db.Exec(ctx, query,
		req.InspectorID,
		req.ObjectType,
		req.Address,
		req.UpdatedAt,
		req.ID,
		prevUpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM requests WHERE id = $1)", req.ID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}
	return nil
}

func (r *postgresRepository) ChangeStatus(ctx context.Context, id uuid.UUID, oldStatus, newStatus domain.Status, updatedAt time.Time, event domain.EventEnvelope) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE requests SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4
	`
	tag, err := tx.Exec(ctx, query, newStatus, updatedAt, id, oldStatus)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// No row matched the CAS guard: the tx rolls back, so no outbox row is written.
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM requests WHERE id = $1)", id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}

	if err := insertOutbox(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *postgresRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error) {
	query := `
		SELECT id, client_id, email, phone_number, inspector_id, object_type, address, status, created_at, updated_at
		FROM requests
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]*domain.Request, 0)
	for rows.Next() {
		var req domain.Request
		err := rows.Scan(
			&req.ID,
			&req.ClientID,
			&req.Email,
			&req.PhoneNumber,
			&req.InspectorID,
			&req.ObjectType,
			&req.Address,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		requests = append(requests, &req)
	}
	return requests, rows.Err()
}

func (r *postgresRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	query := `
		SELECT id, client_id, email, phone_number, inspector_id, object_type, address, status, created_at, updated_at
		FROM requests
		WHERE client_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]*domain.Request, 0)
	for rows.Next() {
		var req domain.Request
		err := rows.Scan(
			&req.ID,
			&req.ClientID,
			&req.Email,
			&req.PhoneNumber,
			&req.InspectorID,
			&req.ObjectType,
			&req.Address,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		requests = append(requests, &req)
	}
	return requests, rows.Err()
}
