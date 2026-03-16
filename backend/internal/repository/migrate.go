package repository

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunMigrations(db *sql.DB, dialect Dialect, migrationsDir string) error {
	// Determine the subdirectory based on dialect
	subdir := string(dialect.DBType())
	fullDir := filepath.Join(migrationsDir, subdir)

	// Fall back to base directory if subdirectory doesn't exist (backward compat)
	if _, err := os.Stat(fullDir); os.IsNotExist(err) {
		fullDir = migrationsDir
	}

	// Create schema_migrations table using dialect-appropriate SQL
	createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at %s NOT NULL DEFAULT %s
	)`, schemaTimestamp(dialect), dialect.Now())
	_, err := db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(fullDir)
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

		content, err := os.ReadFile(filepath.Join(fullDir, file))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", file, err)
		}

		// For SQLite, execute statements one at a time since SQLite
		// doesn't support multiple statements in a single Exec
		if dialect.DBType() == DBTypeSQLite {
			stmts := splitSQLStatements(string(content))
			for _, stmt := range stmts {
				stmt = strings.TrimSpace(stmt)
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
