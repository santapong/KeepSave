package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type SecretHandler struct {
	secretService *service.SecretService
}

func NewSecretHandler(secretService *service.SecretService) *SecretHandler {
	return &SecretHandler{secretService: secretService}
}

func (h *SecretHandler) Create(c *gin.Context) {
	var req CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	actorID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Per-key scope (ADR-0022): a key-scoped API key may only create keys its
	// glob permits. No-op for JWT callers and legacy bare scopes.
	if !APIKeyScopeAllowsKey(c, "write", req.Key) {
		WrapError(c, ErrForbidden)
		return
	}
	secret, err := h.secretService.Create(projectID, req.Environment, req.Key, req.Value, actorID, c.GetString("client_ip"))
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"secret": secret})
}

func (h *SecretHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	envName := c.Query("environment")
	if envName == "" {
		RespondError(c, http.StatusBadRequest, "environment query parameter is required")
		return
	}

	// Opt-in secret-reference resolution (ADR-0020): ?resolve=true interpolates
	// ${VAR}-style references against the same environment's keys.
	var secrets []models.Secret
	if c.Query("resolve") == "true" {
		secrets, err = h.secretService.ListResolved(projectID, envName)
	} else {
		secrets, err = h.secretService.List(projectID, envName)
	}
	if err != nil {
		WrapError(c, err)
		return
	}

	if secrets == nil {
		secrets = []models.Secret{}
	}

	// Per-key scope (ADR-0022): a key-scoped API key sees only the keys its
	// globs permit. No-op for JWT callers and legacy bare scopes (which match
	// every key), so the common case keeps the full list.
	if _, scoped := c.Get("api_key_scopes"); scoped {
		filtered := secrets[:0]
		for _, s := range secrets {
			if APIKeyScopeAllowsKey(c, "read", s.Key) {
				filtered = append(filtered, s)
			}
		}
		secrets = filtered
		if secrets == nil {
			secrets = []models.Secret{}
		}
	}

	c.JSON(http.StatusOK, gin.H{"secrets": secrets})
}

func (h *SecretHandler) Get(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	secret, err := h.secretService.GetByID(projectID, secretID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}
	// Per-key scope (ADR-0022): an out-of-scope key is reported as not-found to
	// avoid leaking which keys exist (anti-enumeration).
	if !APIKeyScopeAllowsKey(c, "read", secret.Key) {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *SecretHandler) Update(c *gin.Context) {
	var req UpdateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	actorID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Per-key scope (ADR-0022): resolve the key first so an out-of-scope write
	// is rejected (as not-found) before any mutation.
	if _, scoped := c.Get("api_key_scopes"); scoped {
		existing, gerr := h.secretService.GetByID(projectID, secretID)
		if gerr != nil || !APIKeyScopeAllowsKey(c, "write", existing.Key) {
			RespondError(c, http.StatusNotFound, "secret not found")
			return
		}
	}
	secret, err := h.secretService.Update(projectID, secretID, req.Value, actorID, c.GetString("client_ip"))
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *SecretHandler) Delete(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret id")
		return
	}

	actorID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Per-key scope (ADR-0022): resolve the key first so an out-of-scope delete
	// is rejected (as not-found) before any mutation.
	if _, scoped := c.Get("api_key_scopes"); scoped {
		existing, gerr := h.secretService.GetByID(projectID, secretID)
		if gerr != nil || !APIKeyScopeAllowsKey(c, "delete", existing.Key) {
			RespondError(c, http.StatusNotFound, "secret not found")
			return
		}
	}
	if err := h.secretService.Delete(projectID, secretID, actorID, c.GetString("client_ip")); err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	c.Status(http.StatusNoContent)
}
