package repository

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"
)

func RunMigrations(db *sql.DB, dialect Dialect, migrationsDir string) error {
	return RunMigrationsFS(db, dialect, os.DirFS(migrationsDir))
}

// RunMigrationsFS runs migrations from any fs.FS rooted at the migrations
// directory (an os.DirFS in local/container runs, an embed.FS when the
// binary must be self-contained).
func RunMigrationsFS(db *sql.DB, dialect Dialect, fsys fs.FS) error {
	// Determine the subdirectory based on dialect
	fullDir := string(dialect.DBType())

	// Fall back to base directory if subdirectory doesn't exist (backward compat)
	if _, err := fs.Stat(fsys, fullDir); err != nil {
		fullDir = "."
	}

	// Create schema_migrations table using dialect-appropriate SQL
	defaultTime := dialect.Now()
	if dialect.DBType() == DBTypeSQLite {
		defaultTime = "(" + defaultTime + ")"
	}
	createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at %s NOT NULL DEFAULT %s
	)`, schemaTimestamp(dialect), defaultTime)
	_, err := db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(fsys, fullDir)
	if err != nil {
		return fmt.Errorf("reading migrations directory %s: %w", fullDir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	checkQuery := Q(dialect, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")
	insertQuery := Q(dialect, "INSERT INTO schema_migrations (version) VALUES ($1)")

	for _, file := range files {
		version := strings.TrimSuffix(file, ".sql")

		var exists bool
		err := db.QueryRow(checkQuery, version).Scan(&exists)
		if err != nil {
			// SQLite doesn't have a native EXISTS that returns bool in all drivers
			// Try alternative approach
			if dialect.DBType() == DBTypeSQLite {
				var count int
				altQuery := Q(dialect, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1")
				if err2 := db.QueryRow(altQuery, version).Scan(&count); err2 != nil {
					return fmt.Errorf("checking migration %s: %w", version, err2)
				}
				exists = count > 0
			} else {
				return fmt.Errorf("checking migration %s: %w", version, err)
			}
		}
		if exists {
			continue
		}

		content, err := fs.ReadFile(fsys, path.Join(fullDir, file))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", file, err)
		}

		// SQLite and the default MySQL connection deliberately do not accept
		// multiple statements in one Exec. Keep that restriction for ordinary
		// queries and execute these shipped migrations statement by statement.
		// MySQL DDL implicitly commits; this does not promise atomic DDL.
		if dialect.DBType() == DBTypeSQLite || dialect.DBType() == DBTypeMySQL {
			stmts := splitSQLStatements(string(content))
			if dialect.DBType() == DBTypeMySQL && file == "008_promotion_self_approval_check.sql" {
				// MySQL forbids that CHECK alongside approved_by's SET NULL
				// foreign key. Preserve its behavior using database triggers.
				// This adapter applies only when the original version is absent.
				stmts = []string{
					`CREATE TRIGGER promotion_no_self_approval_insert BEFORE INSERT ON promotion_requests FOR EACH ROW BEGIN IF NEW.approved_by IS NOT NULL AND NEW.approved_by=NEW.requested_by THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='promotion self approval forbidden'; END IF; END`,
					`CREATE TRIGGER promotion_no_self_approval_update BEFORE UPDATE ON promotion_requests FOR EACH ROW BEGIN IF NEW.approved_by IS NOT NULL AND NEW.approved_by=NEW.requested_by THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='promotion self approval forbidden'; END IF; END`,
				}
			}
			for _, stmt := range stmts {
				stmt = strings.TrimSpace(stmt)
				if dialect.DBType() == DBTypeMySQL {
					// Preserve applied file identities while making the legacy
					// TEXT empty default valid on the supported MySQL 8 line.
					stmt = strings.ReplaceAll(stmt, "TEXT DEFAULT ''", "TEXT DEFAULT ('')")
				}
				if stmt == "" {
					continue
				}
				if _, err := tx.Exec(stmt); err != nil {
					tx.Rollback()
					return fmt.Errorf("executing migration %s statement: %w\nSQL: %s", file, err, stmt)
				}
			}
		} else {
			if _, err := tx.Exec(string(content)); err != nil {
				tx.Rollback()
				return fmt.Errorf("executing migration %s: %w", file, err)
			}
		}

		if _, err := tx.Exec(insertQuery, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", file, err)
		}

		log.Printf("Applied migration: %s", file)
	}

	return nil
}

// schemaTimestamp returns the appropriate timestamp type for the schema_migrations table.
func schemaTimestamp(dialect Dialect) string {
	switch dialect.DBType() {
	case DBTypeMySQL:
		return "DATETIME"
	case DBTypeSQLite:
		return "TEXT"
	default:
		return "TIMESTAMPTZ"
	}
}

// splitSQLStatements splits SQL content into individual statements by semicolons,
// respecting string literals and comments.
func splitSQLStatements(content string) []string {
	var stmts []string
	var current strings.Builder
	inSingleQuote := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(content); i++ {
		c := content[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			current.WriteByte(c)
			continue
		}

		if inBlockComment {
			current.WriteByte(c)
			if c == '*' && i+1 < len(content) && content[i+1] == '/' {
				current.WriteByte('/')
				i++
				inBlockComment = false
			}
			continue
		}

		if inSingleQuote {
			current.WriteByte(c)
			if c == '\'' {
				if i+1 < len(content) && content[i+1] == '\'' {
					current.WriteByte('\'')
					i++ // escaped quote
				} else {
					inSingleQuote = false
				}
			}
			continue
		}

		if c == '\'' {
			inSingleQuote = true
			current.WriteByte(c)
			continue
		}

		if c == '-' && i+1 < len(content) && content[i+1] == '-' {
			inLineComment = true
			current.WriteByte(c)
			continue
		}

		if c == '/' && i+1 < len(content) && content[i+1] == '*' {
			inBlockComment = true
			current.WriteByte(c)
			continue
		}

		if c == ';' {
			// Preserve semicolons *inside* a CREATE TRIGGER ... BEGIN ... END
			// body; only the `;` after the closing END terminates the trigger
			// statement. Without this, a trigger would be split into broken
			// fragments and the migration would fail.
			if isUnterminatedTrigger(current.String()) {
				current.WriteByte(';')
				continue
			}
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	// Handle any remaining content without trailing semicolon
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		stmts = append(stmts, stmt)
	}

	return stmts
}

// isUnterminatedTrigger reports whether buf is a CREATE TRIGGER statement whose
// BEGIN...END body has not yet closed. SQLite trigger bodies carry their own
// statement-terminating semicolons; the naive splitter would cut the trigger in
// half, so those inner `;` must be preserved until the closing END.
func isUnterminatedTrigger(buf string) bool {
	up := strings.ToUpper(buf)
	if !strings.Contains(up, "CREATE TRIGGER") {
		return false
	}
	return !endsWithEndKeyword(up)
}

// endsWithEndKeyword reports whether the trimmed, upper-cased buffer ends with
// the word END — the token that closes a trigger body.
func endsWithEndKeyword(up string) bool {
	trimmed := strings.TrimRight(up, " \t\r\n")
	if !strings.HasSuffix(trimmed, "END") {
		return false
	}
	if len(trimmed) == 3 {
		return true
	}
	switch trimmed[len(trimmed)-4] {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
