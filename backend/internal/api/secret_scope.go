package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// RequireSecretScope supplements project/action checks for API-key requests
// addressing a secret ID, including its history. Collection routes already
// enforce their explicit environment and key scope. Mount after authentication,
// RequireProjectAccess and EnforceAPIKeyScope, before any decrypting handler.
func RequireSecretScope(projectRepo *repository.ProjectRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, scoped := c.Get("api_key_scopes"); !scoped || c.Param("secretId") == "" {
			c.Next()
			return
		}
		projectID, projectErr := uuid.Parse(c.Param("id"))
		secretID, secretErr := uuid.Parse(c.Param("secretId"))
		if projectErr != nil || secretErr != nil {
			RespondError(c, http.StatusBadRequest, "invalid resource id")
			c.Abort()
			return
		}
		key, environment, err := projectRepo.SecretAccessMetadata(c.Request.Context(), projectID, secretID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				RespondError(c, http.StatusNotFound, "secret not found")
			} else {
				WrapError(c, err)
			}
			c.Abort()
			return
		}
		allowed := APIKeyScopeAllowsKey(c, scopeForMethod(c.Request.Method), key)
		if value, restricted := c.Get("api_key_environment"); restricted {
			expected, valid := value.(string)
			allowed = allowed && valid && expected != "" && strings.EqualFold(expected, environment)
		}
		if !allowed {
			// Match the missing-resource response to avoid key/environment enumeration.
			RespondError(c, http.StatusNotFound, "secret not found")
			c.Abort()
			return
		}
		c.Next()
	}
}
