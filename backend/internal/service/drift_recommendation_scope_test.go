package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

const ddlDriftSchedules = `CREATE TABLE drift_schedules (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	source_env TEXT,
	target_env TEXT,
	cron_expr TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	last_run_at TIMESTAMP,
	next_run_at TIMESTAMP,
	created_at TIMESTAMP,
	updated_at TIMESTAMP
)`

const ddlSecretRecommendations = `CREATE TABLE secret_recommendations (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	recomm_type TEXT,
	severity TEXT,
	title TEXT,
	description TEXT,
	affected_keys TEXT,
	suggested_action TEXT,
	auto_fixable INTEGER,
	status TEXT,
	created_at TIMESTAMP
)`

// TestDriftAndRecommendation_BoundToProject proves the IDOR fixes: a
// schedule/recommendation can only be mutated through the project it belongs
// to; passing a different (authorized) project id is reported as not-found
// rather than silently mutating another tenant's row.
func TestDriftAndRecommendation_BoundToProject(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlDriftSchedules, ddlSecretRecommendations)
	drift := &DriftService{db: db, dialect: dialect}
	recomm := &RecommendationService{db: db, dialect: dialect}

	projA, projB := uuid.New(), uuid.New()

	schedB := uuid.New()
	if _, err := db.Exec(`INSERT INTO drift_schedules (id, project_id, source_env, target_env, cron_expr, enabled, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		schedB.String(), projB.String(), "alpha", "uat", "0 * * * *", 1, time.Now(), time.Now()); err != nil {
		t.Fatalf("seed schedule: %v", err)
	}
	recB := uuid.New()
	if _, err := db.Exec(`INSERT INTO secret_recommendations (id, project_id, recomm_type, severity, title, description, affected_keys, suggested_action, auto_fixable, status, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		recB.String(), projB.String(), "rotation", "high", "t", "d", `[]`, "rotate", 0, "open", time.Now()); err != nil {
		t.Fatalf("seed recommendation: %v", err)
	}

	// Cross-project mutations are refused.
	if err := drift.UpdateSchedule(schedB, projA, false, "1 * * * *"); !errors.Is(err, ErrDriftScheduleNotFound) {
		t.Errorf("cross-project UpdateSchedule: err = %v, want ErrDriftScheduleNotFound", err)
	}
	if err := drift.DeleteSchedule(schedB, projA); !errors.Is(err, ErrDriftScheduleNotFound) {
		t.Errorf("cross-project DeleteSchedule: err = %v, want ErrDriftScheduleNotFound", err)
	}
	if err := recomm.DismissRecommendation(recB, projA); !errors.Is(err, ErrRecommendationNotFound) {
		t.Errorf("cross-project DismissRecommendation: err = %v, want ErrRecommendationNotFound", err)
	}

	// Correct project succeeds.
	if err := drift.UpdateSchedule(schedB, projB, false, "1 * * * *"); err != nil {
		t.Errorf("own-project UpdateSchedule: %v", err)
	}
	if err := recomm.DismissRecommendation(recB, projB); err != nil {
		t.Errorf("own-project DismissRecommendation: %v", err)
	}
	if err := drift.DeleteSchedule(schedB, projB); err != nil {
		t.Errorf("own-project DeleteSchedule: %v", err)
	}
}
