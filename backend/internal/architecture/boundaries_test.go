// Package architecture enforces the incremental module boundaries. Legacy
// credential adapters are enumerated so their authority cannot quietly spread.
package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestModuleBoundaries(t *testing.T) {
	legacyDecrypt := map[string]bool{
		"api/handlers_mcp_gateway.go": true, "api/handlers_version.go": true,
		"auth/keystore.go": true, "service/template_service.go": true,
		"service/secret_service.go": true, "service/promotion_service.go": true,
		"service/drift_service.go": true, "service/dependency_service.go": true,
		"service/keyrotation_service.go": true, "service/envfile_service.go": true,
		"service/sso_service.go": true,
	}
	legacyExecution := map[string]bool{"api/handlers_mcp_gateway.go": true, "service/mcp_builder_service.go": true}
	root := ".."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, i := range file.Imports {
			name, _ := strconv.Unquote(i.Path.Value)
			if strings.HasPrefix(rel, "policy/") && (strings.Contains(name, "/internal/repository") || strings.Contains(name, "/internal/api") || strings.Contains(name, "/internal/service") || name == "database/sql") {
				t.Errorf("policy depends on infrastructure: %s %s", rel, name)
			}
			if (strings.HasPrefix(rel, "vault/") || strings.HasPrefix(rel, "jobs/")) && (strings.Contains(name, "/internal/api") || strings.Contains(name, "/internal/repository") || strings.Contains(name, "/internal/service")) {
				t.Errorf("module bypasses an explicit port: %s %s", rel, name)
			}
			if name == "os/exec" && !legacyExecution[rel] && !strings.HasPrefix(rel, "runner/") {
				t.Errorf("process execution outside runner/legacy adapter: %s", rel)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "DecryptDEK", "DecryptServiceSecret", "Decrypt":
				if !strings.HasPrefix(rel, "crypto/") && !strings.HasPrefix(rel, "vault/") && !strings.HasPrefix(rel, "broker/") && !legacyDecrypt[rel] {
					t.Errorf("credential decryption outside custody boundary: %s", rel)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
