package api

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi/core.json
var coreContract embed.FS

// OpenAPIHandler serves the maintained contract for the core release.
// Legacy compatibility surfaces have a separate acceptance ledger.
type OpenAPIHandler struct{}

func NewOpenAPIHandler() *OpenAPIHandler { return &OpenAPIHandler{} }

func (h *OpenAPIHandler) Spec(c *gin.Context) {
	data, err := coreContract.ReadFile("openapi/core.json")
	if err != nil {
		WrapError(c, ErrInternal)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// Kept for package callers; the embedded JSON is the only contract source.
func openAPISpec() map[string]interface{} {
	data, err := coreContract.ReadFile("openapi/core.json")
	if err != nil {
		panic(err)
	}
	var spec map[string]interface{}
	if err = json.Unmarshal(data, &spec); err != nil {
		panic(err)
	}
	return spec
}
