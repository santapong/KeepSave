package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// VersionHandler handles secret version history endpoints.
type VersionHandler struct {
	versionRepo *repository.SecretVersionRepository
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	cryptoSvc   *crypto.Service
}

// NewVersionHandler creates a new version handler.
func NewVersionHandler(
	versionRepo *repository.SecretVersionRepository,
	secretRepo *repository.SecretRepository,
	projectRepo *repository.ProjectRepository,
	cryptoSvc *crypto.Service,
) *VersionHandler {
	return &VersionHandler{
		versionRepo: versionRepo,
		secretRepo:  secretRepo,
		projectRepo: projectRepo,
		cryptoSvc:   cryptoSvc,
	}
}

// ListVersions returns all versions of a secret.
func (h *VersionHandler) ListVersions(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret ID")
		return
	}

	// Verify secret belongs to project
	secret, err := h.secretRepo.GetByID(secretID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}
	if secret.ProjectID != projectID {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	versions, err := h.versionRepo.ListVersions(secretID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Decrypt values
	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	dek, err := h.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}

	for i := range versions {
		plaintext, err := crypto.Decrypt(dek, versions[i].EncryptedValue, versions[i].ValueNonce)
		if err != nil {
			continue
		}
		versions[i].Value = string(plaintext)
		versions[i].EncryptedValue = nil
		versions[i].ValueNonce = nil
	}

	c.JSON(http.StatusOK, versions)
}

// GetVersion returns a specific version of a secret.
func (h *VersionHandler) GetVersion(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project ID")
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid secret ID")
		return
	}

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid version number")
		return
	}

	// Verify secret belongs to project
	secret, err := h.secretRepo.GetByID(secretID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}
	if secret.ProjectID != projectID {
		RespondError(c, http.StatusNotFound, "secret not found")
		return
	}

	sv, err := h.versionRepo.GetVersion(secretID, version)
	if err != nil {
		RespondError(c, http.StatusNotFound, "version not found")
		return
	}

	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	dek, err := h.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}

	plaintext, err := crypto.Decrypt(dek, sv.EncryptedValue, sv.ValueNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}

	sv.Value = string(plaintext)
	sv.EncryptedValue = nil
	sv.ValueNonce = nil
	c.JSON(http.StatusOK, sv)
}
