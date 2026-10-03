package auditview

import (
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"testing"
	"time"
)

func TestSafeReferencesAllowOnlyTypedIdentifiers(t *testing.T) {
	id := uuid.NewString()
	r := safeReferences(models.JSONMap{"secret_id": id, "project_id": "contains-secret", "credential": "provider-canary", "details": map[string]any{"run_id": id}})
	if len(r) != 1 || r["secret_id"] != id {
		t.Fatal("unsafe audit projection", r)
	}
}
func TestAuditLimits(t *testing.T) {
	base := Filter{From: time.Now().Add(-time.Hour), To: time.Now(), Limit: 100}
	if validate(base) != nil {
		t.Fatal("valid filter denied")
	}
	for _, limit := range []int{0, 101} {
		f := base
		f.Limit = limit
		if validate(f) == nil {
			t.Fatal("unbounded page accepted")
		}
	}
	base.From = base.To.Add(-32 * 24 * time.Hour)
	if validate(base) == nil {
		t.Fatal("unbounded export window")
	}
}
