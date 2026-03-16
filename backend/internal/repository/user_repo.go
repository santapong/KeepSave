package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type UserRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewUserRepository(db *sql.DB, dialect Dialect) *UserRepository {
	return &UserRepository{db: db, dialect: dialect}
}

func (r *UserRepository) Create(email, passwordHash string) (*models.User, error) {
	user := &models.User{}
	id := uuid.New()

	if r.dialect.SupportsReturning() {
		err := r.db.QueryRow(
			`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)
			 RETURNING id, email, password_hash, created_at, updated_at`,
			id, email, passwordHash,
		).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating user: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`)
		_, err := r.db.Exec(insertQ, id, email, passwordHash)
		if err != nil {
			return nil, fmt.Errorf("creating user: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("reading created user: %w", err)
		}
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`),
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		Q(r.dialect, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`),
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}
