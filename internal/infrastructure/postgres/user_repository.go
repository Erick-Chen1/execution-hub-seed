package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/user"
)

// UserRepository implements user.Repository.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users
		(user_id, username, password_hash, role, user_type, owner_user_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, u.UserID, u.Username, u.PasswordHash, u.Role, u.Type, u.OwnerUserID, u.Status, u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users
		SET username=$1, password_hash=$2, role=$3, user_type=$4, owner_user_id=$5, status=$6, updated_at=$7
		WHERE user_id=$8
	`, u.Username, u.PasswordHash, u.Role, u.Type, u.OwnerUserID, u.Status, u.UpdatedAt, u.UserID)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, username, password_hash, role, user_type, owner_user_id, status, created_at, updated_at
		FROM users WHERE user_id=$1
	`, userID)
	return scanUser(row)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, username, password_hash, role, user_type, owner_user_id, status, created_at, updated_at
		FROM users WHERE username=$1
	`, username)
	return scanUser(row)
}

func (r *UserRepository) List(ctx context.Context, filter user.Filter, limit, offset int) ([]*user.User, error) {
	query := `SELECT id, user_id, username, password_hash, role, user_type, owner_user_id, status, created_at, updated_at FROM users`
	args := []interface{}{}
	idx := 1
	if filter.Role != nil {
		query += " WHERE role=$" + itoa(idx)
		args = append(args, *filter.Role)
		idx++
	}
	if filter.Type != nil {
		query += addWhere(query) + " user_type=$" + itoa(idx)
		args = append(args, *filter.Type)
		idx++
	}
	if filter.Status != nil {
		query += addWhere(query) + " status=$" + itoa(idx)
		args = append(args, *filter.Status)
		idx++
	}
	if filter.OwnerUserID != nil {
		query += addWhere(query) + " owner_user_id=$" + itoa(idx)
		args = append(args, *filter.OwnerUserID)
		idx++
	}
	if filter.Username != nil {
		query += addWhere(query) + " username=$" + itoa(idx)
		args = append(args, *filter.Username)
		idx++
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*user.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	row := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var ownerID *uuid.UUID
	if err := row.Scan(&u.ID, &u.UserID, &u.Username, &u.PasswordHash, &u.Role, &u.Type, &ownerID, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.OwnerUserID = ownerID
	return &u, nil
}
