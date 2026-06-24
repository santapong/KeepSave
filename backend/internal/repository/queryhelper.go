package repository

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var pgPlaceholderRe = regexp.MustCompile(`\$(\d+)`)

// Rebind converts PostgreSQL-style $1, $2, ... placeholders to ? for MySQL/SQLite.
// For PostgreSQL dialect, the query is returned unchanged.
func Rebind(dialect Dialect, query string) string {
	if dialect.DBType() == DBTypePostgres {
		return query
	}
	return pgPlaceholderRe.ReplaceAllString(query, "?")
}

// RebindNow replaces NOW() with the dialect-appropriate function.
func RebindNow(dialect Dialect, query string) string {
	if dialect.DBType() == DBTypePostgres {
		return query
	}
	return strings.ReplaceAll(query, "NOW()", dialect.Now())
}

// RebindBool replaces TRUE/FALSE with dialect-appropriate literals.
func RebindBool(dialect Dialect, query string) string {
	if dialect.DBType() == DBTypePostgres {
		return query
	}
	query = strings.ReplaceAll(query, " TRUE", " "+dialect.BoolLiteral(true))
	query = strings.ReplaceAll(query, " FALSE", " "+dialect.BoolLiteral(false))
	query = strings.ReplaceAll(query, "=TRUE", "="+dialect.BoolLiteral(true))
	query = strings.ReplaceAll(query, "=FALSE", "="+dialect.BoolLiteral(false))
	return query
}

// Q applies all standard query rebinding: placeholders, NOW(), and booleans.
func Q(dialect Dialect, query string) string {
	query = Rebind(dialect, query)
	query = RebindNow(dialect, query)
	return query
}

// dbtx is satisfied by both *sql.DB and *sql.Tx, so repository helpers can run
// either standalone or inside a caller-managed transaction.
type dbtx interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// ExecQ rebinds a PostgreSQL-style ($1,$2,…) statement for the active dialect
// AND reorders the args to match placeholder *appearance* order before
// executing it. This is mandatory for any statement whose placeholders are not
// in strict 1..N positional order (e.g. `SET col=$2 WHERE id=$1`): Q() alone
// rewrites $N→? textually only, so on MySQL/SQLite the anonymous `?` bind
// positionally and the args mis-align — a silent no-op UPDATE (DB-12). For
// PostgreSQL, both Q and rebindArgs are passthroughs.
func ExecQ(db dbtx, dialect Dialect, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(Q(dialect, query), rebindArgs(dialect, query, args)...)
}

// QueryRowQ is the QueryRow counterpart of ExecQ (placeholder + arg rebinding).
func QueryRowQ(db dbtx, dialect Dialect, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(Q(dialect, query), rebindArgs(dialect, query, args)...)
}

// QueryQ is the Query counterpart of ExecQ (placeholder + arg rebinding).
func QueryQ(db dbtx, dialect Dialect, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(Q(dialect, query), rebindArgs(dialect, query, args)...)
}

