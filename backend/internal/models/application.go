package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a registered service/application in the dashboard.
type Application struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Category    string    `json:"category"`
	OwnerID     uuid.UUID `json:"owner_id"`
	IsFavorite  bool      `json:"is_favorite,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ApplicationFavorite represents a user's favorite application.
type ApplicationFavorite struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	CreatedAt     time.Time `json:"created_at"`
}
