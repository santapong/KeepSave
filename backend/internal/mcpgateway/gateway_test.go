package mcpgateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type tokens struct {
	calls atomic.Int32
	p     mcpauth.Principal
	err   error
}

func (v *tokens) Validate(context.Context, string) (mcpauth.Principal, error) {
	v.calls.Add(1)
	return v.p, v.err
}

type operations struct {
	calls        atomic.Int32
	p            policy.Principal
	client       string
	statusResult bool
}

func (o *operations) AvailableRuns(_ context.Context, p policy.Principal, c string) ([]runs.Run, error) {
	o.calls.Add(1)
	o.p = p
	o.client = c
	return []runs.Run{}, nil
}
func (o *operations) Request(_ context.Context, p policy.Principal, c string, id uuid.UUID, key, kind string, a runs.Arguments) (runs.Operation, error) {
	o.calls.Add(1)
	o.p = p
	o.client = c
	if kind != "repository_tree" && kind != "read_file" {
		return runs.Operation{}, errors.New("unbounded kind")
	}
	return runs.Operation{ID: uuid.New(), RunID: id, Status: "queued", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (o *operations) Status(context.Context, policy.Principal, string, uuid.UUID, time.Duration) (runs.Operation, error) {
	if o.statusResult {
		return runs.Operation{Status: "completed", Result: json.RawMessage(`{"content":"must remain withheld"}`)}, nil
	}
	return runs.Operation{}, errors.New("synthetic hidden storage error")
}
func (o *operations) Result(context.Context, policy.Principal, string, uuid.UUID) (runs.Operation, error) {
	return runs.Operation{Status: "completed", Result: json.RawMessage(`{"content":"synthetic repository text"}`)}, nil
}
func (o *operations) CancelOperation(context.Context, policy.Principal, string, uuid.UUID) error {
	o.calls.Add(1)
	return nil
}
func (o *operations) CancelRun(context.Context, policy.Principal, string, uuid.UUID) error {
	o.calls.Add(1)
	return nil
}
func fixture(t *testing.T) (http.Handler, *tokens, *operations, Config) {
	t.Helper()
	cfg := Config{OAuth: mcpauth.Config{Issuer: "https://app.keepsave.example", AppURL: "https://app.keepsave.example", Resource: "https://app.keepsave.example/mcp"}, Version: "synthetic"}
	v := &tokens{p: mcpauth.Principal{UserID: uuid.New(), SessionID: uuid.New(), FamilyID: uuid.New(), TokenID: uuid.New(), ClientID: "keepsave-hermes-linux-v1", Scope: mcpauth.Scope, ExpiresAt: time.Now().Add(time.Minute)}}
	o := &operations{}
	h, e := New(v, o, cfg)
	if e != nil {
		t.Fatal(e)
	}
	return h, v, o, cfg
}
func request(t *testing.T, h http.Handler, version, method string, params map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	if version == "2026-07-28" {
		params["_meta"] = map[string]any{mcp.MetaKeyProtocolVersion: version, mcp.MetaKeyClientInfo: map[string]any{"name": "fixture", "version": "1"}, mcp.MetaKeyClientCapabilities: map[string]any{}}
	}
	b, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "https://app.keepsave.example/mcp", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer synthetic")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", version)
	r.Header.Set("Mcp-Method", method)
	if name, ok := params["name"].(string); ok {
		r.Header.Set("Mcp-Name", name)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestRealSDKStatelessBothProtocolLanesAndTypedTools(t *testing.T) {
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			h, v, o, _ := fixture(t)
			method := "server/discover"
			params := map[string]any{}
			if version == "2025-11-25" {
				method = "initialize"
				params = map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "fixture", "version": "1"}}
			}
			w := request(t, h, version, method, params)
			if w.Code != 200 || strings.Contains(w.Body.String(), `"error"`) {
				t.Fatalf("discovery failed: %d %s", w.Code, w.Body)
			}
			if w.Header().Get("Mcp-Session-Id") != "" {
				t.Fatal("stateless transport issued session")
			}
			w = request(t, h, version, "tools/list", nil)
			if w.Code != 200 || !strings.Contains(w.Body.String(), "cancel_run") || !strings.Contains(w.Body.String(), "additionalProperties") {
				t.Fatalf("tool schema: %d %s", w.Code, w.Body)
			}
			var listed struct {
				Result struct {
					Tools []struct {
						Name   string          `json:"name"`
						Schema json.RawMessage `json:"inputSchema"`
						Meta   map[string]any  `json:"_meta"`
					} `json:"tools"`
				} `json:"result"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &listed); e != nil {
				t.Fatal(e)
			}
			if len(listed.Result.Tools) != len(catalog.Definitions()) {
				t.Fatal("catalog tool count differs")
			}
			for _, tool := range listed.Result.Tools {
				h := sha256.Sum256(tool.Schema)
				if tool.Meta["keepsave/schema_digest"] != hex.EncodeToString(h[:]) || tool.Meta["keepsave/installation"] != catalog.Installation || !strings.HasPrefix(tool.Name, catalog.Installation+"__") {
					t.Fatalf("unbound schema identity: %#v", tool)
				}
			}
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__available_runs", "arguments": map[string]any{}})
			if w.Code != 200 || o.calls.Load() != 1 || o.p.Kind != policy.OAuthDelegation || o.p.ParentGrantID != v.p.FamilyID || o.client != v.p.ClientID {
				t.Fatalf("typed authority mapping: %d %s", w.Code, w.Body)
			}
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__read_file", "arguments": map[string]any{"run_id": uuid.NewString(), "request_key": "request1", "path": "README.md", "arbitrary_url": "https://evil.example"}})
			if o.calls.Load() != 1 || !strings.Contains(w.Body.String(), `"isError":true`) {
				t.Fatalf("schema mismatch allowed: %s", w.Body)
			}
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__operation_status", "arguments": map[string]any{"operation_id": uuid.NewString()}})
			if strings.Contains(w.Body.String(), "hidden storage error") || !strings.Contains(w.Body.String(), "resource unavailable") {
				t.Fatal("unsafe domain error projection")
			}
			o.statusResult = true
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__operation_status", "arguments": map[string]any{"operation_id": uuid.NewString()}})
			if strings.Contains(w.Body.String(), "must remain withheld") || strings.Contains(w.Body.String(), `\"result\"`) {
				t.Fatal("metadata status exposed plaintext")
			}
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__operation_result", "arguments": map[string]any{"operation_id": uuid.NewString()}})
			if !strings.Contains(w.Body.String(), "synthetic repository text") {
				t.Fatal("explicit result delivery failed")
			}
			w = request(t, h, version, "tools/call", map[string]any{"name": "keepsave_v1__cancel_run", "arguments": map[string]any{"run_id": uuid.NewString()}})
			if w.Code != 200 || o.calls.Load() != 2 {
				t.Fatal("explicit cancellation not routed")
			}
		})
	}
}

