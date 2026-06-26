package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRespondErrorWithErrError walks every .go file in this package and
// fails the build if any call site passes err.Error() as the message
// argument to RespondError or as the body of c.JSON / c.AbortWithError.
// This is the regression gate for docs/ERROR_HANDLING_STANDARD.md - new
// handlers must use WrapError so internal error strings never reach the
// client.
func TestNoRespondErrorWithErrError(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no .go files found in package")
	}

	fset := token.NewFileSet()
	var offenders []string

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				switch calleeName(node.Fun) {
				case "RespondError":
					// RespondError(c, code, message) - message is index 2
					if len(node.Args) >= 3 && isErrError(node.Args[2]) {
						offenders = append(offenders, formatOffender(fset, node, "RespondError(_, _, err.Error())"))
					}
				case "AbortWithError":
					// c.AbortWithError(status, err) inlines err.Error() in the body
					offenders = append(offenders, formatOffender(fset, node, "AbortWithError - use WrapError instead"))
				}
			case *ast.CompositeLit:
				// gin.H{... : err.Error()} - an error string placed in a response
				// map value reaches the client body just as surely as RespondError.
				// Use WrapError + a typed sentinel so the cause is logged, not sent.
				if isGinH(node.Type) {
					for _, el := range node.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok && isErrError(kv.Value) {
							offenders = append(offenders, formatOffender(fset, kv, `gin.H{...: err.Error()} - use WrapError instead`))
						}
					}
				}
			}
			return true
		})
	}

	if len(offenders) > 0 {
		t.Fatalf("found %d error-leak site(s); use WrapError + sentinel per docs/ERROR_HANDLING_STANDARD.md:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// calleeName extracts the rightmost identifier from a call expression's
// callee. For "RespondError" it returns "RespondError"; for
// "c.AbortWithError" it returns "AbortWithError".
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// isGinH reports whether a composite-literal type denotes gin.H - the map shape
// used for ad-hoc JSON response bodies. Matched on the selector name so an
// aliased gin import is still caught.
func isGinH(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "H"
}

// isErrError matches expressions like `err.Error()` or `e.Error()` regardless
// of the receiver name - any zero-arg method named Error counts as a leak.
func isErrError(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Error"
}

func formatOffender(fset *token.FileSet, n ast.Node, what string) string {
	pos := fset.Position(n.Pos())
	return pos.Filename + ":" + posLine(pos) + ": " + what
}

func posLine(p token.Position) string {
	// token.Position.String() prints "file:line:col"; we already have the
	// file in formatOffender, so just stitch line/col here.
	return itoa(p.Line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
