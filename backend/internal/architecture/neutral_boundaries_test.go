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

const internalPrefix = "github.com/santapong/KeepSave/backend/internal/"

// These are the new platform's direct module seams, not new legacy exceptions.
var neutralImports = map[string]string{
	"authority":          "policy repository",
	"auditview":          "authority jobs models policy",
	"automation":         "mcpgateway/catalog",
	"broker":             "crypto",
	"diagnostics":        "",
	"harness":            "automation mcpgateway/catalog",
	"identity":           "auth jobs models policy",
	"mcpauth":            "models policy repository",
	"mcpgateway":         "mcpauth mcpgateway/catalog policy runs",
	"mcpgateway/catalog": "",
	"runner":             "",
	"runs":               "automation broker jobs mcpgateway/catalog models policy runner",
}

var neutralHandlers = map[string]bool{
	"api/handlers_identity_platform.go": true,
	"api/handlers_mcp_platform.go":      true,
	"api/handlers_team_vault.go":        true,
	"api/handlers_tool_platform.go":     true,
	"api/runner_router.go":              true,
}

func neutralViolations(file *ast.File, rel string) []string {
	var violations []string
	module := neutralModule(rel)
	allowed, neutral := neutralImports[module]
	handler := neutralHandlers[rel]
	aliases := map[string]string{}
	ginContexts := map[string]bool{}
	for _, spec := range file.Imports {
		name, _ := strconv.Unquote(spec.Path.Value)
		alias := filepath.Base(name)
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		aliases[alias] = name
		if neutral && strings.HasPrefix(name, internalPrefix) {
			dependency := strings.TrimPrefix(name, internalPrefix)
			found := false
			for _, item := range strings.Fields(allowed) {
				found = found || item == dependency
			}
			if !found {
				violations = append(violations, "unapproved direct module import: "+name)
			}
		}
		if neutral && strings.HasPrefix(name, "github.com/modelcontextprotocol/go-sdk") && module != "mcpgateway" {
			violations = append(violations, "protocol SDK outside transport")
		}
		if (module == "harness" || module == "automation" || module == "mcpgateway/catalog" || module == "mcpgateway") && (name == "database/sql" || name == internalPrefix+"crypto") {
			violations = append(violations, "storage or custody in transport/portable source")
		}
		if handler && (name == "database/sql" || name == internalPrefix+"repository" || name == internalPrefix+"crypto" || name == "os/exec" || name == "syscall") {
			violations = append(violations, "handler bypasses typed service port")
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok {
			return true
		}
		pointer, ok := field.Type.(*ast.StarExpr)
		if !ok {
			return true
		}
		selector, ok := pointer.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Context" {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || aliases[qualifier.Name] != "github.com/gin-gonic/gin" {
			return true
		}
		for _, name := range field.Names {
			ginContexts[name.Name] = true
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := selector.Sel.Name
		if method == "Query" {
			if receiver, ok := selector.X.(*ast.Ident); ok && ginContexts[receiver.Name] {
				return true
			}
			if requestURL, ok := selector.X.(*ast.SelectorExpr); ok && requestURL.Sel.Name == "URL" {
				if request, ok := requestURL.X.(*ast.SelectorExpr); ok && request.Sel.Name == "Request" {
					if receiver, ok := request.X.(*ast.Ident); ok && ginContexts[receiver.Name] {
						return true
					}
				}
			}
		}
		if handler {
			switch method {
			case "Query", "QueryContext", "QueryRow", "QueryRowContext", "Exec", "ExecContext", "Begin", "BeginTx", "Decrypt", "DecryptDEK", "DecryptServiceSecret", "WithOpened":
				violations = append(violations, "handler SQL/custody call: "+method)
			}
		}
		if qualifier, ok := selector.X.(*ast.Ident); ok && module != "runner" {
			pkg := aliases[qualifier.Name]
			if pkg == "os" && method == "StartProcess" || pkg == "syscall" && (method == "Exec" || method == "ForkExec" || method == "StartProcess") {
				violations = append(violations, "process start outside restricted runner")
			}
		}
		if neutral && method == "WithOpened" && module != "broker" && rel != "identity/delivery.go" && rel != "runs/operations.go" {
			violations = append(violations, "plaintext opening outside reviewed custody port")
		}
		return true
	})
	return violations
}

func neutralModule(rel string) string {
	directory := filepath.ToSlash(filepath.Dir(rel))
	selected := ""
	for module := range neutralImports {
		if (directory == module || strings.HasPrefix(directory, module+"/")) && len(module) > len(selected) {
			selected = module
		}
	}
	return selected
}

func TestNeutralCoreBoundaries(t *testing.T) {
	if err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel("..", path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if neutralModule(rel) == "" && !neutralHandlers[rel] {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, violation := range neutralViolations(file, rel) {
			t.Errorf("%s: %s", rel, violation)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNeutralBoundaryChecksCatchAliasedBypasses(t *testing.T) {
	for _, tc := range []struct{ path, source string }{
		{"mcpgateway/unsafe.go", `package mcpgateway; import legacy "github.com/santapong/KeepSave/backend/internal/service"; var _=legacy.NewProjectService`},
		{"harness/unsafe.go", `package harness; import "github.com/modelcontextprotocol/go-sdk/mcp"; var _=mcp.NewServer`},
		{"api/handlers_mcp_platform.go", `package api; import store "database/sql"; func unsafe(db *store.DB){ db.QueryRow("SELECT payload") }`},
		{"mcpgateway/unsafe.go", `package mcpgateway; import child "os"; func unsafe(){ child.StartProcess("runner",nil,nil) }`},
		{"runs/unsafe.go", `package runs; func unsafe(c interface{WithOpened()}) { c.WithOpened() }`},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), tc.path, tc.source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(neutralViolations(file, tc.path)) == 0 {
			t.Fatal("boundary bypass was not caught", tc.path)
		}
	}
}

func TestNeutralBoundaryCheckAcceptsOnlyGinRequestQueryParsing(t *testing.T) {
	source := `package api; import transport "github.com/gin-gonic/gin"; func parse(c *transport.Context){ c.Query("request_id"); c.Request.URL.Query() }`
	file, err := parser.ParseFile(token.NewFileSet(), "api/handlers_mcp_platform.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if violations := neutralViolations(file, "api/handlers_mcp_platform.go"); len(violations) != 0 {
		t.Fatal("request parsing was classified as SQL", violations)
	}
	unsafe := `package api; func query(db interface{Query(string)}) { db.Query("SELECT payload") }`
	file, err = parser.ParseFile(token.NewFileSet(), "api/handlers_mcp_platform.go", unsafe, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(neutralViolations(file, "api/handlers_mcp_platform.go")) == 0 {
		t.Fatal("request parsing exception widened to an arbitrary Query method")
	}
}
