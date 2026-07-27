package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/models"
)

type SchoolRepo struct {
	pool *pgxpool.Pool
}

func NewSchoolRepo(pool *pgxpool.Pool) *SchoolRepo {
	return &SchoolRepo{pool: pool}
}

// GetByCode resolves a school by its human-readable code at login time.
// This query runs WITHOUT RLS (schools table has no RLS policy) because
// we need to identify the tenant before we know the school_id.
func (r *SchoolRepo) GetByCode(ctx context.Context, code string) (*models.School, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, code, is_active, created_at, updated_at
		 FROM schools WHERE code = $1`, code)
	s := &models.School{}
	if err := row.Scan(&s.ID, &s.Name, &s.Code, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("school not found: %w", err)
	}
	return s, nil
}

func (r *SchoolRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.School, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, code, is_active, created_at, updated_at
		 FROM schools WHERE id = $1`, id)
	s := &models.School{}
	if err := row.Scan(&s.ID, &s.Name, &s.Code, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("school not found: %w", err)
	}
	return s, nil
}

func (r *SchoolRepo) Create(ctx context.Context, name, code string) (*models.School, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO schools(name, code) VALUES($1,$2)
		 RETURNING id, name, code, is_active, created_at, updated_at`, name, code)
	s := &models.School{}
	if err := row.Scan(&s.ID, &s.Name, &s.Code, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create school: %w", err)
	}
	return s, nil
}
