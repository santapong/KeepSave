package api

import (
	"fmt"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"sync"
	"testing"
)

func TestPlatformAuditTransactionAndReplicas(t *testing.T) {
	f := newPlatformFixture(t)
	key := []byte("synthetic-audit-key-not-for-production")
	a := repository.NewAuditRepository(f.db, f.d)
	a.SetChainKey(key)
	b := repository.NewAuditRepository(f.db, f.d)
	b.SetChainKey(key)
	// Multiple repository instances must serialize through durable database state.
	var wg sync.WaitGroup
	failures := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := a
			if i%2 != 0 {
				r = b
			}
			failures <- r.Create(&f.owner, &f.project, "platform.test", "alpha", models.JSONMap{"n": i}, "")
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if broken, err := a.VerifyChain(); err != nil || broken != nil {
		t.Fatalf("chain integrity: %v %v", broken, err)
	}
	var before string
	if err := f.db.QueryRow(`SELECT entry_hash FROM audit_chain_head WHERE id=1`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(repository.Q(f.d, `UPDATE projects SET name='must rollback' WHERE id=$1`), f.project); err != nil {
		t.Fatal(err)
	}
	if err = a.CreateTx(tx, &f.owner, &f.project, "platform.rolled_back", "alpha", models.JSONMap{}, ""); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var after, name string
	var n int
	if err = f.db.QueryRow(`SELECT entry_hash FROM audit_chain_head WHERE id=1`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='platform.rolled_back'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(repository.Q(f.d, `SELECT name FROM projects WHERE id=$1`), f.project).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if before != after || n != 0 || name == "must rollback" {
		t.Fatal("transaction escaped rollback")
	}
	if err = b.Create(&f.owner, &f.project, "platform.after_rollback", "alpha", models.JSONMap{}, ""); err != nil {
		t.Fatal(err)
	}
	if broken, err := a.VerifyChain(); err != nil || broken != nil {
		t.Fatal(fmt.Sprint(broken, err))
	}
}
