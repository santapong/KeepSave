// Package mcpgateway adapts the official MCP transport to typed domain ports.
// It owns no provider credentials, SQL, runner execution or client authority.
package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"net/http"
	"strings"
	"time"
)

type TokenValidator interface {
	Validate(context.Context, string) (mcpauth.Principal, error)
}
type Operations interface {
	AvailableRuns(context.Context, policy.Principal, string) ([]runs.Run, error)
	Request(context.Context, policy.Principal, string, uuid.UUID, string, string, runs.Arguments) (runs.Operation, error)
	Status(context.Context, policy.Principal, string, uuid.UUID, time.Duration) (runs.Operation, error)
	Result(context.Context, policy.Principal, string, uuid.UUID) (runs.Operation, error)
	CancelOperation(context.Context, policy.Principal, string, uuid.UUID) error
	CancelRun(context.Context, policy.Principal, string, uuid.UUID) error
}
type Config struct {
	OAuth   mcpauth.Config
	Version string
}
type principalKey struct{}

func New(tokens TokenValidator, operations Operations, cfg Config) (http.Handler, error) {
	if tokens == nil || operations == nil {
		return nil, mcpauth.ErrUnavailable
	}
	if e := cfg.OAuth.Validate(); e != nil {
		return nil, e
	}
	if cfg.Version == "" {
		cfg.Version = "1"
	}
	transport := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		p, ok := r.Context().Value(principalKey{}).(mcpauth.Principal)
		if !ok {
			return nil
		}
		return server(operations, p, cfg.Version)
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 64 << 10, PropagateRequestCancellation: true})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The transport and protocol are explicit, independently of harness names.
		if len(r.Header.Values("MCP-Protocol-Version")) > 1 {
			http.Error(w, "ambiguous protocol version", http.StatusBadRequest)
			return
		}
		if v := r.Header.Get("MCP-Protocol-Version"); v != "" && v != "2025-11-25" && v != "2026-07-28" {
			http.Error(w, "unsupported protocol version", http.StatusBadRequest)
			return
		}
		headers := r.Header.Values("Authorization")
		if len(headers) != 1 || !strings.HasPrefix(headers[0], "Bearer ") {
			challenge(w, cfg.OAuth)
			return
		}
		raw := strings.TrimPrefix(headers[0], "Bearer ")
		if strings.ContainsAny(raw, " \t\r\n") {
			challenge(w, cfg.OAuth)
			return
		}
		p, e := tokens.Validate(r.Context(), raw)
		if e != nil {
			if errors.Is(e, mcpauth.ErrUnavailable) {
				http.Error(w, "authority unavailable", http.StatusServiceUnavailable)
			} else {
				challenge(w, cfg.OAuth)
			}
			return
		}
		if p.UserID == uuid.Nil || p.SessionID == uuid.Nil || p.FamilyID == uuid.Nil || p.TokenID == uuid.Nil || p.ClientID == "" || !p.ExpiresAt.After(time.Now()) {
			challenge(w, cfg.OAuth)
			return
		}
		transport.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
	return mcpauth.GuardHTTP(cfg.OAuth, handler), nil
}
func challenge(w http.ResponseWriter, cfg mcpauth.Config) {
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+cfg.AppURL+`/.well-known/oauth-protected-resource/mcp", scope="`+mcpauth.Scope+`"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}
func actor(p mcpauth.Principal) policy.Principal {
	return policy.Principal{Kind: policy.OAuthDelegation, SubjectID: p.UserID, ActorID: p.UserID, SessionID: p.SessionID, ParentGrantID: p.FamilyID, TokenID: p.TokenID.String(), ExpiresAt: p.ExpiresAt}
}

var errDenied = errors.New("resource unavailable")

func safe(e error) error {
	if e != nil {
		return errDenied
	}
	return nil
}

type runInput struct {
	RunID      string `json:"run_id"`
	RequestKey string `json:"request_key"`
}
type fileInput struct {
	RunID      string `json:"run_id"`
	RequestKey string `json:"request_key"`
	Path       string `json:"path"`
}
type statusInput struct {
	OperationID string `json:"operation_id"`
	WaitMS      int    `json:"wait_ms,omitempty"`
}
type operationInput struct {
	OperationID string `json:"operation_id"`
}
type cancelRunInput struct {
	RunID string `json:"run_id"`
}

func tool(name, description string) *mcp.Tool {
	d := catalog.Find(name)
	return &mcp.Tool{Name: d.Name, Description: description, InputSchema: d.InputSchema, Meta: mcp.Meta{"keepsave/installation": d.Installation, "keepsave/schema_digest": d.SchemaDigest, "keepsave/operation": d.Operation}}
}

// Bound the actual JSON representation, including both legacy text fallback
// and structured content. Escaping overhead can exceed a simple payload/2 cap.
func output(value any, e error) (*mcp.CallToolResult, any, error) {
	if e != nil {
		return nil, nil, safe(e)
	}
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, nil, errDenied
	}
	result := &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}
	wire, e := json.Marshal(result)
	if e != nil || len(wire) > catalog.MaxEnvelopeBytes-(64<<10) {
		return nil, nil, errors.New("result exceeds response limit")
	}
	return result, nil, nil
}
func server(ops Operations, p mcpauth.Principal, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "keepsave", Version: version}, &mcp.ServerOptions{SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25"}, Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}}, PageSize: 20})
	a := actor(p)
	mcp.AddTool(s, tool("available_runs", "List live runs explicitly approved for this account and client."), func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		r, e := ops.AvailableRuns(ctx, a, p.ClientID)
		if r == nil {
			r = []runs.Run{}
		}
		return output(map[string]any{"runs": r}, e)
	})
	mcp.AddTool(s, tool("repository_tree", "Queue a bounded read of the approved repository tree at the run's pinned commit. Reuse request_key to reconcile retries."), func(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.RunID)
		if e != nil {
			return nil, nil, errDenied
		}
		r, e := ops.Request(ctx, a, p.ClientID, id, in.RequestKey, "repository_tree", runs.Arguments{})
		r.Result = nil // Queuing and reconciliation only return metadata.
		return output(r, e)
	})
	mcp.AddTool(s, tool("read_file", "Queue a bounded read of an approved file at the pinned commit. No arbitrary URLs, shell commands or credentials."), func(ctx context.Context, _ *mcp.CallToolRequest, in fileInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.RunID)
		if e != nil {
			return nil, nil, errDenied
		}
		r, e := ops.Request(ctx, a, p.ClientID, id, in.RequestKey, "read_file", runs.Arguments{Path: in.Path})
		r.Result = nil
		return output(r, e)
	})
	mcp.AddTool(s, tool("operation_status", "Read operation metadata and receipt, without plaintext. Optional wait_ms is bounded to 2000."), func(ctx context.Context, _ *mcp.CallToolRequest, in statusInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.OperationID)
		if e != nil || in.WaitMS < 0 || in.WaitMS > 2000 {
			return nil, nil, errDenied
		}
		r, e := ops.Status(ctx, a, p.ClientID, id, time.Duration(in.WaitMS)*time.Millisecond)
		r.Result = nil // Plaintext delivery is an explicit, reauthorized operation.
		return output(r, e)
	})
	mcp.AddTool(s, tool("operation_result", "Deliver an operation result only after current stored authority is rechecked."), func(ctx context.Context, _ *mcp.CallToolRequest, in operationInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.OperationID)
		if e != nil {
			return nil, nil, errDenied
		}
		r, e := ops.Result(ctx, a, p.ClientID, id)
		return output(r, e)
	})
	mcp.AddTool(s, tool("cancel_operation", "Durably cancel an owned operation. Cancellation cannot recall returned data."), func(ctx context.Context, _ *mcp.CallToolRequest, in operationInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.OperationID)
		if e != nil {
			return nil, nil, errDenied
		}
		e = ops.CancelOperation(ctx, a, p.ClientID, id)
		return output(map[string]any{"cancelled": e == nil}, e)
	})
	mcp.AddTool(s, tool("cancel_run", "Durably cancel this client-owned run and prevent later dispatch. Use this explicit control to cancel an accepted run; stopping a client RPC is a separate action."), func(ctx context.Context, _ *mcp.CallToolRequest, in cancelRunInput) (*mcp.CallToolResult, any, error) {
		id, e := uuid.Parse(in.RunID)
		if e != nil {
			return nil, nil, errDenied
		}
		e = ops.CancelRun(ctx, a, p.ClientID, id)
		return output(map[string]any{"cancelled": e == nil}, e)
	})
	return s
}
