package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SecretAccessMetadata reads only the authorization attributes of a secret
// inside a project. No encrypted values or key material cross this boundary.
func (r *ProjectRepository) SecretAccessMetadata(ctx context.Context, projectID, secretID uuid.UUID) (key, environment string, err error) {
	err = r.db.QueryRowContext(ctx, secretQuery(r.dialect, `
		SELECT s."key", e.name FROM secrets s
		JOIN environments e ON e.id = s.environment_id AND e.project_id = s.project_id
		WHERE s.project_id = $1 AND s.id = $2`), projectID, secretID).Scan(&key, &environment)
	if err != nil {
		return "", "", fmt.Errorf("getting secret access metadata: %w", err)
	}
	return key, environment, nil
}
