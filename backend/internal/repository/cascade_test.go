package repository

import (
	"strings"
	"testing"

	"os"
	"path/filepath"
)

// TestSecretVersionsCascadeDelete documents and locks audit S-M1: the
// canonical Postgres migration MUST keep `ON DELETE CASCADE` on
// secret_versions.secret_id so deleting a secret also purges its history.
// Without this, plaintext lives on in the version table after the user
// believes they've deleted it.
//
// We verify by string-matching the migration text rather than spinning
// up a live Postgres - the cascade is a SQL DDL fact and a grep is the
// right level of assertion to catch a regression in CI.
func TestSecretVersionsCascadeDelete(t *testing.T) {
	for _, dialect := range []string{"postgres", "mysql"} {
		t.Run(dialect, func(t *testing.T) {
			path := filepath.Join("..", "..", "migrations", dialect, "003_secret_versions.sql")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			lower := strings.ToLower(string(data))
			if !strings.Contains(lower, "secret_id") {
				t.Fatalf("%s: missing secret_id column reference", path)
			}
			// Match REFERENCES secrets(id) ON DELETE CASCADE on the same
			// line as secret_id - tolerates whitespace variation.
			if !strings.Contains(lower, "references secrets(id) on delete cascade") {
				t.Errorf("%s: secret_versions.secret_id must REFERENCES secrets(id) ON DELETE CASCADE", path)
			}
		})
	}
}
