package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

// VersionHandler handles secret version history endpoints.
type VersionHandler struct {
	versionRepo *repository.SecretVersionRepository
	secretRepo  *repository.SecretRepository
	projectRepo *repository.ProjectRepository
	cryptoSvc   *crypto.Service
	vault       *vault.Service
}

func (h *VersionHandler) EnableVault(v *vault.Service) { h.vault = v }

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
	if h.vault != nil {
		versions, err := h.vault.History(c.Request.Context(), PrincipalFromContext(c), projectID, secretID)
		if err != nil {
			WrapError(c, err)
			return
		}
		c.JSON(http.StatusOK, versions)
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
		WrapError(c, err)
		return
	}

	// Decrypt values
	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		WrapError(c, err)
		return
	}
	dek, err := h.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}
	defer crypto.SecureZero(dek)

	for i := range versions {
		plaintext, err := crypto.Decrypt(dek, versions[i].EncryptedValue, versions[i].ValueNonce)
		if err != nil {
			RespondError(c, http.StatusInternalServerError, "historical value unavailable")
			return
		}
		versions[i].Value = string(plaintext)
		crypto.SecureZero(plaintext)
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
	if err != nil || version < 1 {
		RespondError(c, http.StatusBadRequest, "invalid version number")
		return
	}
	if h.vault != nil {
		r, err := h.vault.Read(c.Request.Context(), PrincipalFromContext(c), projectID, secretID, int64(version))
		if err != nil {
			WrapError(c, err)
			return
		}
		c.JSON(http.StatusOK, r)
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
		WrapError(c, err)
		return
	}
	dek, err := h.cryptoSvc.DecryptDEK(project.EncryptedDEK, project.DEKNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}
	defer crypto.SecureZero(dek)

	plaintext, err := crypto.Decrypt(dek, sv.EncryptedValue, sv.ValueNonce)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "decryption error")
		return
	}

	sv.Value = string(plaintext)
	crypto.SecureZero(plaintext)
	sv.EncryptedValue = nil
	sv.ValueNonce = nil
	c.JSON(http.StatusOK, sv)
}

// RestoreVersion appends a new current revision. The required precondition
// prevents a stale browser from overwriting a newer edit.
func (h *VersionHandler) RestoreVersion(c *gin.Context) {
	if h.vault == nil {
		WrapError(c, ErrServiceUnavailable)
		return
	}
	project, err := uuid.Parse(c.Param("id"))
	if err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	id, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || version < 1 {
		WrapError(c, ErrInvalidInput)
		return
	}
	var req struct {
		ExpectedRevision *int64 `json:"expected_current_revision"`
	}
	if err = c.ShouldBindJSON(&req); err != nil || req.ExpectedRevision == nil || *req.ExpectedRevision < 1 {
		WrapError(c, ErrInvalidInput)
		return
	}
	r, err := h.vault.Restore(c.Request.Context(), PrincipalFromContext(c), project, id, version, *req.ExpectedRevision)
	if err != nil {
		WrapError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}
