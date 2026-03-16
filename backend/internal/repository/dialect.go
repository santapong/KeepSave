package repository

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// DBType represents a supported database type.
type DBType string

const (
	DBTypePostgres DBType = "postgres"
	DBTypeMySQL    DBType = "mysql"
	DBTypeSQLite   DBType = "sqlite"
)

// Dialect provides database-specific SQL fragment generation.
type Dialect interface {
	DBType() DBType
	Placeholder(index int) string
	SupportsReturning() bool
	Now() string
	FormatUpsert(conflictTarget, updateSet string) string
	ArrayParam(val []string) (interface{}, error)
	ScanArray(src interface{}) ([]string, error)
	DateTrunc(field, column string) string
	IntervalAgo(duration string) string
	BoolLiteral(val bool) string
}

// DetectDBType determines the database type from a connection URL.
func DetectDBType(databaseURL string) DBType {
	lower := strings.ToLower(databaseURL)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return DBTypePostgres
	}
	if strings.HasPrefix(lower, "mysql://") || strings.Contains(lower, "@tcp(") {
		return DBTypeMySQL
	}
	if strings.HasPrefix(lower, "sqlite://") || strings.HasPrefix(lower, "file:") ||
		strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") ||
		lower == ":memory:" {
		return DBTypeSQLite
	}
	// Default to postgres for backward compatibility
	return DBTypePostgres
}

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct{}

func (d *PostgresDialect) DBType() DBType                { return DBTypePostgres }
func (d *PostgresDialect) Placeholder(index int) string   { return fmt.Sprintf("$%d", index) }
func (d *PostgresDialect) SupportsReturning() bool        { return true }
func (d *PostgresDialect) Now() string                    { return "NOW()" }
func (d *PostgresDialect) BoolLiteral(val bool) string {
	if val {
		return "TRUE"
	}
	return "FALSE"
}

func (d *PostgresDialect) FormatUpsert(conflictTarget, updateSet string) string {
	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", conflictTarget, updateSet)
}

func (d *PostgresDialect) ArrayParam(val []string) (interface{}, error) {
	return pq.Array(val), nil
}

func (d *PostgresDialect) ScanArray(src interface{}) ([]string, error) {
	var result []string
	if src == nil {
		return result, nil
	}
	switch v := src.(type) {
	case []byte:
		// Try pg array format first, then JSON
		s := string(v)
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			return parsePgArray(s), nil
		}
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, fmt.Errorf("scanning array: %w", err)
		}
		return result, nil
	case string:
		if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
			return parsePgArray(v), nil
		}
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return nil, fmt.Errorf("scanning array: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported array source type: %T", src)
	}
}

func (d *PostgresDialect) DateTrunc(field, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", field, column)
}

func (d *PostgresDialect) IntervalAgo(duration string) string {
	return fmt.Sprintf("NOW() - INTERVAL '%s'", duration)
}

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct{}

func (d *MySQLDialect) DBType() DBType                { return DBTypeMySQL }
func (d *MySQLDialect) Placeholder(index int) string   { return "?" }
func (d *MySQLDialect) SupportsReturning() bool        { return false }
func (d *MySQLDialect) Now() string                    { return "NOW()" }
func (d *MySQLDialect) BoolLiteral(val bool) string {
	if val {
		return "1"
	}
	return "0"
}

func (d *MySQLDialect) FormatUpsert(conflictTarget, updateSet string) string {
	// MySQL uses ON DUPLICATE KEY UPDATE; rewrite EXCLUDED.col to VALUES(col)
	updated := replaceExcludedRefs(updateSet)
	return fmt.Sprintf("ON DUPLICATE KEY UPDATE %s", updated)
}

func (d *MySQLDialect) ArrayParam(val []string) (interface{}, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshaling array param: %w", err)
	}
	return string(data), nil
}

func (d *MySQLDialect) ScanArray(src interface{}) ([]string, error) {
	return scanJSONArray(src)
}

