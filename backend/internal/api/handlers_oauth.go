package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type OAuthHandler struct {
	oauthService *service.OAuthService
}

func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

func (h *OAuthHandler) RegisterClient(c *gin.Context) {
	var req RegisterOAuthClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.MustGet("user_id").(uuid.UUID)
	client, rawSecret, err := h.oauthService.RegisterClient(req.Name, req.Description, userID, req.RedirectURIs, req.Scopes, req.GrantTypes, req.LogoURL, req.HomepageURL, req.IsPublic)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"client": client, "client_secret": rawSecret})
}

func (h *OAuthHandler) ListClients(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	clients, err := h.oauthService.ListClients(userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if clients == nil {
		clients = []models.OAuthClient{}
	}
	c.JSON(http.StatusOK, gin.H{"clients": clients})
}

func (h *OAuthHandler) DeleteClient(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	clientID, err := uuid.Parse(c.Param("clientId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid client id")
		return
	}
	if err := h.oauthService.DeleteClient(clientID, userID); err != nil {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *OAuthHandler) Authorize(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	responseType := c.Query("response_type")
	scope := c.Query("scope")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	if responseType != "code" {
		RespondError(c, http.StatusBadRequest, "unsupported response_type, must be 'code'")
		return
	}
	if clientID == "" || redirectURI == "" {
		RespondError(c, http.StatusBadRequest, "client_id and redirect_uri required")
		return
	}
	userID := c.MustGet("user_id").(uuid.UUID)
	var scopes []string
	if scope != "" {
		scopes = strings.Split(scope, " ")
	}
	code, err := h.oauthService.Authorize(clientID, userID, redirectURI, scopes, codeChallenge, codeChallengeMethod)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code, "redirect_uri": redirectURI})
}

func (h *OAuthHandler) Token(c *gin.Context) {
	grantType := c.PostForm("grant_type")
	if grantType == "" {
		var req TokenRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			grantType = req.GrantType
			h.handleTokenJSON(c, req)
			return
		}
	}
	switch grantType {
	case "authorization_code":
		h.handleAuthorizationCodeGrant(c)
	case "client_credentials":
		h.handleClientCredentialsGrant(c)
	case "refresh_token":
		h.handleRefreshTokenGrant(c)
	default:
		RespondError(c, http.StatusBadRequest, "unsupported grant_type")
	}
}

func (h *OAuthHandler) handleTokenJSON(c *gin.Context, req TokenRequest) {
	switch req.GrantType {
	case "authorization_code":
		resp, err := h.oauthService.ExchangeCode(req.Code, req.ClientID, req.ClientSecret, req.RedirectURI, req.CodeVerifier)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	case "client_credentials":
		var scopes []string
		if req.Scope != "" {
			scopes = strings.Split(req.Scope, " ")
		}
		resp, err := h.oauthService.ClientCredentialsGrant(req.ClientID, req.ClientSecret, scopes)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	case "refresh_token":
		resp, err := h.oauthService.RefreshToken(req.RefreshToken, req.ClientID, req.ClientSecret)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	default:
		RespondError(c, http.StatusBadRequest, "unsupported grant_type")
	}
}

func (h *OAuthHandler) handleAuthorizationCodeGrant(c *gin.Context) {
	resp, err := h.oauthService.ExchangeCode(c.PostForm("code"), c.PostForm("client_id"), c.PostForm("client_secret"), c.PostForm("redirect_uri"), c.PostForm("code_verifier"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *OAuthHandler) handleClientCredentialsGrant(c *gin.Context) {
	var scopes []string
	if scope := c.PostForm("scope"); scope != "" {
		scopes = strings.Split(scope, " ")
	}
	resp, err := h.oauthService.ClientCredentialsGrant(c.PostForm("client_id"), c.PostForm("client_secret"), scopes)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *OAuthHandler) handleRefreshTokenGrant(c *gin.Context) {
	resp, err := h.oauthService.RefreshToken(c.PostForm("refresh_token"), c.PostForm("client_id"), c.PostForm("client_secret"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *OAuthHandler) UserInfo(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		RespondError(c, http.StatusUnauthorized, "authorization required")
		return
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		RespondError(c, http.StatusUnauthorized, "invalid authorization format")
		return
	}
	info, err := h.oauthService.GetUserInfo(parts[1])
	if err != nil {
		RespondError(c, http.StatusUnauthorized, err.Error())
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *OAuthHandler) Revoke(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		var req struct{ Token string `json:"token"` }
		if err := c.ShouldBindJSON(&req); err == nil {
			token = req.Token
		}
	}
	if token == "" {
		RespondError(c, http.StatusBadRequest, "token required")
		return
	}
	if err := h.oauthService.RevokeToken(token); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *OAuthHandler) JWKS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"keys": []interface{}{}, "note": "KeepSave currently uses HS256 for JWT signing. JWKS with RS256 support is planned."})
}

func (h *OAuthHandler) OpenIDConfiguration(c *gin.Context) {
	baseURL := c.Request.Host
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	issuer := scheme + "://" + baseURL
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/api/v1/oauth/authorize",
		"token_endpoint":                        issuer + "/api/v1/oauth/token",
		"userinfo_endpoint":                     issuer + "/api/v1/oauth/userinfo",
		"revocation_endpoint":                   issuer + "/api/v1/oauth/revoke",
		"jwks_uri":                              issuer + "/api/v1/oauth/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "client_credentials", "refresh_token"},
		"scopes_supported":                      []string{"read", "write", "delete", "promote", "admin", "mcp"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"code_challenge_methods_supported":      []string{"S256", "plain"},
	})
}
