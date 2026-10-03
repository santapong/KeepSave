package api

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"io"
	"mime"
	"net/http"
	"net/url"
)

type MCPPlatformHandler struct{ service *mcpauth.Service }

func NewMCPPlatformHandler(s *mcpauth.Service) *MCPPlatformHandler {
	return &MCPPlatformHandler{service: s}
}
func (h *MCPPlatformHandler) guard(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.service == nil {
		WrapError(c, ErrServiceUnavailable)
		return false
	}
	if !mcpauth.OriginAllowed(h.service.Config(), c.Request) {
		WrapError(c, ErrForbidden)
		return false
	}
	return true
}
func mcpOAuthError(c *gin.Context, e error) {
	status := http.StatusBadRequest
	code := "invalid_grant"
	if errors.Is(e, mcpauth.ErrUnavailable) {
		status = http.StatusServiceUnavailable
		code = "temporarily_unavailable"
	} else if errors.Is(e, mcpauth.ErrInvalid) {
		code = "invalid_request"
	}
	c.JSON(status, gin.H{"error": code})
}
func (h *MCPPlatformHandler) ProtectedMetadata(c *gin.Context) {
	if h.guard(c) {
		c.JSON(http.StatusOK, h.service.ProtectedMetadata())
	}
}
func (h *MCPPlatformHandler) AuthorizationMetadata(c *gin.Context) {
	if h.guard(c) {
		c.JSON(http.StatusOK, h.service.AuthorizationMetadata())
	}
}
func (h *MCPPlatformHandler) Authorize(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	req, e := h.service.BeginAuthorization(c.Request.Context(), c.Request.URL.Query())
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.Redirect(http.StatusSeeOther, h.service.Config().AppURL+"/mcp/consent?request_id="+url.QueryEscape(req.ID.String()))
}
func (h *MCPPlatformHandler) ConsentPreview(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	user, sid, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	raw := c.Query("request_id")
	if len(c.Request.URL.Query()["request_id"]) > 1 {
		WrapError(c, ErrInvalidInput)
		return
	}
	if raw == "" {
		raw = c.Param("requestId")
	}
	id, e := uuid.Parse(raw)
	if e != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	req, e := h.service.Preview(c.Request.Context(), user, sid, id)
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.JSON(http.StatusOK, req)
}
func (h *MCPPlatformHandler) Decide(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	user, sid, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	media, _, mediaError := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if mediaError != nil || media != "application/json" {
		WrapError(c, ErrInvalidInput)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	id, approve, e := mcpDecision(c.Request.Body)
	if e != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	redirect, e := h.service.Decide(c.Request.Context(), user, sid, id, approve)
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"redirect_url": redirect})
}

// Read security-significant keys once. Duplicate, unknown and trailing JSON
// fields cannot quietly change the browser's explicit decision.
func mcpDecision(body io.Reader) (uuid.UUID, bool, error) {
	decoder := json.NewDecoder(body)
	first, e := decoder.Token()
	if e != nil || first != json.Delim('{') {
		return uuid.Nil, false, mcpauth.ErrInvalid
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, e := decoder.Token()
		name, ok := key.(string)
		if e != nil || !ok || (name != "request_id" && name != "approve") {
			return uuid.Nil, false, mcpauth.ErrInvalid
		}
		if _, exists := fields[name]; exists {
			return uuid.Nil, false, mcpauth.ErrInvalid
		}
		var raw json.RawMessage
		if e = decoder.Decode(&raw); e != nil {
			return uuid.Nil, false, mcpauth.ErrInvalid
		}
		fields[name] = raw
	}
	if _, e = decoder.Token(); e != nil {
		return uuid.Nil, false, mcpauth.ErrInvalid
	}
	var extra any
	if e = decoder.Decode(&extra); e != io.EOF {
		return uuid.Nil, false, mcpauth.ErrInvalid
	}
	var id uuid.UUID
	var approve *bool
	if json.Unmarshal(fields["request_id"], &id) != nil || id == uuid.Nil || json.Unmarshal(fields["approve"], &approve) != nil || approve == nil {
		return uuid.Nil, false, mcpauth.ErrInvalid
	}
	return id, *approve, nil
}
func mcpForm(c *gin.Context) (url.Values, bool) {
	media, _, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/x-www-form-urlencoded" || c.Request.URL.RawQuery != "" || len(c.Request.Header.Values("Authorization")) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if e = c.Request.ParseForm(); e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return nil, false
	}
	v := c.Request.PostForm
	for k, values := range v {
		if len(values) != 1 || k == "client_secret" || k == "client_assertion" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return nil, false
		}
	}
	return v, true
}
func (h *MCPPlatformHandler) Token(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	v, ok := mcpForm(c)
	if !ok {
		return
	}
	result, e := h.service.Exchange(c.Request.Context(), mcpauth.TokenRequest{GrantType: v.Get("grant_type"), ClientID: v.Get("client_id"), Code: v.Get("code"), RedirectURI: v.Get("redirect_uri"), Verifier: v.Get("code_verifier"), RefreshToken: v.Get("refresh_token"), Resource: v.Get("resource"), Scope: v.Get("scope")})
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *MCPPlatformHandler) Revoke(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	v, ok := mcpForm(c)
	if !ok {
		return
	}
	if v.Get("token") == "" || v.Get("client_id") == "" {
		mcpOAuthError(c, mcpauth.ErrInvalid)
		return
	}
	if e := h.service.Revoke(c.Request.Context(), v.Get("token"), v.Get("client_id")); e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.Status(http.StatusOK)
}
func (h *MCPPlatformHandler) Delegations(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	user, sid, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	records, e := h.service.ListDelegations(c.Request.Context(), policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid})
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"delegations": records})
}
func (h *MCPPlatformHandler) RevokeDelegation(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	user, sid, ok := sessionPrincipal(c)
	if !ok {
		return
	}
	id, e := uuid.Parse(c.Param("familyId"))
	if e != nil {
		WrapError(c, ErrInvalidInput)
		return
	}
	e = h.service.RevokeDelegation(c.Request.Context(), policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user, SessionID: sid}, id)
	if e != nil {
		mcpOAuthError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
