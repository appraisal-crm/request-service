package repository

import (
	"context"
	"time"

	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) RequestRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, req *domain.Request) error {
	query := `
		INSERT INTO requests (id, client_id, inspector_id, object_type, address, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		req.ID,
		req.ClientID,
		req.InspectorID,
		req.ObjectType,
		req.Address,
		req.Status,
		req.CreatedAt,
		req.UpdatedAt,
	)
	return err
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	query := `
		SELECT id, client_id, inspector_id, object_type, address, status, created_at, updated_at
		FROM requests
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)

	var req domain.Request
	err := row.Scan(
		&req.ID,
		&req.ClientID,
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
	return &req, nil
}

func (r *postgresRepository) Update(ctx context.Context, req *domain.Request) error {
	query := `
		UPDATE requests
		SET inspector_id = $1, object_type = $2, address = $3, status = $4, updated_at = $5
		WHERE id = $6
	`
	req.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx, query,
		req.InspectorID,
		req.ObjectType,
		req.Address,
		req.Status,
		req.UpdatedAt,
		req.ID,
	)
	return err
}

func (r *postgresRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	query := `
		SELECT id, client_id, inspector_id, object_type, address, status, created_at, updated_at
		FROM requests
		WHERE client_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*domain.Request
	for rows.Next() {
		var req domain.Request
		err := rows.Scan(
			&req.ID,
			&req.ClientID,
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
