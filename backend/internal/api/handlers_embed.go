package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// EmbedHandler serves the public embed-config endpoint described in
// ADR-0006. This handler is intentionally UNAUTHENTICATED — the widget
// must learn its allow-list before it has any credentials to authenticate
// with. Abuse is mitigated by an IP rate-limiter (registered in router.go).
type EmbedHandler struct {
	projectService *service.ProjectService
}

func NewEmbedHandler(projectService *service.ProjectService) *EmbedHandler {
	return &EmbedHandler{projectService: projectService}
}

// GetEmbedConfig serves GET /api/v1/embed-config/:project_id.
//
// Response contract (per ADR-0006 / EMBED_ORIGIN_POLICY.md §1):
//   - 200: {"project_id": uuid, "allowed_origins": [string], "embed_policy_enabled": true}
//     The widget MUST verify its host origin against allowed_origins before
//     accepting any postMessage.
//   - 404: project does not exist OR embed_policy_enabled=false. The two
//     cases share a response shape to mitigate project-ID enumeration.
//   - 400: malformed project_id (not a uuid).
//   - 429: enforced by the upstream rate-limit middleware.
//
// SECURITY: this handler must never include any secret-bearing data
// (no DEK, no owner identity, no secret keys). The service layer's
// EmbedConfig struct is exhaustive by design — do not add fields here.
func (h *EmbedHandler) GetEmbedConfig(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	cfg, err := h.projectService.GetEmbedConfig(projectID)
	if err != nil {
		if errors.Is(err, service.ErrEmbedConfigNotFound) {
			// Same response shape for "missing" and "disabled" — see
			// service.ErrEmbedConfigNotFound godoc and ADR-0006 §Open-Q #1.
			RespondError(c, http.StatusNotFound, "embed config not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id":           cfg.ProjectID,
		"allowed_origins":      cfg.AllowedOrigins,
		"embed_policy_enabled": cfg.EmbedPolicyEnabled,
	})
}

// UpdateEmbedConfigRequest is the body for PUT /api/v1/projects/:id/embed-config.
// This endpoint IS authenticated (project owner) — unlike GetEmbedConfig.
type UpdateEmbedConfigRequest struct {
	AllowedOrigins     []string `json:"allowed_origins"`
	EmbedPolicyEnabled bool     `json:"embed_policy_enabled"`
}

// UpdateEmbedConfig serves PUT /api/v1/projects/:id/embed-config.
//
// Rejects the wildcard sentinel ["*"] with 422. Operators MUST use
// embed_policy_enabled=false to disable the widget; "*" as an origin is a
// well-known footgun (the very class of bug ADR-0006 closes).
func (h *EmbedHandler) UpdateEmbedConfig(c *gin.Context) {
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid project id")
		return
	}

	var req UpdateEmbedConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.projectService.UpdateEmbedConfig(projectID, userID, req.AllowedOrigins, req.EmbedPolicyEnabled); err != nil {
		if errors.Is(err, service.ErrWildcardOriginNotPermitted) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":  "wildcard_origin_not_permitted",
				"reason": "use embed_policy_enabled=false to disable the widget; '*' is rejected",
			})
			return
		}
		RespondError(c, http.StatusNotFound, "project not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed_origins":      req.AllowedOrigins,
		"embed_policy_enabled": req.EmbedPolicyEnabled,
	})
}
