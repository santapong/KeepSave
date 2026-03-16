package repository

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// NewDB opens a database connection and returns the db handle along with the detected dialect.
// The database type is auto-detected from the DATABASE_URL scheme:
//   - postgres:// or postgresql:// -> PostgreSQL
//   - mysql:// -> MySQL
//   - sqlite://, file:, or .db/.sqlite path -> SQLite
func NewDB(databaseURL string) (*sql.DB, Dialect, error) {
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
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)

	case DBTypeMySQL:
		dsn, dsnErr := mysqlURLToDSN(databaseURL)
		if dsnErr != nil {
			return nil, nil, fmt.Errorf("parsing mysql URL: %w", dsnErr)
		}
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("opening mysql database: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)

	case DBTypeSQLite:
		dsn := sqliteDSN(databaseURL)
		db, err = sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("opening sqlite database: %w", err)
		}
		// SQLite supports only one writer at a time
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// Enable WAL mode and foreign keys
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, nil, fmt.Errorf("setting WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return nil, nil, fmt.Errorf("enabling foreign keys: %w", err)
		}

	default:
		return nil, nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	if err := db.Ping(); err != nil {
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
func sqliteDSN(rawURL string) string {
	if rawURL == ":memory:" {
		return rawURL
	}
	rawURL = strings.TrimPrefix(rawURL, "sqlite://")
	rawURL = strings.TrimPrefix(rawURL, "sqlite:")
	return rawURL
}
