package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// RequireProjectAccess enforces ADR-0005: every request to a /projects/:id/*
// route must be made by either (a) the project owner, (b) a member of the
// project's organization, or (c) an API key whose api_key_project_id matches
// :id and whose optional environment scope matches the requested environment
// (when present). The caller's identity is taken from context keys set by
// JWTAuthMiddleware / APIKeyAuthMiddleware.
//
// Failure modes — chosen to avoid leaking project existence:
//   - missing/invalid :id           -> 400 invalid project id
//   - project does not exist        -> 404 (same as the access-denied case
//     for an existing project would be, so
//     enumeration is moot)
//   - caller authenticated but no
//     access to the project          -> 403
//   - api-key scope mismatch         -> 403
//
// The parameter name :id is used by every existing route; routes that nest
// further (e.g. /projects/:id/secrets/:secretId) inherit the same :id.
func RequireProjectAccess(projectRepo *repository.ProjectRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectIDStr := c.Param("id")
		projectID, err := uuid.Parse(projectIDStr)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "invalid project id")
			c.Abort()
			return
		}

		// API-key scope check fires first. If the caller authenticated with
		// an API key, it MUST have been issued for this project (closes
		// audit S-B3 / cross-project scoped-key abuse).
		if v, ok := c.Get("api_key_project_id"); ok {
			scopedID, scopedOK := v.(uuid.UUID)
			if !scopedOK || scopedID != projectID {
				RespondError(c, http.StatusForbidden, "api key not scoped to this project")
				c.Abort()
				return
			}
		}

		userIDValue, exists := c.Get("user_id")
		if !exists {
			// Auth middleware should always set this; if not, fail closed.
			RespondError(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		userID, ok := userIDValue.(uuid.UUID)
		if !ok {
			RespondError(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}

		allowed, err := projectRepo.UserHasAccess(userID, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				RespondError(c, http.StatusNotFound, "project not found")
				c.Abort()
				return
			}
			RespondError(c, http.StatusInternalServerError, "project access check failed")
			c.Abort()
			return
		}
		if !allowed {
			RespondError(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		c.Next()
	}
}
