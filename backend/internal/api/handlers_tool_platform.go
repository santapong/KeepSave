package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runner"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"io"
	"net/http"
	"strings"
	"time"
)

type ToolPlatformHandler struct{ service *runs.Service }

func NewToolPlatformHandler(s *runs.Service) *ToolPlatformHandler { return &ToolPlatformHandler{s} }
func (h *ToolPlatformHandler) available(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.service == nil || !h.service.Ready() {
		WrapError(c, ErrServiceUnavailable)
		return false
	}
	return true
}
func toolError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, runs.ErrInvalid):
		WrapError(c, ErrInvalidInput)
	case errors.Is(e, runs.ErrDenied):
		WrapError(c, ErrForbidden)
	case errors.Is(e, runs.ErrConflict):
		WrapError(c, ErrConflict)
	case errors.Is(e, runs.ErrNoWork):
		c.Status(204)
	default:
		WrapError(c, ErrServiceUnavailable)
	}
}
func toolBind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	raw, e := io.ReadAll(c.Request.Body)
	defer clear(raw)
	if e != nil || !toolUniqueJSON(raw) {
		WrapError(c, ErrInvalidInput)
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		WrapError(c, ErrInvalidInput)
		return false
	}
	return true
}
func toolUniqueJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return runs.ErrInvalid
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := key.(string)
				if !ok || s != strings.ToLower(s) || seen[s] {
					return runs.ErrInvalid
				}
				seen[s] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return runs.ErrInvalid
			}
		case '[':
			for d.More() {
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return runs.ErrInvalid
			}
		default:
			return runs.ErrInvalid
		}
		return nil
	}
	if walk(0) != nil {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func toolID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, e := uuid.Parse(c.Param(key))
	if e != nil || id == uuid.Nil {
		WrapError(c, ErrInvalidInput)
		return id, false
	}
	return id, true
}
func (h *ToolPlatformHandler) project(c *gin.Context) (uuid.UUID, policy.Principal, bool) {
	if !h.available(c) {
		return uuid.Nil, policy.Principal{}, false
	}
	id, ok := toolID(c, "id")
	p := PrincipalFromContext(c)
	if !ok {
		return id, p, false
	}
	if p.Kind != policy.Human {
		WrapError(c, ErrForbidden)
		return id, p, false
	}
	return id, p, true
}
func (h *ToolPlatformHandler) RegisterRoutes(api *gin.RouterGroup) {
	g := api.Group("/projects/:id/tool-platform")
	g.GET("/catalog", h.Catalog)
	g.GET("/artifacts", h.ListArtifacts)
	g.POST("/artifacts", h.CreateArtifact)
	g.DELETE("/artifacts/:resourceId", h.revoke("artifact"))
	g.GET("/profiles", h.ListProfiles)
	g.POST("/profiles", h.CreateProfile)
	g.POST("/profiles/:resourceId/approve", h.ApproveProfile)
	g.DELETE("/profiles/:resourceId", h.revoke("profile"))
	g.GET("/packages", h.ListPackages)
	g.POST("/packages", h.CreatePackage)
	g.GET("/packages/:resourceId", h.GetPackage)
	g.POST("/packages/:resourceId/approve", h.ApprovePackage)
	g.DELETE("/packages/:resourceId", h.revoke("package"))
	g.GET("/connections", h.ListConnections)
	g.POST("/connections", h.CreateConnection)
	g.POST("/connections/:resourceId/check", h.CheckConnection)
	g.DELETE("/connections/:resourceId", h.revoke("connection"))
	g.GET("/bindings", h.ListBindings)
	g.POST("/bindings", h.CreateBinding)
	g.DELETE("/bindings/:resourceId", h.revoke("binding"))
	g.GET("/workloads", h.ListWorkloads)
	g.POST("/workloads", h.EnrollWorkload)
	g.DELETE("/workloads/:resourceId", h.revoke("workload"))
	g.GET("/grants", h.ListGrants)
	g.POST("/grants", h.IssueGrant)
	g.DELETE("/grants/:resourceId", h.revoke("grant"))
	g.POST("/runs", h.CreateRun)
	g.GET("/runs", h.ListRuns)
	g.GET("/runs/:runId", h.GetRun)
	g.POST("/runs/:runId/cancel", h.CancelRun)
	g.GET("/runs/:runId/receipts", h.Receipts)
	g.POST("/runs/:runId/operations", h.Request)
	g.GET("/runs/:runId/operations/:operationId", h.Status)
	g.GET("/runs/:runId/operations/:operationId/result", h.Result)
	g.POST("/runs/:runId/operations/:operationId/cancel", h.CancelOperation)
	g.POST("/runs/:runId/operations/:operationId/retry", h.Retry)
}
func (h *ToolPlatformHandler) RegisterRunnerRoutes(api *gin.RouterGroup) {
	g := api.Group("/runner/operations")
	g.POST("/claim", h.Claim)
	g.POST("/execute", h.Execute)
	g.POST("/status", h.TicketStatus)
}
func (h *ToolPlatformHandler) Catalog(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.Catalog(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"tools": out, "catalog_digest": automation.CatalogDigest()})
}
func (h *ToolPlatformHandler) ListArtifacts(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListArtifacts(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"artifacts": out})
}
func (h *ToolPlatformHandler) CreateArtifact(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.CreateArtifact(c.Request.Context(), p, id, in.Name, in.Source)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) ListProfiles(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListProfiles(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"profiles": out})
}
func (h *ToolPlatformHandler) CreateProfile(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		ArtifactID uuid.UUID `json:"artifact_id"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.CreateProfile(c.Request.Context(), p, id, in.ArtifactID)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) ApproveProfile(c *gin.Context) { h.approve(c, false) }
func (h *ToolPlatformHandler) ApprovePackage(c *gin.Context) { h.approve(c, true) }
func (h *ToolPlatformHandler) approve(c *gin.Context, pkg bool) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	rid, ok := toolID(c, "resourceId")
	if !ok {
		return
	}
	var in struct {
		Digest string `json:"digest"`
	}
	if !toolBind(c, &in) {
		return
	}
	var e error
	if pkg {
		e = h.service.ApprovePackage(c.Request.Context(), p, id, rid, in.Digest)
	} else {
		e = h.service.ApproveProfile(c.Request.Context(), p, id, rid, in.Digest)
	}
	if e != nil {
		toolError(c, e)
		return
	}
	c.Status(204)
}
func (h *ToolPlatformHandler) ListPackages(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListPackages(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"packages": out})
}
func (h *ToolPlatformHandler) CreatePackage(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		ProfileID uuid.UUID `json:"profile_id"`
		Harness   string    `json:"harness"`
		Version   string    `json:"version"`
	}
	if !toolBind(c, &in) {
		return
	}
	pid, out, e := h.service.Package(c.Request.Context(), p, id, in.ProfileID, in.Harness, in.Version)
	if e != nil {
		toolError(c, e)
		return
	}
	b, _ := json.Marshal(out)
	c.JSON(201, gin.H{"id": pid, "digest": automation.Digest(b), "package": out})
}
func (h *ToolPlatformHandler) GetPackage(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	rid, ok := toolID(c, "resourceId")
	if !ok {
		return
	}
	out, e := h.service.GetPackage(c.Request.Context(), p, id, rid)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) ListConnections(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListConnections(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"connections": out})
}
func (h *ToolPlatformHandler) CreateConnection(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		AppID          int64  `json:"app_id"`
		InstallationID int64  `json:"installation_id"`
		PrivateKey     string `json:"private_key_pem"`
	}
	if !toolBind(c, &in) {
		return
	}
	key := []byte(in.PrivateKey)
	defer clear(key)
	out, e := h.service.CreateConnection(c.Request.Context(), p, id, in.AppID, in.InstallationID, key)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, gin.H{"id": out, "provider": "github_app", "external_read": false})
}
func (h *ToolPlatformHandler) ListBindings(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListBindings(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"bindings": out})
}
func (h *ToolPlatformHandler) CreateBinding(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		ConnectionID uuid.UUID     `json:"connection_id"`
		Target       broker.Target `json:"target"`
		Environment  string        `json:"environment"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.CreateBindingInEnvironment(c.Request.Context(), p, id, in.ConnectionID, in.Environment, in.Target)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) ListWorkloads(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListWorkloads(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"workloads": out})
}
func (h *ToolPlatformHandler) EnrollWorkload(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		Certificate string `json:"certificate_sha256"`
		Image       string `json:"image_digest"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.EnrollWorkload(c.Request.Context(), p, id, in.Certificate, in.Image)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) ListGrants(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListGrants(c.Request.Context(), p, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"grants": out})
}
func (h *ToolPlatformHandler) IssueGrant(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		ProfileID  uuid.UUID `json:"profile_id"`
		PackageID  uuid.UUID `json:"package_id"`
		BindingID  uuid.UUID `json:"binding_id"`
		WorkloadID uuid.UUID `json:"workload_id"`
		ActorID    uuid.UUID `json:"actor_id"`
		ClientID   string    `json:"client_id"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if !toolBind(c, &in) {
		return
	}
	g := runs.Grant{ProjectID: id, ProfileID: in.ProfileID, PackageID: in.PackageID, BindingID: in.BindingID, WorkloadID: in.WorkloadID, ActorID: in.ActorID, ClientID: in.ClientID, ExpiresAt: in.ExpiresAt}
	out, e := h.service.IssueGrant(c.Request.Context(), p, g)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) revoke(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, p, ok := h.project(c)
		if !ok {
			return
		}
		rid, ok := toolID(c, "resourceId")
		if !ok {
			return
		}
		var e error
		if kind == "package" {
			e = h.service.RevokePackage(c.Request.Context(), p, id, rid)
		} else {
			e = h.service.Revoke(c.Request.Context(), p, id, rid, kind)
		}
		if e != nil {
			toolError(c, e)
			return
		}
		c.Status(204)
	}
}
func (h *ToolPlatformHandler) CreateRun(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	var in struct {
		runs.CreateRunRequest
		ClientID string `json:"client_id"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.CreateProjectRun(c.Request.Context(), p, id, in.ClientID, in.CreateRunRequest)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(201, out)
}
func (h *ToolPlatformHandler) run(c *gin.Context) (policy.Principal, string, uuid.UUID, bool) {
	project, p, ok := h.project(c)
	if !ok {
		return p, "", uuid.Nil, false
	}
	id, ok := toolID(c, "runId")
	if !ok {
		return p, "", id, false
	}
	client := c.Query("client_id")
	r, e := h.service.GetRun(c.Request.Context(), p, client, id)
	if e != nil || r.ProjectID != project {
		toolError(c, runs.ErrDenied)
		return p, client, id, false
	}
	return p, client, id, true
}
func (h *ToolPlatformHandler) GetRun(c *gin.Context) {
	p, client, id, ok := h.run(c)
	if !ok {
		return
	}
	out, e := h.service.GetRun(c.Request.Context(), p, client, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) CancelRun(c *gin.Context) {
	p, client, id, ok := h.run(c)
	if !ok {
		return
	}
	if e := h.service.CancelRun(c.Request.Context(), p, client, id); e != nil {
		toolError(c, e)
		return
	}
	c.Status(204)
}
func (h *ToolPlatformHandler) Receipts(c *gin.Context) {
	p, client, id, ok := h.run(c)
	if !ok {
		return
	}
	out, e := h.service.Receipts(c.Request.Context(), p, client, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"receipts": out})
}
func (h *ToolPlatformHandler) Request(c *gin.Context) {
	p, client, id, ok := h.run(c)
	if !ok {
		return
	}
	var in struct {
		Key       string         `json:"request_key"`
		Kind      string         `json:"kind"`
		Arguments runs.Arguments `json:"arguments"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.Request(c.Request.Context(), p, client, id, in.Key, in.Kind, in.Arguments)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(202, out)
}
func (h *ToolPlatformHandler) operation(c *gin.Context) (policy.Principal, string, uuid.UUID, bool) {
	p, client, run, ok := h.run(c)
	if !ok {
		return p, client, uuid.Nil, false
	}
	id, ok := toolID(c, "operationId")
	if !ok {
		return p, client, id, false
	}
	o, e := h.service.Status(c.Request.Context(), p, client, id, 0)
	if e != nil || o.RunID != run {
		toolError(c, runs.ErrDenied)
		return p, client, id, false
	}
	return p, client, id, true
}
func (h *ToolPlatformHandler) Status(c *gin.Context) {
	p, client, id, ok := h.operation(c)
	if !ok {
		return
	}
	out, e := h.service.Status(c.Request.Context(), p, client, id, 0)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) Result(c *gin.Context) {
	p, client, id, ok := h.operation(c)
	if !ok {
		return
	}
	out, e := h.service.Result(c.Request.Context(), p, client, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) CancelOperation(c *gin.Context) {
	p, client, id, ok := h.operation(c)
	if !ok {
		return
	}
	if e := h.service.CancelOperation(c.Request.Context(), p, client, id); e != nil {
		toolError(c, e)
		return
	}
	c.Status(204)
}
func (h *ToolPlatformHandler) Retry(c *gin.Context) {
	p, client, id, ok := h.operation(c)
	if !ok {
		return
	}
	out, e := h.service.RetryOperation(c.Request.Context(), p, client, id)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(202, out)
}
func (h *ToolPlatformHandler) certificate(c *gin.Context) (string, bool) {
	if !h.available(c) {
		return "", false
	}
	tls := c.Request.TLS
	if tls == nil || len(tls.VerifiedChains) == 0 || len(tls.PeerCertificates) == 0 || len(tls.VerifiedChains[0]) == 0 || !tls.PeerCertificates[0].Equal(tls.VerifiedChains[0][0]) || !time.Now().Before(tls.PeerCertificates[0].NotAfter) {
		WrapError(c, ErrForbidden)
		return "", false
	}
	sum := sha256.Sum256(tls.PeerCertificates[0].Raw)
	return hex.EncodeToString(sum[:]), true
}
func (h *ToolPlatformHandler) Claim(c *gin.Context) {
	cert, ok := h.certificate(c)
	if !ok {
		return
	}
	var in runner.ClaimRequest
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.Claim(c.Request.Context(), cert, in.ImageDigest)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) Execute(c *gin.Context) {
	cert, ok := h.certificate(c)
	if !ok {
		return
	}
	var in runner.ExecuteRequest
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.Execute(c.Request.Context(), cert, in)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *ToolPlatformHandler) TicketStatus(c *gin.Context) {
	cert, ok := h.certificate(c)
	if !ok {
		return
	}
	var in runner.TicketStatusRequest
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.TicketStatus(c.Request.Context(), cert, in)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}

func (h *ToolPlatformHandler) ListRuns(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	out, e := h.service.ListRuns(c.Request.Context(), p, id, c.Query("client_id"))
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, gin.H{"runs": out})
}
func (h *ToolPlatformHandler) CheckConnection(c *gin.Context) {
	id, p, ok := h.project(c)
	if !ok {
		return
	}
	conn, ok := toolID(c, "resourceId")
	if !ok {
		return
	}
	var in struct {
		BindingID    uuid.UUID `json:"binding_id"`
		ExternalRead bool      `json:"allow_external_read"`
		TokenMint    bool      `json:"allow_authorization_token_mint"`
	}
	if !toolBind(c, &in) {
		return
	}
	out, e := h.service.CheckConnection(c.Request.Context(), p, id, conn, in.BindingID, in.ExternalRead, in.TokenMint)
	if e != nil {
		toolError(c, e)
		return
	}
	c.JSON(200, out)
}
