package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/models"
)

type UserRepo struct{ db *DB }

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Db() *DB { return r.db }
func (r *UserRepo) q(ctx context.Context) DBTX { return r.db.Querier(ctx) }

func (r *UserRepo) GetByEmail(ctx context.Context, schoolID uuid.UUID, email string) (*models.User, string, error) {
	// Called during login: we must set app.current_school_id for RLS even though
	// the JWT doesn't exist yet — we already resolved schoolID from school_code.
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin login tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id',$1,true)", schoolID.String()); err != nil {
		return nil, "", fmt.Errorf("set login tenant: %w", err)
	}
	row := tx.QueryRow(ctx,
		`SELECT id, school_id, role, full_name, email, password_hash, phone, is_active, created_at, updated_at
		 FROM users WHERE school_id=$1 AND email=$2`, schoolID, email)
	u := &models.User{}
	var hash string
	if err := row.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &hash, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, "", fmt.Errorf("user not found: %w", err)
	}
	return u, hash, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT id, school_id, role, full_name, email, phone, is_active, created_at, updated_at
		 FROM users WHERE id=$1`, id)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (r *UserRepo) ListTeachers(ctx context.Context, schoolID uuid.UUID, limit, offset int) ([]*models.User, int64, error) {
	var total int64
	if err := r.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE school_id=$1 AND role='teacher'`, schoolID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count teachers: %w", err)
	}

	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, school_id, role, full_name, email, phone, is_active, created_at, updated_at
		 FROM users WHERE school_id=$1 AND role='teacher' ORDER BY full_name ASC LIMIT $2 OFFSET $3`,
		schoolID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}
	defer rows.Close()

	users := make([]*models.User, 0)
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("teachers rows err: %w", err)
	}
	return users, total, nil
}

func (r *UserRepo) Create(ctx context.Context, schoolID uuid.UUID, role models.UserRole, fullName, email, passwordHash string, phone *string) (*models.User, error) {
	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO users(school_id, role, full_name, email, password_hash, phone)
		 VALUES($1,$2,$3,$4,$5,$6)
		 RETURNING id, school_id, role, full_name, email, phone, is_active, created_at, updated_at`,
		schoolID, role, fullName, email, passwordHash, phone)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, fullName string, phone *string) (*models.User, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE users SET full_name=$2, phone=$3, updated_at=now() WHERE id=$1
		 RETURNING id, school_id, role, full_name, email, phone, is_active, created_at, updated_at`,
		id, fullName, phone)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE users SET is_active=false, updated_at=now() WHERE id=$1`, id)
	return err
}

func (r *UserRepo) Activate(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE users SET is_active=true, updated_at=now() WHERE id=$1`, id)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, _ = r.q(ctx).Exec(ctx, `UPDATE classes SET class_teacher_id=NULL WHERE class_teacher_id=$1`, id)
	_, err := r.q(ctx).Exec(ctx,
		`DELETE FROM users WHERE id=$1`, id)
	return err
}

func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, id, hash)
	return err
}

func (r *UserRepo) GetPasswordHashByID(ctx context.Context, id uuid.UUID) (string, error) {
	var hash string
	err := r.q(ctx).QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id=$1`, id).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// StoreRefreshToken saves a hashed refresh token.
// Uses the pool directly — refresh_tokens has no RLS, and this is called at login
// before a tenant transaction exists in context.
func (r *UserRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt interface{}) error {
	_, err := r.db.pool.Exec(ctx,
		`INSERT INTO refresh_tokens(user_id, token_hash, expires_at) VALUES($1,$2,$3)`,
		userID, tokenHash, expiresAt)
	return err
}

// GetRefreshToken retrieves an unrevoked, unexpired refresh token by its hash.
func (r *UserRepo) GetRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.db.pool.QueryRow(ctx,
		`SELECT user_id FROM refresh_tokens
		 WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at > now()`, tokenHash).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("refresh token not found: %w", err)
	}
	return userID, nil
}

// RevokeRefreshToken marks a specific token as revoked.
func (r *UserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1`, tokenHash)
	return err
}

// RevokeAllRefreshTokens revokes every token for a user (on password reset).
// Uses pool directly — called from admin reset-password which does have a tenant tx,
// but refresh_tokens has no RLS so pool is safe either way.
func (r *UserRepo) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}

// GetByIDNoRLS fetches a user by ID during token refresh (no JWT context yet).
// Uses the get_user_school_id() SECURITY DEFINER function to resolve school_id
// (bypasses RLS for that one lookup), then opens a scoped tx to fetch the full user row.
func (r *UserRepo) GetByIDNoRLS(ctx context.Context, id uuid.UUID) (*models.User, uuid.UUID, models.UserRole, error) {
	var schoolID uuid.UUID
	if err := r.db.pool.QueryRow(ctx, `SELECT get_user_school_id($1)`, id).Scan(&schoolID); err != nil {
		return nil, uuid.Nil, "", fmt.Errorf("resolve school for user: %w", err)
	}
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, "", fmt.Errorf("begin refresh tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id',$1,true)", schoolID.String()); err != nil {
		return nil, uuid.Nil, "", fmt.Errorf("set refresh tenant: %w", err)
	}
	row := tx.QueryRow(ctx,
		`SELECT id, school_id, role, full_name, email, phone, is_active, created_at, updated_at
		 FROM users WHERE id=$1`, id)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.SchoolID, &u.Role, &u.FullName, &u.Email, &u.Phone,
		&u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, uuid.Nil, "", fmt.Errorf("user not found: %w", err)
	}
	return u, u.SchoolID, u.Role, nil
}