func TestSerializedEnvelopeIncludesEscapedTextFallback(t *testing.T) {
	// The logical payload is under half the envelope, but quotes and control
	// characters amplify the duplicate JSON string fallback on the wire.
	payload := strings.Repeat("\"\\\n", (catalog.MaxEnvelopeBytes-(64<<10))/12)
	if len(payload) >= catalog.MaxEnvelopeBytes/2 {
		t.Fatal("invalid escaping fixture")
	}
	_, _, e := output(map[string]string{"content": payload}, nil)
	if e == nil {
		t.Fatal("escaping bypassed complete response cap")
	}
	r, _, e := output(map[string]string{"content": "bounded"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	wire, e := json.Marshal(r)
	if e != nil || len(wire) > catalog.MaxEnvelopeBytes-(64<<10) {
		t.Fatal("bounded response did not serialize within cap")
	}
}
func TestOriginBeforeAuthAndBearerChallenge(t *testing.T) {
	h, v, _, cfg := fixture(t)
	for _, test := range []struct {
		origin, authorization string
		want                  int
	}{{"https://evil.example", "Bearer synthetic", 403}, {"", "", 401}, {cfg.OAuth.AppURL, "Bearer synthetic", 405}, {"null", "", 403}} {
		r := httptest.NewRequest("GET", cfg.OAuth.Resource, nil)
		if test.origin != "" {
			r.Header.Set("Origin", test.origin)
		}
		if test.authorization != "" {
			r.Header.Set("Authorization", test.authorization)
		}
		w := httptest.NewRecorder()
		before := v.calls.Load()
		h.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("status %d want %d", w.Code, test.want)
		}
		if test.want == 403 && v.calls.Load() != before {
			t.Fatal("origin denied after auth")
		}
		if test.want == 401 && !strings.Contains(w.Header().Get("WWW-Authenticate"), "oauth-protected-resource/mcp") {
			t.Fatal("missing resource challenge")
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable authority response")
		}
	}
	v.err = mcpauth.ErrUnavailable
	w := request(t, h, "2025-11-25", "tools/list", nil)
	if w.Code != 503 {
		t.Fatal("authority outage did not fail closed")
	}
	v.err = nil
	w = request(t, h, "2025-06-18", "tools/list", nil)
	if w.Code != 400 {
		t.Fatal("unselected protocol accepted")
	}
}
