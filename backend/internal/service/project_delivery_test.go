package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"
	"testing"

	"github.com/lib/pq"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// Wrap a real PostgreSQL commit so authority changes in the precise interval
// where an unguarded follow-up lookup used to occur. SQL and locks remain real.
type projectCommitConnector struct {
	driver.Connector
	once        sync.Once
	afterCommit func() error
}

func (c *projectCommitConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &projectCommitConn{Conn: conn, owner: c}, nil
}

type projectCommitConn struct {
	driver.Conn
	owner *projectCommitConnector
}

func (c *projectCommitConn) Begin() (driver.Tx, error) {
	tx, err := c.Conn.Begin()
	if err != nil {
		return nil, err
	}
	return &projectCommitTx{Tx: tx, owner: c.owner}, nil
}
func (c *projectCommitConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if begin, ok := c.Conn.(driver.ConnBeginTx); ok {
		tx, err := begin.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &projectCommitTx{Tx: tx, owner: c.owner}, nil
	}
	return c.Begin()
}

type projectCommitTx struct {
	driver.Tx
	owner *projectCommitConnector
}

func (tx *projectCommitTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	var err error
	tx.owner.once.Do(func() { err = tx.owner.afterCommit() })
	return err
}

func TestProjectMetadataCapturedBeforeAuthorityBarrierRelease(t *testing.T) {
	for _, operation := range []string{"read", "update"} {
		t.Run(operation, func(t *testing.T) {
			f := newGrantFixture(t)
			if f.d.DBType() != repository.DBTypePostgres {
				t.Skip("real PostgreSQL commit boundary")
			}
			ctx := context.Background()
			var schema string
			if err := f.db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
				t.Fatal(err)
			}
			base, err := pq.NewConnector("postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable&search_path=" + schema + ",public")
			if err != nil {
				t.Fatal(err)
			}
			hook := &projectCommitConnector{Connector: base, afterCommit: func() error {
				if err := f.sessions.Revoke(ctx, f.human.SubjectID, f.human.SessionID, f.human.SessionID, ""); err != nil {
					return err
				}
				_, err := f.db.Exec(`UPDATE projects SET name='metadata-after-revocation' WHERE id=$1`, f.project)
				return err
			}}
			db := sql.OpenDB(hook)
			t.Cleanup(func() { _ = db.Close() })
			audit := repository.NewAuditRepository(db, f.d)
			audit.SetChainKey([]byte("synthetic-identity-audit-key-only"))
			projects := NewProjectService(repository.NewProjectRepository(db, f.d), repository.NewEnvironmentRepository(db, f.d), audit, nil)
			projects.EnableSessions(f.sessions)
			want := "Grant fixture"
			var name string
			if operation == "read" {
				result, err := projects.GetByIDAuthorized(ctx, f.human, f.project)
				if err != nil {
					t.Fatal(err)
				}
				name = result.Name
			} else {
				want = "metadata-admitted-update"
				result, err := projects.UpdateAuthorized(ctx, f.human, f.project, want, "", "")
				if err != nil {
					t.Fatal(err)
				}
				name = result.Name
			}
			if name != want {
				t.Fatal("delivered metadata read after authority release")
			}
			var revoked bool
			var stored string
			if err := f.db.QueryRow(`SELECT revoked FROM session_tokens WHERE id=$1`, f.human.SessionID).Scan(&revoked); err != nil || !revoked {
				t.Fatal("commit hook did not revoke parent", err)
			}
			if err := f.db.QueryRow(`SELECT name FROM projects WHERE id=$1`, f.project).Scan(&stored); err != nil || stored != "metadata-after-revocation" {
				t.Fatal("commit hook did not change metadata", err)
			}
			if result, err := projects.GetByIDAuthorized(ctx, f.human, f.project); err == nil || result != nil {
				t.Fatal("later revoked session read allowed")
			}
		})
	}
}
