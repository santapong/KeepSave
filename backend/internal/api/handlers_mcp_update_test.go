package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestUpdateServer_RejectsUnsafeEntryCommand pins API-F02: the MCP UpdateServer
// path must re-validate the entry command (not only RegisterServer), so an
// authenticated owner cannot swap a vetted command for an arbitrary one and
// reach the exec path. The validation fires before the service call, so a nil
// service is never reached for the rejected inputs.
func TestUpdateServer_RejectsUnsafeEntryCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewMCPHubHandler(nil, nil)
	r.PUT("/servers/:serverId", stubAuth(), h.UpdateServer)

	for _, bad := range []string{
		"node; rm -rf /",   // shell metacharacter
		"/bin/sh",          // path separator
		"bash -c evil",     // binary not in allow-list
		"node $(whoami)",   // command substitution
	} {
		body := `{"name":"srv","entry_command":"` + bad + `"}`
		req, _ := http.NewRequest(http.MethodPut, "/servers/"+uuid.New().String(), strings.NewReader(body))
		req.Header.Set("X-Test-User-ID", uuid.New().String())
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("entry_command %q -> %d, want 400 (body=%s)", bad, w.Code, w.Body.String())
		}
	}
}