// runInTx runs fn inside a transaction on db, committing on success and rolling
// back on any error. Shared by the repositories that expose a WithTx entry point.
func runInTx(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// InsertReturning executes an INSERT statement and scans the returned row.
// For PostgreSQL, it uses RETURNING clause.
// For MySQL/SQLite, it executes the INSERT then SELECTs by the given ID.
func InsertReturning(
	db *sql.DB,
	dialect Dialect,
	insertQuery string,
	args []interface{},
	table string,
	idColumn string,
	idValue interface{},
	selectColumns string,
	dest ...interface{},
) error {
	if dialect.SupportsReturning() {
		return db.QueryRow(insertQuery, args...).Scan(dest...)
	}

	// Execute the INSERT without RETURNING
	insertOnly := stripReturning(insertQuery)
	insertOnly = Q(dialect, insertOnly)
	reboundArgs := rebindArgs(dialect, insertQuery, args)
	_, err := db.Exec(insertOnly, reboundArgs...)
	if err != nil {
		return err
	}

	// SELECT back the row by ID
	selectQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", selectColumns, table, idColumn)
	return db.QueryRow(selectQuery, idValue).Scan(dest...)
}

// UpdateReturning executes an UPDATE statement and scans the returned row.
// For PostgreSQL, it uses RETURNING clause.
// For MySQL/SQLite, it executes the UPDATE then SELECTs.
func UpdateReturning(
	db *sql.DB,
	dialect Dialect,
	updateQuery string,
	args []interface{},
	table string,
	whereClause string,
	whereArgs []interface{},
	selectColumns string,
	dest ...interface{},
) error {
	if dialect.SupportsReturning() {
		return db.QueryRow(updateQuery, args...).Scan(dest...)
	}

	// Execute the UPDATE without RETURNING
	updateOnly := stripReturning(updateQuery)
	updateOnly = Q(dialect, updateOnly)
	reboundArgs := rebindArgs(dialect, updateQuery, args)
	_, err := db.Exec(updateOnly, reboundArgs...)
	if err != nil {
		return err
	}

	// SELECT back the row
	selectQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s", selectColumns, table, whereClause)
	return db.QueryRow(selectQuery, whereArgs...).Scan(dest...)
}

// UpsertReturning executes an INSERT...ON CONFLICT/ON DUPLICATE KEY and scans the returned row.
func UpsertReturning(
	db *sql.DB,
	dialect Dialect,
	pgQuery string,
	args []interface{},
	table string,
	conflictTarget string,
	updateSet string,
	idColumn string,
	idValue interface{},
	selectColumns string,
	dest ...interface{},
) error {
	if dialect.SupportsReturning() {
		return db.QueryRow(pgQuery, args...).Scan(dest...)
	}

	// Build the upsert query without RETURNING
	upsertOnly := stripReturning(pgQuery)
	// Replace ON CONFLICT clause for MySQL
	if dialect.DBType() == DBTypeMySQL {
		upsertOnly = replaceOnConflict(upsertOnly, dialect, conflictTarget, updateSet)
	}
	upsertOnly = Q(dialect, upsertOnly)
	reboundArgs := rebindArgs(dialect, pgQuery, args)
	_, err := db.Exec(upsertOnly, reboundArgs...)
	if err != nil {
		return err
	}

	// SELECT back the row
	selectQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", selectColumns, table, idColumn)
	return db.QueryRow(selectQuery, idValue).Scan(dest...)
}

// stripReturning removes the RETURNING clause from a SQL query.
func stripReturning(query string) string {
	upper := strings.ToUpper(query)
	idx := strings.LastIndex(upper, "RETURNING")
	if idx < 0 {
		return query
	}
	return strings.TrimSpace(query[:idx])
}

// rebindArgs reorders args for ? placeholders if needed.
// For PostgreSQL $N params, the args must match the $N order which may differ from positional.
// This handles the case where $1 is used multiple times or out of order.
func rebindArgs(dialect Dialect, query string, args []interface{}) []interface{} {
	if dialect.DBType() == DBTypePostgres {
		return args
	}

	// Find all $N references in order of appearance
	matches := pgPlaceholderRe.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return args
	}

	result := make([]interface{}, 0, len(matches))
	for _, match := range matches {
		idx, _ := strconv.Atoi(match[1])
		if idx > 0 && idx <= len(args) {
			result = append(result, args[idx-1])
		}
	}
	return result
}

// replaceOnConflict replaces PostgreSQL ON CONFLICT syntax with MySQL ON DUPLICATE KEY UPDATE.
func replaceOnConflict(query string, dialect Dialect, conflictTarget, updateSet string) string {
	upper := strings.ToUpper(query)
	idx := strings.Index(upper, "ON CONFLICT")
	if idx < 0 {
		return query
	}
	prefix := query[:idx]
	return prefix + dialect.FormatUpsert(conflictTarget, updateSet)
}
