package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type ApplicationRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewApplicationRepository(db *sql.DB, dialect Dialect) *ApplicationRepository {
	return &ApplicationRepository{db: db, dialect: dialect}
}

func (r *ApplicationRepository) Create(app *models.Application) error {
	id := uuid.New()
	query := Q(r.dialect, `INSERT INTO applications (id, name, url, description, icon, category, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`)
	_, err := r.db.Exec(query, id, app.Name, app.URL, app.Description, app.Icon, app.Category, app.OwnerID)
	if err != nil {
		return fmt.Errorf("creating application: %w", err)
	}
	app.ID = id
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	return nil
}

func (r *ApplicationRepository) GetByID(id uuid.UUID) (*models.Application, error) {
	app := &models.Application{}
	query := Q(r.dialect, `SELECT id, name, url, description, icon, category, owner_id, created_at, updated_at
		FROM applications WHERE id = $1`)
	err := r.db.QueryRow(query, id).Scan(
		&app.ID, &app.Name, &app.URL, &app.Description, &app.Icon, &app.Category, &app.OwnerID,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting application: %w", err)
	}
	return app, nil
}

func (r *ApplicationRepository) ListByOwner(ownerID uuid.UUID, search, category string) ([]models.Application, error) {
	baseQuery := `SELECT id, name, url, description, icon, category, owner_id, created_at, updated_at
		FROM applications WHERE owner_id = $1`
	args := []interface{}{ownerID}
	argIdx := 2

	if search != "" {
		baseQuery += fmt.Sprintf(` AND (LOWER(name) LIKE LOWER($%d) OR LOWER(description) LIKE LOWER($%d))`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if category != "" && category != "All" {
		baseQuery += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}

	baseQuery += ` ORDER BY created_at DESC`
	query := Q(r.dialect, baseQuery)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing applications: %w", err)
	}
	defer rows.Close()

	var apps []models.Application
	for rows.Next() {
		var app models.Application
		if err := rows.Scan(&app.ID, &app.Name, &app.URL, &app.Description, &app.Icon, &app.Category,
			&app.OwnerID, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning application: %w", err)
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (r *ApplicationRepository) Update(app *models.Application) error {
	query := Q(r.dialect, `UPDATE applications SET name = $1, url = $2, description = $3, icon = $4, category = $5, updated_at = NOW()
		WHERE id = $6 AND owner_id = $7`)
	result, err := r.db.Exec(query, app.Name, app.URL, app.Description, app.Icon, app.Category, app.ID, app.OwnerID)
	if err != nil {
		return fmt.Errorf("updating application: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("application not found or not owned by user")
	}
	return nil
}

func (r *ApplicationRepository) Delete(id, ownerID uuid.UUID) error {
	query := Q(r.dialect, `DELETE FROM applications WHERE id = $1 AND owner_id = $2`)
	result, err := r.db.Exec(query, id, ownerID)
	if err != nil {
		return fmt.Errorf("deleting application: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("application not found or not owned by user")
	}
	return nil
}

// Favorites

func (r *ApplicationRepository) ToggleFavorite(userID, appID uuid.UUID) (bool, error) {
	// Check if favorite exists
	var existingID uuid.UUID
	checkQuery := Q(r.dialect, `SELECT id FROM application_favorites WHERE user_id = $1 AND application_id = $2`)
	err := r.db.QueryRow(checkQuery, userID, appID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Add favorite
		id := uuid.New()
		insertQuery := Q(r.dialect, `INSERT INTO application_favorites (id, user_id, application_id) VALUES ($1, $2, $3)`)
		_, err = r.db.Exec(insertQuery, id, userID, appID)
		if err != nil {
			return false, fmt.Errorf("adding favorite: %w", err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking favorite: %w", err)
	}

	// Remove favorite
	deleteQuery := Q(r.dialect, `DELETE FROM application_favorites WHERE id = $1`)
	_, err = r.db.Exec(deleteQuery, existingID)
	if err != nil {
		return false, fmt.Errorf("removing favorite: %w", err)
	}
	return false, nil
}

func (r *ApplicationRepository) GetFavoriteAppIDs(userID uuid.UUID) (map[uuid.UUID]bool, error) {
	query := Q(r.dialect, `SELECT application_id FROM application_favorites WHERE user_id = $1`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing favorites: %w", err)
	}
	defer rows.Close()

	favs := make(map[uuid.UUID]bool)
	for rows.Next() {
		var appID uuid.UUID
		if err := rows.Scan(&appID); err != nil {
			return nil, fmt.Errorf("scanning favorite: %w", err)
		}
		favs[appID] = true
	}
	return favs, rows.Err()
}

func (r *ApplicationRepository) GetCategories(ownerID uuid.UUID) ([]string, error) {
	query := Q(r.dialect, `SELECT DISTINCT category FROM applications WHERE owner_id = $1 AND category != '' ORDER BY category`)
	rows, err := r.db.Query(query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, cat)
	}
	return categories, rows.Err()
}
