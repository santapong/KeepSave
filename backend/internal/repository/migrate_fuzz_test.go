package repository

import (
	"bytes"
	"strings"
	"testing"
)

// FuzzSplitSQLStatements asserts the SQLite migration splitter never panics on
// arbitrary input and never emits an empty/whitespace-only statement.
func FuzzSplitSQLStatements(f *testing.F) {
	f.Add("SELECT 1; SELECT 2;")
	f.Add("CREATE TRIGGER t BEFORE UPDATE ON x BEGIN SELECT RAISE(ABORT,'no'); END;")
	f.Add("-- comment\n/* block; */ INSERT INTO t VALUES ('a;b');")
	f.Add("")
	f.Fuzz(func(t *testing.T, content string) {
		stmts := splitSQLStatements(content)
		for _, s := range stmts {
			if strings.TrimSpace(s) == "" {
				t.Errorf("splitSQLStatements emitted an empty statement for %q", content)
			}
		}
	})
}

// FuzzCanonicalizeJSON asserts the audit-chain JSON canonicalizer never panics
// and is idempotent (canon(canon(x)) == canon(x)).
func FuzzCanonicalizeJSON(f *testing.F) {
	f.Add([]byte(`{"b":1,"a":2}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))
	f.Add([]byte(`{"nested":{"z":[1,2,3],"a":null}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		once := canonicalizeJSON(raw)
		twice := canonicalizeJSON(once)
		if !bytes.Equal(once, twice) {
			t.Errorf("canonicalizeJSON not idempotent: %q -> %q -> %q", raw, once, twice)
		}
	})
}
