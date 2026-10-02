package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Hold a dedicated connection until recovery finishes. This operator-only lock
// serializes recoveries; the target must have no application tables before any
// migration executes, even when those tables happen to contain zero rows.
func lockFreshRecoveryTarget(ctx context.Context, db *sql.DB) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	release := func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, `SELECT pg_advisory_unlock(773468255289)`)
		_ = conn.Close()
	}
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(773468255289)`).Scan(&locked); err != nil || !locked {
		_ = conn.Close()
		return nil, fmt.Errorf("isolated recovery target unavailable")
	}
	var tables int
	err = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','f') AND n.nspname <> 'information_schema' AND n.nspname NOT LIKE 'pg_%'`).Scan(&tables)
	if err != nil || tables != 0 {
		release()
		return nil, fmt.Errorf("isolated recovery requires a fresh database without application tables")
	}
	return release, nil
}
