package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/identity"
)

type IdentityPlatformHandler struct{ service *identity.Service }

func NewIdentityPlatformHandler(s *identity.Service) *IdentityPlatformHandler {
	return &IdentityPlatformHandler{service: s}
}

// The caller supplies existing authentication/rate-limit middleware on groups.
// Public proof confirmation routes accept proof only in a nonlogged JSON body.
func (h *IdentityPlatformHandler) RegisterRoutes(public, protected *gin.RouterGroup) {
	public.POST("/auth/recovery/request", h.RequestRecovery)
	public.POST("/auth/recovery/confirm", h.ResetPassword)
	protected.GET("/account/methods", h.Methods)
	protected.GET("/account/contacts", h.Contacts)
	protected.DELETE("/account/methods/:method", h.RemoveMethod)
	protected.POST("/account/contact-proofs", h.RequestContact)
	protected.POST("/account/contact-proofs/:proofId/confirm", h.ConfirmContact)
	protected.GET("/account/proofs/:proofId", h.ProofStatus)
	protected.POST("/account/proofs/:proofId/resend", h.ResendProof)
	protected.POST("/organizations/:orgId/invitations", h.CreateInvitation)
	protected.DELETE("/organizations/:orgId/invitations/:invitationId", h.RevokeInvitation)
	protected.POST("/invitations/:invitationId/accept", h.AcceptInvitation)
	protected.POST("/organizations/:orgId/members/:userId/offboarding-preview", h.PreviewOffboarding)
	protected.POST("/organizations/:orgId/members/:userId/offboarding", h.Offboard)
}
func (h *IdentityPlatformHandler) available(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if h == nil || !h.service.Enabled() {
		RespondError(c, http.StatusServiceUnavailable, "identity platform is unavailable")
		return false
	}
	return true
}
func identityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrUnavailable):
		RespondError(c, 503, "identity platform is unavailable")
	case errors.Is(err, identity.ErrDenied):
		RespondError(c, 403, "identity operation denied")
	case errors.Is(err, identity.ErrProof):
		RespondError(c, 400, "invalid or expired proof")
	case errors.Is(err, identity.ErrInvalid):
		RespondError(c, 400, "invalid identity request")
	case errors.Is(err, identity.ErrConflict), errors.Is(err, identity.ErrLastMethod):
		RespondError(c, 409, "identity operation conflicts with current state")
	default:
		RespondError(c, 500, "identity operation unavailable")
	}
}
func identityID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil || id == uuid.Nil {
		RespondError(c, 400, "invalid resource id")
		return id, false
	}
	return id, true
}
func (h *IdentityPlatformHandler) RequestRecovery(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var input struct {
		Contact string `json:"contact" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid recovery request")
		return
	}
	result, err := h.service.RequestRecovery(c.Request.Context(), input.Contact)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(202, gin.H{"request": result})
}
func (h *IdentityPlatformHandler) ResetPassword(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var input struct {
		ID       uuid.UUID `json:"id" binding:"required"`
		Proof    string    `json:"proof" binding:"required"`
		Password string    `json:"password" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid recovery request")
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), input.ID, input.Proof, input.Password); err != nil {
		identityError(c, err)
		return
	}
	c.Status(204)
}
func (h *IdentityPlatformHandler) RequestContact(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var input struct {
		Contact string `json:"contact" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid contact request")
		return
	}
	result, err := h.service.RequestContact(c.Request.Context(), PrincipalFromContext(c), input.Contact)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(202, gin.H{"request": result})
}
func (h *IdentityPlatformHandler) ConfirmContact(c *gin.Context) {
	if !h.available(c) {
		return
	}
	id, ok := identityID(c, "proofId")
	if !ok {
		return
	}
	var input struct {
		Proof string `json:"proof" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid proof request")
		return
	}
	if err := h.service.ConfirmContact(c.Request.Context(), PrincipalFromContext(c), id, input.Proof); err != nil {
		identityError(c, err)
		return
	}
	c.Status(204)
}
func (h *IdentityPlatformHandler) Methods(c *gin.Context) {
	if !h.available(c) {
		return
	}
	result, err := h.service.Methods(c.Request.Context(), PrincipalFromContext(c))
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(200, gin.H{"methods": result})
}
func (h *IdentityPlatformHandler) Contacts(c *gin.Context) {
	if !h.available(c) {
		return
	}
	result, err := h.service.Contacts(c.Request.Context(), PrincipalFromContext(c))
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(200, gin.H{"contacts": result})
}
func (h *IdentityPlatformHandler) RemoveMethod(c *gin.Context) {
	if !h.available(c) {
		return
	}
	if err := h.service.RemoveMethod(c.Request.Context(), PrincipalFromContext(c), c.Param("method")); err != nil {
		identityError(c, err)
		return
	}
	c.Status(204)
}
func (h *IdentityPlatformHandler) ProofStatus(c *gin.Context) {
	if !h.available(c) {
		return
	}
	id, ok := identityID(c, "proofId")
	if !ok {
		return
	}
	result, err := h.service.ProofStatus(c.Request.Context(), PrincipalFromContext(c), id)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(200, gin.H{"delivery": result})
}
func (h *IdentityPlatformHandler) ResendProof(c *gin.Context) {
	if !h.available(c) {
		return
	}
	id, ok := identityID(c, "proofId")
	if !ok {
		return
	}
	result, err := h.service.ResendProof(c.Request.Context(), PrincipalFromContext(c), id)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(202, gin.H{"request": result})
}
func (h *IdentityPlatformHandler) CreateInvitation(c *gin.Context) {
	if !h.available(c) {
		return
	}
	org, ok := identityID(c, "orgId")
	if !ok {
		return
	}
	var input struct {
		Target  *uuid.UUID `json:"target_user_id"`
		Contact string     `json:"contact" binding:"required"`
		Role    string     `json:"role" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid invitation request")
		return
	}
	result, err := h.service.CreateInvitation(c.Request.Context(), PrincipalFromContext(c), org, input.Target, input.Contact, input.Role)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(201, gin.H{"invitation": result})
}
func (h *IdentityPlatformHandler) RevokeInvitation(c *gin.Context) {
	if !h.available(c) {
		return
	}
	org, ok := identityID(c, "orgId")
	if !ok {
		return
	}
	id, ok := identityID(c, "invitationId")
	if !ok {
		return
	}
	if err := h.service.RevokeInvitation(c.Request.Context(), PrincipalFromContext(c), org, id); err != nil {
		identityError(c, err)
		return
	}
	c.Status(204)
}
func (h *IdentityPlatformHandler) AcceptInvitation(c *gin.Context) {
	if !h.available(c) {
		return
	}
	id, ok := identityID(c, "invitationId")
	if !ok {
		return
	}
	var input struct {
		Proof string `json:"proof" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid invitation request")
		return
	}
	if err := h.service.AcceptInvitation(c.Request.Context(), PrincipalFromContext(c), id, input.Proof); err != nil {
		identityError(c, err)
		return
	}
	c.Status(204)
}
func (h *IdentityPlatformHandler) PreviewOffboarding(c *gin.Context) {
	if !h.available(c) {
		return
	}
	org, ok := identityID(c, "orgId")
	if !ok {
		return
	}
	target, ok := identityID(c, "userId")
	if !ok {
		return
	}
	result, err := h.service.PreviewOffboarding(c.Request.Context(), PrincipalFromContext(c), org, target)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(200, gin.H{"offboarding": result})
}
func (h *IdentityPlatformHandler) Offboard(c *gin.Context) {
	if !h.available(c) {
		return
	}
	org, ok := identityID(c, "orgId")
	if !ok {
		return
	}
	target, ok := identityID(c, "userId")
	if !ok {
		return
	}
	var input struct {
		Epoch     int64     `json:"expected_authority_epoch" binding:"required"`
		PreviewID uuid.UUID `json:"preview_id" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		RespondError(c, 400, "invalid offboarding request")
		return
	}
	result, err := h.service.OffboardWithPreview(c.Request.Context(), PrincipalFromContext(c), org, target, input.Epoch, c.GetHeader("Idempotency-Key"), input.PreviewID)
	if err != nil {
		identityError(c, err)
		return
	}
	c.JSON(200, gin.H{"offboarding": result})
}
