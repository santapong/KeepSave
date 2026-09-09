package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// ErrUserExists is returned by UserRepository.Create when the email is already registered.
var ErrUserExists = errors.New("email already registered")

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	msg := strings.ToLower(err.Error())
	// SQLite: "UNIQUE constraint failed". MySQL: "Error 1062: Duplicate entry".
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "duplicate key")
}

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
		).Scan(&user.ID, &user.Email, &user.PasswordHash, dbTime(&user.CreatedAt), dbTime(&user.UpdatedAt))
		if err != nil {
			if isUniqueViolation(err) {
				return nil, ErrUserExists
			}
			return nil, fmt.Errorf("creating user: %w", err)
		}
	} else {
		insertQ := Q(r.dialect, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`)
		_, err := r.db.Exec(insertQ, id, email, passwordHash)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, ErrUserExists
			}
			return nil, fmt.Errorf("creating user: %w", err)
		}
		selectQ := Q(r.dialect, `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`)
		err = r.db.QueryRow(selectQ, id).Scan(&user.ID, &user.Email, &user.PasswordHash, dbTime(&user.CreatedAt), dbTime(&user.UpdatedAt))
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
	).Scan(&user.ID, &user.Email, &user.PasswordHash, dbTime(&user.CreatedAt), dbTime(&user.UpdatedAt))
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
	).Scan(&user.ID, &user.Email, &user.PasswordHash, dbTime(&user.CreatedAt), dbTime(&user.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}
