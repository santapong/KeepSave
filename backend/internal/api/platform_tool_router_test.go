package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/harness"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func toolRouterFixture(t *testing.T) (*coreFixture, *runs.Service) {
	t.Helper()
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("requires disposable real PostgreSQL")
	}
	f := newCoreFixture(t)
	cs, e := crypto.NewService(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	a := repository.NewAuditRepository(f.db, f.d)
	s := runs.New(f.db, broker.New(broker.CryptoCustody{Service: cs}), func(ctx context.Context, tx *sql.Tx, p policy.Principal, action policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, action, r)
	}, a, runs.Flags{Enabled: true, Admission: true, Dispatch: true, Endpoint: "https://app.keepsave.example/mcp"})
	guard := authority.Postgres(f.db)
	s.LockProjectSubjects = guard.LockProjectSubjects
	s.LockMaintenanceSubjects = guard.LockProjectMaintenance
	s.PackageExporter = harness.ExportAutomation
	f.r = NewRouter(Dependencies{CoreRelease: true, CORSOrigins: "http://localhost", JWTService: f.jwt, APIKeyRepo: f.keys, ProjectRepo: repository.NewProjectRepository(f.db, f.d), AuthHandler: &AuthHandler{}, ToolPlatformHandler: NewToolPlatformHandler(s), DB: f.db})
	return f, s
}
func TestPlatformToolRouterMetadataStrictInputsAndTenant(t *testing.T) {
	f, _ := toolRouterFixture(t)
	base := "/api/v1/projects/" + f.project.String() + "/tool-platform"
	source := "---\nname: router-review\ndescription: Review one approved target.\n---\nUse the explicit bounded tool workflow.\n"
	body, _ := json.Marshal(map[string]string{"name": "router-review", "source": source})
	w := f.request("POST", base+"/artifacts", string(body), f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/artifacts", "post", w, 201)
	var artifact runs.Artifact
	if e := json.Unmarshal(w.Body.Bytes(), &artifact); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(w.Body.String(), source) {
		t.Fatal("artifact creation response exposed source instead of metadata")
	}
	w = f.request("GET", base+"/artifacts", "", f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/artifacts", "get", w, 200)
	w = f.request("GET", base+"/catalog", "", f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/catalog", "get", w, 200)
	if !strings.Contains(w.Body.String(), "keepsave_v1__read_file") {
		t.Fatal("qualified reviewed catalog missing")
	}
	w = f.request("POST", base+"/profiles", `{"artifact_id":"`+artifact.ID.String()+`"}`, f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/profiles", "post", w, 201)
	var profile runs.Profile
	_ = json.Unmarshal(w.Body.Bytes(), &profile)
	w = f.request("POST", base+"/profiles/"+profile.ID.String()+"/approve", `{"digest":"`+profile.Digest+`"}`, f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/profiles/{resourceId}/approve", "post", w, 403)
	for _, test := range []struct {
		method, path, body string
		user               uuid.UUID
		status             int
	}{{"GET", base + "/artifacts", "", f.other, 403}, {"GET", base + "/artifacts", "", uuid.Nil, 401}, {"POST", base + "/artifacts", `{"name":"one","name":"two","source":"x"}`, f.owner, 400}, {"POST", base + "/artifacts", `{"name":"one","source":"x","command":"sh"}`, f.owner, 400}, {"POST", base + "/bindings", `{"environment":"prod","connection_id":"` + uuid.NewString() + `","target":{"repository_id":1,"owner":"example","repository":"one","reference":"main"}}`, f.owner, 403}} {
		w = f.request(test.method, test.path, test.body, test.user, "")
		if w.Code != test.status {
			t.Fatalf("strict/tenant expected%d got%d", test.status, w.Code)
		}
		var safe ErrorResponse
		if json.Unmarshal(w.Body.Bytes(), &safe) != nil || safe.Error.Code != test.status {
			t.Fatal("unsafe error envelope")
		}
	}
	w = f.request("GET", base+"/runs", "", f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/runs", "get", w, 200)
	if !strings.Contains(w.Body.String(), `"runs":[]`) {
		t.Fatal("owned run list missing")
	}
	w = f.request("POST", base+"/connections/"+uuid.NewString()+"/check", `{"binding_id":"`+uuid.NewString()+`","allow_external_read":false,"allow_authorization_token_mint":false}`, f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/connections/{resourceId}/check", "post", w, 403)
}
func toolCertificates(t *testing.T) (*x509.CertPool, tls.Certificate, string) {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic runner CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	parsed, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "synthetic enrolled workload"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	leafDER, e := x509.CreateCertificate(rand.Reader, leaf, parsed, &leafKey.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	private, e := x509.MarshalPKCS8PrivateKey(leafKey)
	if e != nil {
		t.Fatal(e)
	}
	cert, e := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(leafDER)
	return pool, cert, hex.EncodeToString(sum[:])
}
func TestPlatformRunnerActualMTLSAndHeaderSpoofDenial(t *testing.T) {
	f, service := toolRouterFixture(t)
	pool, cert, fingerprint := toolCertificates(t)
	image := "localhost/keepsave-connector@sha256:" + strings.Repeat("a", 64)
	base := "/api/v1/projects/" + f.project.String() + "/tool-platform/workloads"
	w := f.request("POST", base, `{"certificate_sha256":"`+fingerprint+`","image_digest":"`+image+`"}`, f.owner, "")
	assertCoreResponse(t, "/projects/{id}/tool-platform/workloads", "post", w, 201)
	server := httptest.NewUnstartedServer(RunnerRouter(NewToolPlatformHandler(service)))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	trust := x509.NewCertPool()
	trust.AddCert(server.Certificate())
	call := func(withCert bool, body string, headers map[string]string) (int, []byte) {
		t.Helper()
		cfg := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: trust}
		if withCert {
			cfg.Certificates = []tls.Certificate{cert}
		}
		transport := &http.Transport{TLSClientConfig: cfg}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/runner/operations/claim", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	body := `{"image_digest":"` + image + `"}`
	public := f.request("POST", "/api/v1/runner/operations/claim", body, f.owner, "")
	if public.Code != 404 {
		t.Fatal("runner route exposed on public router")
	}
	status, b := call(false, body, map[string]string{"X-Client-Cert-SHA256": fingerprint, "X-Forwarded-Client-Cert": fingerprint})
	if status != 403 || strings.Contains(string(b), fingerprint) {
		t.Fatal("unverified header identity accepted or leaked")
	}
	status, _ = call(true, body, nil)
	if status != 204 {
		t.Fatalf("actual enrolled TLS certificate idleclaim=%d", status)
	}
	status, _ = call(true, `{"image_digest":"`+image+`","image_digest":"other"}`, nil)
	if status != 400 {
		t.Fatal("duplicate typed runner field accepted")
	}
	status, _ = call(true, `{"image_digest":"localhost/unapproved@sha256:`+strings.Repeat("b", 64)+`"}`, nil)
	if status != 403 {
		t.Fatal("certificate widened connector digest")
	}
}
func TestToolUniqueJSONRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"nested":{"x":1,"x":2}}`, `{"name":"one","Name":"two"}`, `{"Name":"one"}`, `{"a":1} {"b":2}`, `{"a":1`, strings.Repeat("[", 26) + "0" + strings.Repeat("]", 26)} {
		if toolUniqueJSON([]byte(raw)) {
			t.Fatal("ambiguous JSON accepted")
		}
	}
	if !toolUniqueJSON([]byte(`{"a":{"b":1},"c":[1,2]}`)) {
		t.Fatal("valid JSON refused")
	}
}
