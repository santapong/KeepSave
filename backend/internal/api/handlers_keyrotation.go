package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// KeyRotationHandler handles key rotation API endpoints.
type KeyRotationHandler struct {
	rotationService *service.KeyRotationService
}

// NewKeyRotationHandler creates a new key rotation handler.
func NewKeyRotationHandler(rotationService *service.KeyRotationService) *KeyRotationHandler {
	return &KeyRotationHandler{rotationService: rotationService}
}

// RotateProjectKey rotates the DEK for a specific project.
func (h *KeyRotationHandler) RotateProjectKey(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	result, err := h.rotationService.RotateProjectKeyAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID, c.ClientIP())
	if err != nil {
		WrapError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "key rotation completed successfully",
		"result":  result,
	})
}

// RotateAllKeys rotates keys for all projects owned by the authenticated user.
func (h *KeyRotationHandler) RotateAllKeys(c *gin.Context) {
	_, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	results, err := h.rotationService.RotateAllProjectsAuthorized(c.Request.Context(), PrincipalFromContext(c), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrorDetail{Code: 500, ErrorCode: "PARTIAL_ROTATION_FAILURE", Message: "rotation stopped; completed projects are listed"}, "results": results})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "all project keys rotated successfully",
		"results": results,
	})
}

// VerifyEncryption verifies that all secrets in a project can be decrypted.
func (h *KeyRotationHandler) VerifyEncryption(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	failedSecrets, err := h.rotationService.VerifyProjectEncryptionAuthorized(c.Request.Context(), PrincipalFromContext(c), projectID)
	if err != nil {
		WrapError(c, err)
		return
	}

	status := "healthy"
	if len(failedSecrets) > 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         status,
		"failed_secrets": len(failedSecrets),
	})
}
