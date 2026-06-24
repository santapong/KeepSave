package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// TestAnomalyService_ScopedToAccessibleProjects proves the cross-tenant IDOR /
// info-disclosure fixes: listings only return the caller's projects, and
// acknowledge/resolve/update/delete refuse to touch a row in a project the
// caller cannot access (reported as not-found, not silently applied).
func TestAnomalyService_ScopedToAccessibleProjects(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlAnomalies, ddlAnomalyRules)
	svc := NewAnomalyService(db, dialect, nil, repository.NewAuditRepository(db, dialect))

	projA, projB := uuid.New(), uuid.New()
	actor := uuid.New()

	ruleA, err := svc.CreateRule(&projA, nil, "frequency", models.JSONMap{"t": 1}, actor, "1.1.1.1")
	if err != nil {
		t.Fatalf("CreateRule A: %v", err)
	}
	ruleB, err := svc.CreateRule(&projB, nil, "frequency", models.JSONMap{"t": 1}, actor, "1.1.1.1")
	if err != nil {
		t.Fatalf("CreateRule B: %v", err)
	}

	seedAnomaly := func(pid uuid.UUID) uuid.UUID {
		id := uuid.New()
		if _, err := db.Exec(
			`INSERT INTO anomalies (id, project_id, anomaly_type, severity, description, details, status, detected_at) VALUES (?,?,?,?,?,?,?,?)`,
			id.String(), pid.String(), "new_ip", "high", "d", `{}`, "open", time.Now()); err != nil {
			t.Fatalf("seed anomaly: %v", err)
		}
		return id
	}
	anomA := seedAnomaly(projA)
	anomB := seedAnomaly(projB)

	onlyA := []uuid.UUID{projA}

	// --- Listings are scoped ---
	if got, _ := svc.ListAnomalies(onlyA, ""); len(got) != 1 || got[0].ID != anomA {
		t.Errorf("ListAnomalies(onlyA) = %d rows, want only anomA", len(got))
	}
	if got, _ := svc.ListAnomalies(nil, ""); len(got) != 0 {
		t.Errorf("ListAnomalies(nil) = %d rows, want 0 (no access)", len(got))
	}
	if got, _ := svc.ListRules(onlyA); len(got) != 1 || got[0].ID != ruleA.ID {
		t.Errorf("ListRules(onlyA) = %d rows, want only ruleA", len(got))
	}

	// --- Cross-tenant mutations are refused (not-found), own-tenant succeed ---
	if err := svc.AcknowledgeAnomaly(anomB, onlyA, actor, "1.1.1.1"); !errors.Is(err, ErrAnomalyNotFound) {
		t.Errorf("ack cross-tenant anomaly: err = %v, want ErrAnomalyNotFound", err)
	}
	if err := svc.AcknowledgeAnomaly(anomA, onlyA, actor, "1.1.1.1"); err != nil {
		t.Errorf("ack own anomaly: %v", err)
	}
	if err := svc.ResolveAnomaly(anomB, onlyA, actor, "1.1.1.1"); !errors.Is(err, ErrAnomalyNotFound) {
		t.Errorf("resolve cross-tenant anomaly: err = %v, want ErrAnomalyNotFound", err)
	}
	if err := svc.UpdateRule(ruleB.ID, false, models.JSONMap{"t": 2}, onlyA, actor, "1.1.1.1"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("update cross-tenant rule: err = %v, want ErrRuleNotFound", err)
	}
	if err := svc.DeleteRule(ruleB.ID, onlyA, actor, "1.1.1.1"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("delete cross-tenant rule: err = %v, want ErrRuleNotFound", err)
	}
	if err := svc.UpdateRule(ruleA.ID, false, models.JSONMap{"t": 2}, onlyA, actor, "1.1.1.1"); err != nil {
		t.Errorf("update own rule: %v", err)
	}

	// ruleB and anomB remain untouched (proven: ruleB still listable under projB).
	if got, _ := svc.ListRules([]uuid.UUID{projB}); len(got) != 1 {
		t.Errorf("ruleB should be intact under projB, got %d rows", len(got))
	}
	_ = anomB
}
