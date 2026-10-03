package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/migrations"
)

// Exercise the actual router, real credentials and embedded migrations. No
// plaintext fixture, token or response body is printed on assertion failures.
func TestSecretIDScopeBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, suffix, target, scope, environment, credential string
		want                                                         int
	}{
		{"allowed read", "GET", "", "alpha", "read:DB_*", "alpha", "key", 200},
		{"environment denied", "GET", "", "prod", "read:DB_*", "alpha", "key", 404},
		{"forged query denied", "GET", "?environment=alpha", "prod", "read:DB_*", "alpha", "key", 404},
		{"key glob denied", "GET", "", "other-key", "read:DB_*", "alpha", "key", 404},
		{"history environment denied", "GET", "/versions", "prod", "read:DB_*", "alpha", "key", 404},
		{"history key denied", "GET", "/versions", "other-key", "read:DB_*", "alpha", "key", 404},
		{"version environment denied", "GET", "/versions/1", "prod", "read:DB_*", "alpha", "key", 404},
		{"version key denied", "GET", "/versions/1", "other-key", "read:DB_*", "alpha", "key", 404},
		{"history allowed", "GET", "/versions", "alpha", "read:DB_*", "alpha", "key", 200},
		{"version allowed", "GET", "/versions/1", "alpha", "read:DB_*", "alpha", "key", 200},
		{"update environment denied", "PUT", "", "prod", "write:DB_*", "alpha", "key", 404},
		{"update forged query denied", "PUT", "?environment=alpha", "prod", "write:DB_*", "alpha", "key", 404},
		{"update key denied", "PUT", "", "other-key", "write:DB_*", "alpha", "key", 404},
		{"delete environment denied", "DELETE", "", "prod", "write:DB_*", "alpha", "key", 404},
		{"delete key denied", "DELETE", "", "other-key", "delete:DB_*", "alpha", "key", 404},
		{"update allowed", "PUT", "", "alpha", "write:DB_*", "alpha", "key", 200},
		{"delete allowed", "DELETE", "", "alpha", "delete:DB_*", "alpha", "key", 204},
		{"read cannot update", "PUT", "", "alpha", "read:DB_*", "alpha", "key", 403},
		{"missing secret", "GET", "", "missing", "read:DB_*", "alpha", "key", 404},
		{"foreign secret", "GET", "/versions", "foreign", "read:DB_*", "alpha", "key", 404},
		{"unrestricted key", "GET", "/versions/1", "prod", "read", "", "key", 200},
		{"human owner unchanged", "GET", "/versions/1", "prod", "", "", "human", 200},
		{"anonymous denied", "GET", "/versions/1", "alpha", "", "", "", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, d, err := repository.NewDB("sqlite://" + filepath.Join(t.TempDir(), "scope.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
				t.Fatal(err)
			}
			owner, err := repository.NewUserRepository(db, d).Create(uuid.NewString()+"@example.invalid", "!")
			if err != nil {
				t.Fatal(err)
			}
			cs, err := crypto.NewService(make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			pr := repository.NewProjectRepository(db, d)
			er := repository.NewEnvironmentRepository(db, d)
			sr := repository.NewSecretRepository(db, d)
			vr := repository.NewSecretVersionRepository(db, d)
			ar := repository.NewAuditRepository(db, d)
			ps := service.NewProjectService(pr, er, ar, cs)
			ss := service.NewSecretService(sr, pr, er, ar, cs)
			project, err := ps.Create("Scope fixture", "", owner.ID, "")
			if err != nil {
				t.Fatal(err)
			}
			other, err := ps.Create("Other fixture", "", owner.ID, "")
			if err != nil {
				t.Fatal(err)
			}
			ids := map[string]uuid.UUID{"missing": uuid.New()}
			values := map[string]string{}
			for _, seed := range []struct {
				name, environment, key string
				project                uuid.UUID
			}{
				{"alpha", "alpha", "DB_URL", project.ID}, {"prod", "prod", "DB_URL", project.ID},
				{"other-key", "alpha", "PRIVATE_TOKEN", project.ID}, {"foreign", "alpha", "DB_URL", other.ID},
			} {
				value := uuid.NewString()
				s, err := ss.Create(seed.project, seed.environment, seed.key, value, owner.ID, "")
				if err != nil {
					t.Fatal(err)
				}
				ids[seed.name], values[seed.name] = s.ID, value
				raw, err := sr.GetByID(s.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = vr.CreateVersion(s.ID, s.ProjectID, s.EnvironmentID, 1, raw.EncryptedValue, raw.ValueNonce, &owner.ID); err != nil {
					t.Fatal(err)
				}
			}
			jwt := auth.NewJWTService("local-scope-test-signing-key-at-least-32-bytes")
			kr := repository.NewAPIKeyRepository(db, d)
			router := SetupRouter("http://localhost", true, nil, nil, jwt, kr, pr,
				&AuthHandler{}, nil, NewSecretHandler(ss), nil, nil, nil,
				nil, NewVersionHandler(vr, sr, pr, cs), nil,
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil,
				nil, nil, db, nil)
			updatedValue := uuid.NewString()
			body := ""
			if tc.method == "PUT" {
				body = `{"value":"` + updatedValue + `"}`
			}
			req := httptest.NewRequest(tc.method, "/api/v1/projects/"+project.ID.String()+"/secrets/"+ids[tc.target].String()+tc.suffix, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tc.credential == "key" {
				raw, hash, err := auth.GenerateAPIKey()
				if err != nil {
					t.Fatal(err)
				}
				var env *string
				if tc.environment != "" {
					env = &tc.environment
				}
				if _, err = kr.Create("scope fixture", hash, owner.ID, project.ID, []string{tc.scope}, env, nil); err != nil {
					t.Fatal(err)
				}
				req.Header.Set("X-API-Key", raw)
			} else if tc.credential == "human" {
				token, err := jwt.GenerateToken(owner.ID, owner.Email)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			var before int
			if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&before); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want >= 400 {
				for _, value := range values {
					if strings.Contains(w.Body.String(), value) {
						t.Fatal("denied response contains a secret")
					}
				}
				var after int
				if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&after); err != nil {
					t.Fatal(err)
				}
				if after != before {
					t.Fatal("denied operation emitted a mutation audit row")
				}
				if tc.target != "missing" {
					pid := project.ID
					if tc.target == "foreign" {
						pid = other.ID
					}
					s, err := ss.GetByID(pid, ids[tc.target])
					if err != nil || s.Value != values[tc.target] {
						t.Fatal("denied operation changed the stored value")
					}
				}
			} else if tc.method == "PUT" || tc.method == "DELETE" {
				action := "secret.updated"
				if tc.method == "DELETE" {
					action = "secret.deleted"
				}
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action = ? AND project_id = ?", action, project.ID).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 1 {
					t.Fatal("allowed mutation did not emit its audit row")
				}
				if tc.method == "PUT" {
					s, err := ss.GetByID(project.ID, ids[tc.target])
					if err != nil || s.Value != updatedValue {
						t.Fatal("allowed update did not persist its value")
					}
				}
			} else if !strings.Contains(w.Body.String(), values[tc.target]) {
				t.Fatal("allowed read did not return the requested value")
			}
		})
	}
}
