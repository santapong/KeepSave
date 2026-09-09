package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// PoolOptions bounds database connections per API process. SQLite always uses
// one connection; remote databases can be tuned for the deployment's budget.
type PoolOptions struct {
	MaxOpen int
	MaxIdle int
}

func DefaultPoolOptions() PoolOptions { return PoolOptions{MaxOpen: 10, MaxIdle: 2} }

// connMaxLifetime bounds connection age below the typical idle-termination
// window of managed Postgres providers (Neon evicts at ~5min). Without this,
// the pool hands out connections the server has already closed, surfacing as
// cascading 500s every few minutes.
const (
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 2 * time.Minute
)

// NewDB opens a database connection and returns the db handle along with the detected dialect.
// The database type is auto-detected from the DATABASE_URL scheme:
//   - postgres:// or postgresql:// -> PostgreSQL
//   - mysql:// -> MySQL
//   - sqlite://, file:, or .db/.sqlite path -> SQLite
func NewDB(databaseURL string) (*sql.DB, Dialect, error) {
	return NewDBWithPool(databaseURL, DefaultPoolOptions())
}

func NewDBWithPool(databaseURL string, pool PoolOptions) (*sql.DB, Dialect, error) {
	if pool.MaxOpen < 1 || pool.MaxIdle < 0 || pool.MaxIdle > pool.MaxOpen {
		return nil, nil, fmt.Errorf("invalid database pool: require max open >= 1 and 0 <= max idle <= max open")
	}
	dbType := DetectDBType(databaseURL)
	dialect := NewDialect(dbType)

	var db *sql.DB
	var err error

	switch dbType {
	case DBTypePostgres:
		db, err = sql.Open("postgres", databaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("opening postgres database: %w", err)
		}
		db.SetMaxOpenConns(pool.MaxOpen)
		db.SetMaxIdleConns(pool.MaxIdle)
		db.SetConnMaxLifetime(connMaxLifetime)
		db.SetConnMaxIdleTime(connMaxIdleTime)

	case DBTypeMySQL:
		dsn, dsnErr := mysqlURLToDSN(databaseURL)
		if dsnErr != nil {
			return nil, nil, fmt.Errorf("parsing mysql URL: %w", dsnErr)
		}
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("opening mysql database: %w", err)
		}
		db.SetMaxOpenConns(pool.MaxOpen)
		db.SetMaxIdleConns(pool.MaxIdle)
		db.SetConnMaxLifetime(connMaxLifetime)
		db.SetConnMaxIdleTime(connMaxIdleTime)

	case DBTypeSQLite:
		dsn, dsnErr := sqliteDSN(databaseURL)
		if dsnErr != nil {
			return nil, nil, dsnErr
		}
		db, err = sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("opening sqlite database: %w", err)
		}
		// SQLite supports only one writer at a time
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// Connection-local pragmas are supplied through the DSN, so replacement
		// connections also enforce foreign keys and wait for short write locks.

	default:
		return nil, nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, dialect, nil
}

// mysqlURLToDSN converts a mysql:// URL to a Go MySQL DSN string.
// mysql://user:pass@host:port/dbname -> user:pass@tcp(host:port)/dbname?parseTime=true
func mysqlURLToDSN(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing MySQL URL: %w", err)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}

	dbName := strings.TrimPrefix(u.Path, "/")

	var userInfo string
	if u.User != nil {
		password, _ := u.User.Password()
		userInfo = u.User.Username() + ":" + password
	}

	dsn := fmt.Sprintf("%s@tcp(%s:%s)/%s?parseTime=true", userInfo, host, port, dbName)

	// Append any additional query parameters
	if u.RawQuery != "" {
		dsn += "&" + u.RawQuery
	}

	return dsn, nil
}

// sqliteDSN normalizes a SQLite connection string.
func sqliteDSN(rawURL string) (string, error) {
	rawURL = strings.TrimPrefix(rawURL, "sqlite://")
	rawURL = strings.TrimPrefix(rawURL, "sqlite:")
	name, query, _ := strings.Cut(rawURL, "?")
	params, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("invalid SQLite connection options")
	}
	params.Del("_fk")
	params.Del("_journal")
	params.Set("_foreign_keys", "on")
	params.Set("_journal_mode", "WAL")
	if !params.Has("_busy_timeout") && !params.Has("_timeout") {
		params.Set("_busy_timeout", "5000")
	}
	return name + "?" + params.Encode(), nil
}