func (d *MySQLDialect) DateTrunc(field, column string) string {
	switch field {
	case "hour":
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", column)
	case "day":
		return fmt.Sprintf("DATE(%s)", column)
	default:
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", column)
	}
}

func (d *MySQLDialect) IntervalAgo(duration string) string {
	// Convert "7 days" to MySQL format
	parts := strings.Fields(duration)
	if len(parts) == 2 {
		unit := strings.ToUpper(strings.TrimSuffix(parts[1], "s"))
		if unit == "DAY" {
			return fmt.Sprintf("NOW() - INTERVAL %s DAY", parts[0])
		}
	}
	return fmt.Sprintf("NOW() - INTERVAL %s", duration)
}

// SQLiteDialect implements Dialect for SQLite.
type SQLiteDialect struct{}

func (d *SQLiteDialect) DBType() DBType                { return DBTypeSQLite }
func (d *SQLiteDialect) Placeholder(index int) string   { return "?" }
func (d *SQLiteDialect) SupportsReturning() bool        { return false }
func (d *SQLiteDialect) Now() string                    { return "datetime('now')" }
func (d *SQLiteDialect) BoolLiteral(val bool) string {
	if val {
		return "1"
	}
	return "0"
}

func (d *SQLiteDialect) FormatUpsert(conflictTarget, updateSet string) string {
	// SQLite ON CONFLICT syntax is similar to PostgreSQL
	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", conflictTarget, updateSet)
}

func (d *SQLiteDialect) ArrayParam(val []string) (interface{}, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshaling array param: %w", err)
	}
	return string(data), nil
}

func (d *SQLiteDialect) ScanArray(src interface{}) ([]string, error) {
	return scanJSONArray(src)
}

func (d *SQLiteDialect) DateTrunc(field, column string) string {
	switch field {
	case "hour":
		return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', %s)", column)
	case "day":
		return fmt.Sprintf("date(%s)", column)
	default:
		return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', %s)", column)
	}
}

func (d *SQLiteDialect) IntervalAgo(duration string) string {
	parts := strings.Fields(duration)
	if len(parts) == 2 {
		return fmt.Sprintf("datetime('now', '-%s %s')", parts[0], parts[1])
	}
	return fmt.Sprintf("datetime('now', '-%s')", duration)
}

// NewDialect returns the appropriate dialect for the given DBType.
func NewDialect(dbType DBType) Dialect {
	switch dbType {
	case DBTypeMySQL:
		return &MySQLDialect{}
	case DBTypeSQLite:
		return &SQLiteDialect{}
	default:
		return &PostgresDialect{}
	}
}

// Helper functions

func parsePgArray(s string) []string {
	// Strip surrounding braces
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func scanJSONArray(src interface{}) ([]string, error) {
	var result []string
	if src == nil {
		return result, nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported array source type: %T", src)
	}
	if len(data) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("scanning JSON array: %w", err)
	}
	return result, nil
}

// replaceExcludedRefs rewrites "EXCLUDED.col" to "VALUES(col)" for MySQL.
func replaceExcludedRefs(s string) string {
	// Simple replacement: EXCLUDED.columnname -> VALUES(columnname)
	result := s
	for {
		idx := strings.Index(strings.ToUpper(result), "EXCLUDED.")
		if idx < 0 {
			break
		}
		// Find the column name after "EXCLUDED."
		prefix := result[:idx]
		rest := result[idx+9:] // skip "EXCLUDED."
		// Find end of column name
		end := 0
		for end < len(rest) && (rest[end] == '_' || (rest[end] >= 'a' && rest[end] <= 'z') || (rest[end] >= 'A' && rest[end] <= 'Z') || (rest[end] >= '0' && rest[end] <= '9')) {
			end++
		}
		colName := rest[:end]
		result = prefix + "VALUES(" + colName + ")" + rest[end:]
	}
	return result
}
