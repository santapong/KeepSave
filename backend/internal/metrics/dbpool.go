package metrics

import (
	"context"
	"database/sql"
	"time"
)

// StartDBPoolUpdater polls db.Stats() at the given interval and writes
// the values onto the AppMetrics gauges. Exits when ctx is cancelled
// (SIGTERM/SIGINT). 15s matches the typical Prometheus scrape interval -
// finer resolution wastes work, coarser misses transient saturation.
//
// Per audit B-L1. Pairs with the Neon-tuned pool config (ConnMaxLifetime
// = 5m) - operators can alert when in_use approaches max_open or when
// open drops then rises (pool churn from idle-conn eviction).
func StartDBPoolUpdater(ctx context.Context, db *sql.DB, m *AppMetrics) {
	if db == nil || m == nil {
		return
	}
	interval := 15 * time.Second
	t := time.NewTicker(interval)
	defer t.Stop()

	publish := func() {
		s := db.Stats()
		m.DBOpenConnections.Set("", int64(s.OpenConnections))
		m.DBInUseConnections.Set("", int64(s.InUse))
		m.DBIdleConnections.Set("", int64(s.Idle))
	}
	publish()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			publish()
		}
	}
}
