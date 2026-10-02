package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
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

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	// Per-key scope (ADR-0022): a key-scoped API key may only create keys its
	// glob permits. No-op for JWT callers and legacy bare scopes.
	if !APIKeyScopeAllowsKey(c, "write", req.Key) {
		WrapError(c, ErrForbidden)
		return
	}
	secret, err := h.secretService.CreateAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, req.Environment, req.Key, req.Value, c.ClientIP())
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
		secrets, err = h.secretService.ListResolvedAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, envName)
	} else {
		secrets, err = h.secretService.ListAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, envName)
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

	c.Header("Cache-Control", "no-store")
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

	secret, err := h.secretService.GetByIDAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, secretID)
	if err != nil {
		if errors.Is(err, vault.ErrNotFound) || errors.Is(err, vault.ErrDenied) {
			RespondError(c, http.StatusNotFound, "secret not found")
		} else {
			WrapError(c, err)
		}
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

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	secret, err := h.secretService.UpdateAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, secretID, req.Value, c.ClientIP(), req.ExpectedRevision)
	if err != nil {
		WrapError(c, err)
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

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	var expected *int64
	if raw := c.Query("expected_revision"); raw != "" {
		n, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || n < 1 {
			WrapError(c, ErrInvalidInput)
			return
		}
		expected = &n
	}
	if err := h.secretService.DeleteAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, secretID, c.ClientIP(), expected); err != nil {
		WrapError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// BatchRead is the exact POST read contract used by CLI/SDK/widget clients.
func (h *SecretHandler) BatchRead(c *gin.Context) {
	var req struct {
		Environment string   `json:"environment" binding:"required,oneof=alpha uat prod"`
		Keys        []string `json:"keys" binding:"required,min=1,max=100,dive,required,max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	project, err := uuid.Parse(c.Param("id"))
	if err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	secrets, missing, err := h.secretService.BatchAuthorized(c.Request.Context(), PrincipalFromContext(c), project, req.Environment, req.Keys)
	if err != nil {
		WrapError(c, err)
		return
	}
	if missing == nil {
		missing = []string{}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"secrets": secrets, "missing_keys": missing})
}
