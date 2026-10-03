package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/identity"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

type identityFixture struct {
	*platformGrantFixture
	identity *identity.Service
	audit    *repository.AuditRepository
}

func (f *identityFixture) requestIdempotent(method, path, body, key, token string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Authorization", "Bearer "+token)
	result := httptest.NewRecorder()
	f.r.ServeHTTP(result, req)
	return result
}

func newIdentityFixture(t *testing.T) *identityFixture {
	t.Helper()
	f := newPlatformGrantFixture(t)
	if f.d.DBType() != repository.DBTypePostgres {
		t.Skip("real PostgreSQL identity platform contract")
	}
	audit := repository.NewAuditRepository(f.db, f.d)
	audit.SetChainKey([]byte("synthetic-grant-http-audit-key"))
	c, e := crypto.NewService(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	s := identity.New(f.db, identity.Config{Enabled: true, PostgreSQL: true, SMTPConfigured: true, ApplicationOrigin: "https://app.example.invalid", EnabledSocialProviders: map[string]bool{"google": true, "github": true}}, audit, vault.NewPayloadCustody(c), authority.Guard{DB: f.db, Dialect: f.d})
	s.EnableRecoveryRevocation(f.sessions)
	r := gin.New()
	NewIdentityPlatformHandler(s).RegisterRoutes(r.Group("/api/v1"), r.Group("/api/v1", JWTAuthMiddleware(f.jwt)))
	f.r = r
	return &identityFixture{f, s, audit}
}
func (f *identityFixture) principal(t *testing.T, token string) policy.Principal {
	t.Helper()
	c, e := f.jwt.ValidateToken(token)
	if e != nil {
		t.Fatal(e)
	}
	sid, e := uuid.Parse(c.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	return policy.Principal{Kind: policy.Human, SubjectID: c.UserID, ActorID: c.UserID, SessionID: sid}
}
func (f *identityFixture) session(t *testing.T, user uuid.UUID, method string) string {
	t.Helper()
	u, e := repository.NewUserRepository(f.db, f.d).GetByID(user)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := f.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	token, e := f.sessions.IssueTx(context.Background(), tx, u, method, "", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	return token
}
func identityRequestID(t *testing.T, body string) uuid.UUID {
	t.Helper()
	var value struct {
		Request identity.ProofRequest `json:"request"`
	}
	if e := json.Unmarshal([]byte(body), &value); e != nil {
		t.Fatal(e)
	}
	if value.Request.ID == uuid.Nil {
		t.Fatal("missing nonsecret request id")
	}
	return value.Request.ID
}

type syntheticProofMail struct {
	mu      sync.Mutex
	calls   int
	body    string
	outcome identity.DeliveryOutcome
}

func (m *syntheticProofMail) Send(_ context.Context, _ string, _ string, body string) identity.DeliveryOutcome {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.body = body
	return m.outcome
}
func (m *syntheticProofMail) proof(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, field := range strings.Fields(m.body) {
		if strings.HasPrefix(field, "https://") {
			u, e := url.Parse(field)
			if e != nil {
				t.Fatal(e)
			}
			q, e := url.ParseQuery(u.Fragment)
			if e != nil {
				t.Fatal(e)
			}
			if raw := q.Get("proof"); len(raw) == 43 {
				return raw
			}
		}
	}
	t.Fatal("synthetic message missing proof")
	return ""
}
func (f *identityFixture) deliver(t *testing.T, id uuid.UUID, m *syntheticProofMail) string {
	t.Helper()
	var delivery uuid.UUID
	if e := f.db.QueryRow(`SELECT id FROM identity_proof_deliveries WHERE proof_id=$1`, id).Scan(&delivery); e != nil {
		t.Fatal(e)
	}
	if _, e := f.identity.Deliver(context.Background(), delivery, m); e != nil {
		t.Fatal(e)
	}
	return m.proof(t)
}
func (f *identityFixture) organization(t *testing.T) uuid.UUID {
	t.Helper()
	orgs := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), f.audit)
	org, e := orgs.CreateWorkspace("Synthetic identity team", f.owner, "", uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	return org.ID
}
func identityCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if e := db.QueryRow(query, args...).Scan(&count); e != nil {
		t.Fatal(e)
	}
	return count
}

func TestPlatformIdentityContactAtomicReplayAndProofIsolation(t *testing.T) {
	f := newIdentityFixture(t)
	w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"proof-owner@example.invalid"}`, "", f.token)
	if w.Code != 202 {
		t.Fatalf("request=%d %s", w.Code, w.Body.String())
	}
	id := identityRequestID(t, w.Body.String())
	var lifetime int
	if e := f.db.QueryRow(`SELECT EXTRACT(EPOCH FROM (expires_at-created_at))::integer FROM identity_proofs WHERE id=$1`, id).Scan(&lifetime); e != nil || lifetime != 900 {
		t.Fatalf("proof lifetime=%d err=%v", lifetime, e)
	}
	mail := &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}}
	raw := f.deliver(t, id, mail)
	var jobs, audit string
	if e := f.db.QueryRow(`SELECT COALESCE(string_agg(payload::text,''),'') FROM outbox_jobs`).Scan(&jobs); e != nil {
		t.Fatal(e)
	}
	if e := f.db.QueryRow(`SELECT COALESCE(string_agg(details::text,''),'') FROM audit_log`).Scan(&audit); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(jobs, raw) || strings.Contains(audit, raw) || strings.Contains(w.Body.String(), raw) {
		t.Fatal("proof escaped ID-only custody contract")
	}
	if _, e := f.db.Exec(grantHTTPAuditFailureDDL(f.d, "identity.proof_consumed")); e != nil {
		t.Fatal(e)
	}
	confirm := "/api/v1/account/contact-proofs/" + id.String() + "/confirm"
	body := fmt.Sprintf(`{"proof":"%s"}`, raw)
	if w = f.request("POST", confirm, body, "", f.token); w.Code != 500 {
		t.Fatal("required audit failure did not fail confirmation", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1`, f.owner) != 0 || identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proofs WHERE id=$1 AND consumed`, id) != 0 {
		t.Fatal("proof/contact escaped audit rollback")
	}
	if _, e := f.db.Exec(`DROP TRIGGER fail_grant_http_audit ON audit_log; DROP FUNCTION grant_http_audit_failure()`); e != nil {
		t.Fatal(e)
	}
	responses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- f.request("POST", confirm, body, "", f.token).Code }()
	}
	wg.Wait()
	close(responses)
	ok, bad := 0, 0
	for code := range responses {
		if code == 204 {
			ok++
		} else if code == 400 {
			bad++
		} else {
			t.Fatal("concurrent proof code", code)
		}
	}
	if ok != 1 || bad != 1 {
		t.Fatal("proof consumed more than once")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1`, f.owner) != 1 {
		t.Fatal("verified contact missing")
	}
	if w = f.request("GET", "/api/v1/account/contacts", "", "", f.token); w.Code != 200 || !strings.Contains(w.Body.String(), "proof-owner@example.invalid") {
		t.Fatal("contact metadata", w.Code)
	}
}

func TestPlatformIdentitySMTPUncertaintyRequiresFreshManualProof(t *testing.T) {
	f := newIdentityFixture(t)
	w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"uncertain@example.invalid"}`, "", f.token)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	id := identityRequestID(t, w.Body.String())
	mail := &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "uncertain", Reason: "smtp_acceptance_unknown"}}
	old := f.deliver(t, id, mail)
	var delivery uuid.UUID
	if e := f.db.QueryRow(`SELECT id FROM identity_proof_deliveries WHERE proof_id=$1`, id).Scan(&delivery); e != nil {
		t.Fatal(e)
	}
	if _, e := f.identity.Deliver(context.Background(), delivery, mail); e == nil {
		t.Fatal("uncertain mail was automatically resent")
	}
	if mail.calls != 1 {
		t.Fatal("duplicate SMTP effect")
	}
	w = f.request("POST", "/api/v1/account/proofs/"+id.String()+"/resend", "{}", "", f.token)
	if w.Code != 202 {
		t.Fatal("manual fresh proof", w.Code, w.Body.String())
	}
	fresh := identityRequestID(t, w.Body.String())
	if fresh == id {
		t.Fatal("resend reused old proof")
	}
	if w = f.request("POST", "/api/v1/account/contact-proofs/"+id.String()+"/confirm", fmt.Sprintf(`{"proof":"%s"}`, old), "", f.token); w.Code != 400 {
		t.Fatal("old proof remains valid", w.Code)
	}
	mail.outcome = identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}
	raw := f.deliver(t, fresh, mail)
	if raw == old {
		t.Fatal("resend retained secret proof")
	}
	if w = f.request("POST", "/api/v1/account/contact-proofs/"+fresh.String()+"/confirm", fmt.Sprintf(`{"proof":"%s"}`, raw), "", f.token); w.Code != 204 {
		t.Fatal("fresh proof failed", w.Code)
	}
}

func TestPlatformIdentityRecoveryUsesVerifiedAccountAndRevokesSessions(t *testing.T) {
	f := newIdentityFixture(t)
	if _, e := f.db.Exec(`INSERT INTO identity_verified_contacts(user_id,contact,verified_at) VALUES($1,'recovery@example.invalid',NOW())`, f.owner); e != nil {
		t.Fatal(e)
	}
	unknown := f.request("POST", "/api/v1/auth/recovery/request", `{"contact":"unknown@example.invalid"}`, "", "")
	known := f.request("POST", "/api/v1/auth/recovery/request", `{"contact":"recovery@example.invalid"}`, "", "")
	if unknown.Code != 202 || known.Code != 202 {
		t.Fatal("recovery enumeration shape")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proofs WHERE id=$1`, identityRequestID(t, unknown.Body.String())) != 0 {
		t.Fatal("unknown contact got account authority")
	}
	id := identityRequestID(t, known.Body.String())
	raw := f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
	body := fmt.Sprintf(`{"id":"%s","proof":"%s","password":"Synthetic-New-Password1!"}`, id, raw)
	if w := f.request("POST", "/api/v1/auth/recovery/confirm", body, "", ""); w.Code != 204 {
		t.Fatal("reset", w.Code, w.Body.String())
	}
	if w := f.request("GET", "/api/v1/account/methods", "", "", f.token); w.Code != 401 {
		t.Fatal("old session survived reset", w.Code)
	}
	if w := f.request("POST", "/api/v1/auth/recovery/confirm", body, "", ""); w.Code != 400 {
		t.Fatal("reset replay accepted", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE user_id=$1`, f.owner) != 0 || identityCount(t, f.db, `SELECT COUNT(*) FROM platform_admin_grants WHERE user_id=$1`, f.owner) != 0 {
		t.Fatal("reset created authority")
	}
}

func TestPlatformIdentityPurposeExpiryAndNoEmailMerge(t *testing.T) {
	f := newIdentityFixture(t)
	w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"shared-contact@example.invalid"}`, "", f.token)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	id := identityRequestID(t, w.Body.String())
	raw := f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
	wrongPurpose := fmt.Sprintf(`{"id":"%s","proof":"%s","password":"A-strong-fixture-password-2026!"}`, id, raw)
	if w = f.request("POST", "/api/v1/auth/recovery/confirm", wrongPurpose, "", ""); w.Code != 400 {
		t.Fatal("contact proof recovered password", w.Code)
	}
	path := "/api/v1/account/contact-proofs/" + id.String() + "/confirm"
	proof := fmt.Sprintf(`{"proof":"%s"}`, raw)
	if _, e := f.db.Exec(`UPDATE identity_proofs SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if w = f.request("POST", path, proof, "", f.token); w.Code != 400 {
		t.Fatal("expired proof accepted", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1`, f.owner) != 0 {
		t.Fatal("expired proof created contact")
	}
	if _, e := f.db.Exec(`INSERT INTO identity_verified_contacts(user_id,contact,verified_at) VALUES($1,'shared-contact@example.invalid',NOW())`, f.owner); e != nil {
		t.Fatal(e)
	}
	other := f.session(t, f.other, "password")
	w = f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"shared-contact@example.invalid"}`, "", other)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	id = identityRequestID(t, w.Body.String())
	raw = f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
	if w = f.request("POST", "/api/v1/account/contact-proofs/"+id.String()+"/confirm", fmt.Sprintf(`{"proof":"%s"}`, raw), "", other); w.Code != 409 {
		t.Fatal("contact collision did not refuse merge", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1`, f.other) != 0 {
		t.Fatal("email collision attached authority")
	}
}

func TestPlatformIdentityAcceptedSMTPWithFailedOutcomeAuditStaysUncertain(t *testing.T) {
	f := newIdentityFixture(t)
	w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"outcome-unknown@example.invalid"}`, "", f.token)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	id := identityRequestID(t, w.Body.String())
	var delivery uuid.UUID
	if e := f.db.QueryRow(`SELECT id FROM identity_proof_deliveries WHERE proof_id=$1`, id).Scan(&delivery); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(grantHTTPAuditFailureDDL(f.d, "identity.delivery_completed")); e != nil {
		t.Fatal(e)
	}
	mail := &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}}
	outcome, e := f.identity.Deliver(context.Background(), delivery, mail)
	if e == nil || outcome.State != "uncertain" || outcome.Reason != "outcome_persistence_failed" {
		t.Fatal("SMTP outcome overclaimed", outcome, e)
	}
	if _, e = f.identity.Deliver(context.Background(), delivery, mail); e == nil {
		t.Fatal("unknown SMTP outcome automatically retried")
	}
	if mail.calls != 1 {
		t.Fatal("SMTP was called more than once", mail.calls)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proof_deliveries WHERE id=$1 AND state='dispatched'`, delivery) != 1 {
		t.Fatal("failed outcome audit escaped rollback")
	}
	if _, e = f.db.Exec(`DROP TRIGGER fail_grant_http_audit ON audit_log; DROP FUNCTION grant_http_audit_failure()`); e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE identity_proof_deliveries SET dispatched_at=NOW()-INTERVAL '2 minutes' WHERE id=$1`, delivery); e != nil {
		t.Fatal(e)
	}
	if e = f.identity.MarkDeliveryUncertain(context.Background(), delivery); e != nil {
		t.Fatal(e)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proof_deliveries WHERE id=$1 AND state='uncertain' AND octet_length(ciphertext)=0`, delivery) != 1 {
		t.Fatal("recovery failed to clear uncertain proof")
	}
}

func TestPlatformIdentityMethodRemovalRequiresRemainingMethodProof(t *testing.T) {
	f := newIdentityFixture(t)
	hash, e := auth.HashPassword("Synthetic-Password1!")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, hash, f.owner); e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`INSERT INTO social_identities(provider,subject,user_id,email) VALUES('google',$1,$2,'method@example.invalid')`, uuid.NewString(), f.owner); e != nil {
		t.Fatal(e)
	}
	// A recent unknown/legacy session and authentication through the method being
	// removed cannot establish possession of a remaining method.
	if w := f.request("DELETE", "/api/v1/account/methods/password", "", "", f.token); w.Code != 403 {
		t.Fatal("unknown method proof accepted", w.Code)
	}
	password := f.session(t, f.owner, "password")
	if w := f.request("DELETE", "/api/v1/account/methods/password", "", "", password); w.Code != 403 {
		t.Fatal("same method proof accepted", w.Code)
	}
	google := f.session(t, f.owner, "google")
	if w := f.request("DELETE", "/api/v1/account/methods/password", "", "", google); w.Code != 204 {
		t.Fatal("remaining method proof refused", w.Code, w.Body.String())
	}
	if w := f.request("DELETE", "/api/v1/account/methods/google", "", "", google); w.Code != 409 {
		t.Fatal("last usable method removed", w.Code)
	}
	if w := f.request("GET", "/api/v1/account/methods", "", "", password); w.Code != 401 {
		t.Fatal("other session survived method removal", w.Code)
	}
}

func TestPlatformIdentityOffboardingEpochScopeAndAuditRollback(t *testing.T) {
	f := newIdentityFixture(t)
	org := f.organization(t)
	orgs := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), f.audit)
	if _, e := orgs.AddMember(org, f.owner, f.other, "editor", ""); e != nil {
		t.Fatal(e)
	}
	if e := orgs.AssignProjectWithAudit(org, f.owner, f.project, ""); e != nil {
		t.Fatal(e)
	}
	// A grant owned by the departing member belongs only to this organization.
	if _, e := f.db.Exec(`INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes) VALUES($1,'offboard-key',$2,$3,$4,ARRAY['read'])`, uuid.New(), uuid.NewString(), f.other, f.project); e != nil {
		t.Fatal(e)
	}
	token := f.session(t, f.other, "password")
	principal := f.principal(t, f.token)
	preview, e := f.identity.PreviewOffboarding(context.Background(), principal, org, f.other)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(grantHTTPAuditFailureDDL(f.d, "identity.member_offboarded")); e != nil {
		t.Fatal(e)
	}
	if _, e = f.identity.OffboardWithPreview(context.Background(), principal, org, f.other, preview.Epoch, "failure", preview.PreviewID); e == nil {
		t.Fatal("offboard ignored required audit")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org, f.other) != 1 || identityCount(t, f.db, `SELECT COUNT(*) FROM api_keys WHERE project_id=$1 AND user_id=$2`, f.project, f.other) != 1 {
		t.Fatal("offboard escaped rollback")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_offboarding_previews WHERE id=$1 AND consumed_at IS NULL`, preview.PreviewID) != 1 {
		t.Fatal("failed offboard consumed preview")
	}
	if _, e = f.db.Exec(`DROP TRIGGER fail_grant_http_audit ON audit_log; DROP FUNCTION grant_http_audit_failure()`); e != nil {
		t.Fatal(e)
	}
	receipt, e := f.identity.OffboardWithPreview(context.Background(), principal, org, f.other, preview.Epoch, "success", preview.PreviewID)
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.identity.OffboardWithPreview(context.Background(), principal, org, f.other, preview.Epoch, "success", preview.PreviewID)
	if e != nil || again.ID != receipt.ID {
		t.Fatal("offboard retry lost receipt", e)
	}
	if w := f.request("GET", "/api/v1/account/methods", "", "", token); w.Code != 200 {
		t.Fatal("organization offboard revoked global session", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND owner_id=$2 AND organization_id IS NULL`, f.foreign, f.other) != 1 {
		t.Fatal("unrelated personal scope changed")
	}
	if _, e = orgs.AddMember(org, f.owner, f.other, "editor", ""); e != nil {
		t.Fatal(e)
	}
	var epoch int64
	if e = f.db.QueryRow(`SELECT epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2`, org, f.other).Scan(&epoch); e != nil || epoch <= receipt.Epoch {
		t.Fatal("rejoin reused old epoch", e)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM api_keys WHERE project_id=$1 AND user_id=$2`, f.project, f.other) != 0 {
		t.Fatal("rejoin revived old grants")
	}
}

func TestPlatformIdentityInvitationCurrentAuthorityAndOneTimeAccept(t *testing.T) {
	f := newIdentityFixture(t)
	org := f.organization(t)
	other := f.session(t, f.other, "password")
	if _, e := f.db.Exec(`INSERT INTO identity_verified_contacts(user_id,contact,verified_at) VALUES($1,'invite-target@example.invalid',NOW())`, f.other); e != nil {
		t.Fatal(e)
	}
	body := `{"contact":"invite-target@example.invalid","role":"editor"}`
	w := f.request("POST", "/api/v1/organizations/"+org.String()+"/invitations", body, "", f.token)
	if w.Code != 201 {
		t.Fatal("invitation create", w.Code, w.Body.String())
	}
	var response struct {
		Invitation identity.Invitation `json:"invitation"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	id := response.Invitation.ID
	var lifetime int
	if e := f.db.QueryRow(`SELECT EXTRACT(EPOCH FROM (expires_at-created_at))::integer FROM identity_proofs WHERE id=$1`, id).Scan(&lifetime); e != nil || lifetime != 86400 {
		t.Fatal("invitation lifetime", lifetime, e)
	}
	raw := f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
	path := "/api/v1/invitations/" + id.String() + "/accept"
	proof := fmt.Sprintf(`{"proof":"%s"}`, raw)
	if w = f.request("POST", path, proof, "", f.token); w.Code != 400 {
		t.Fatal("wrong identity accepted email invite", w.Code)
	}
	if w = f.request("POST", path, proof, "", other); w.Code != 204 {
		t.Fatal("verified invite acceptance", w.Code, w.Body.String())
	}
	if w = f.request("POST", path, proof, "", other); w.Code != 400 {
		t.Fatal("invitation replay", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2 AND role='editor'`, org, f.other) != 1 {
		t.Fatal("invitation membership missing")
	}
	// An invitation whose parent session is revoked cannot confer membership.
	org2 := f.organization(t)
	w = f.request("POST", "/api/v1/organizations/"+org2.String()+"/invitations", body, "", f.token)
	if w.Code != 201 {
		t.Fatal(w.Code)
	}
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	id = response.Invitation.ID
	raw = f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
	principal := f.principal(t, f.token)
	if e := f.sessions.Revoke(context.Background(), f.owner, principal.SessionID, principal.SessionID, ""); e != nil {
		t.Fatal(e)
	}
	if w = f.request("POST", "/api/v1/invitations/"+id.String()+"/accept", fmt.Sprintf(`{"proof":"%s"}`, raw), "", other); w.Code != 400 {
		t.Fatal("revoked inviter session accepted", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org2, f.other) != 0 {
		t.Fatal("revoked invitation changed membership")
	}
}

func TestPlatformIdentityOffboardPreviewPinsMintedInventoryAndOwnership(t *testing.T) {
	f := newIdentityFixture(t)
	org := f.organization(t)
	orgs := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), f.audit)
	if _, e := orgs.AddMember(org, f.owner, f.other, "editor", ""); e != nil {
		t.Fatal(e)
	}
	if e := orgs.AssignProjectWithAudit(org, f.owner, f.project, ""); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE projects SET owner_id=$1 WHERE id=$2`, f.other, f.project); e != nil {
		t.Fatal(e)
	}
	key := uuid.New()
	if _, e := f.db.Exec(`INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes) VALUES($1,'preview-mint',$2,$3,$4,ARRAY['read'])`, key, uuid.NewString(), f.other, f.project); e != nil {
		t.Fatal(e)
	}
	p := f.principal(t, f.token)
	cs, e := crypto.NewService(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	v := vault.New(f.db, cs, func(ctx context.Context, tx *sql.Tx, principal policy.Principal, action policy.Action, resource policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d}}).Authorize(ctx, principal, action, resource)
	}, f.audit)
	if e = v.Enroll(context.Background(), p, f.project); e != nil {
		t.Fatal(e)
	}
	var ownedSecret uuid.UUID
	if e = f.db.QueryRow(`SELECT secret_id FROM vault_entries WHERE project_id=$1 AND secret_key='DB_URL'`, f.project).Scan(&ownedSecret); e != nil {
		t.Fatal(e)
	}
	if _, e = v.UpdateLifecycle(context.Background(), p, f.project, ownedSecret, vault.LifecycleChange{ResponsibleUserID: &f.other, ExpectedRevision: 0}); e != nil {
		t.Fatal(e)
	}
	preview, e := f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil {
		t.Fatal(e)
	}
	if preview.PreviewID == uuid.Nil || time.Until(preview.PreviewExpires) > 5*time.Minute || len(preview.OwnedProjects) != 1 || preview.OwnedProjects[0].ProjectID != f.project || preview.OwnedProjects[0].Consequence == "" {
		t.Fatal("ownership/revision preview missing", preview)
	}
	if len(preview.OwnedCredentials) != 1 || preview.OwnedCredentials[0].SecretID != ownedSecret || preview.OwnedCredentials[0].Environment != "alpha" || preview.OwnedCredentials[0].Key != "DB_URL" || preview.OwnedCredentials[0].LifecycleRevision != 1 {
		t.Fatal("credential ownership preview missing", preview.OwnedCredentials)
	}
	path := "/api/v1/organizations/" + org.String() + "/members/" + f.other.String() + "/offboarding"
	if w := f.requestIdempotent("POST", path, fmt.Sprintf(`{"expected_authority_epoch":%d}`, preview.Epoch), "missing-preview", f.token); w.Code != 400 {
		t.Fatal("production accepted epoch-only offboard", w.Code)
	}
	ls := service.NewLeaseService(f.db, f.d, f.audit)
	ls.EnableSessions(f.sessions)
	if _, e = ls.CreateLeaseAuthorized(context.Background(), policy.Principal{Kind: policy.APIKey, SubjectID: key, ActorID: f.other}, f.project, "alpha", []string{"DB_URL"}, time.Minute, ""); e != nil {
		t.Fatal("real mint", e)
	}
	var epoch int64
	if e = f.db.QueryRow(`SELECT epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2`, org, f.other).Scan(&epoch); e != nil || epoch != preview.Epoch {
		t.Fatal("fixture must isolate resource revision from membership epoch", epoch, e)
	}
	body := fmt.Sprintf(`{"expected_authority_epoch":%d,"preview_id":"%s"}`, preview.Epoch, preview.PreviewID)
	if w := f.requestIdempotent("POST", path, body, "stale-preview", f.token); w.Code != 409 {
		t.Fatal("mint after preview accepted", w.Code, w.Body.String())
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org, f.other) != 1 || identityCount(t, f.db, `SELECT COUNT(*) FROM identity_offboarding_previews WHERE id=$1 AND consumed_at IS NULL`, preview.PreviewID) != 1 {
		t.Fatal("stale preview mutated authority")
	}
	preview, e = f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil || preview.Leases != 1 {
		t.Fatal("fresh inventory", preview, e)
	}
	if _, e = v.UpdateLifecycle(context.Background(), p, f.project, ownedSecret, vault.LifecycleChange{ResponsibleUserID: &f.other, ExpectedRevision: 1, Provenance: "Synthetic lifecycle metadata change"}); e != nil {
		t.Fatal(e)
	}
	if _, e = f.identity.OffboardWithPreview(context.Background(), p, org, f.other, preview.Epoch, "stale-lifecycle", preview.PreviewID); e != identity.ErrConflict {
		t.Fatal("lifecycle revision did not stale preview", e)
	}
	preview, e = f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil || preview.OwnedCredentials[0].LifecycleRevision != 2 {
		t.Fatal("fresh lifecycle preview", preview, e)
	}
	if _, e = f.db.Exec(`UPDATE identity_offboarding_previews SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, preview.PreviewID); e != nil {
		t.Fatal(e)
	}
	body = fmt.Sprintf(`{"expected_authority_epoch":%d,"preview_id":"%s"}`, preview.Epoch, preview.PreviewID)
	if w := f.requestIdempotent("POST", path, body, "expired-preview", f.token); w.Code != 409 {
		t.Fatal("expired preview accepted", w.Code)
	}
	preview, e = f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil {
		t.Fatal(e)
	}
	org2 := f.organization(t)
	if _, e = orgs.AddMember(org2, f.owner, f.other, "editor", ""); e != nil {
		t.Fatal(e)
	}
	if _, e = f.identity.OffboardWithPreview(context.Background(), p, org2, f.other, preview.Epoch, "cross-org", preview.PreviewID); e != identity.ErrConflict {
		t.Fatal("preview crossed organization", e)
	}
	body = fmt.Sprintf(`{"expected_authority_epoch":%d,"preview_id":"%s"}`, preview.Epoch, preview.PreviewID)
	w := f.requestIdempotent("POST", path, body, "fresh-preview", f.token)
	if w.Code != 200 {
		t.Fatal("fresh preview rejected", w.Code, w.Body.String())
	}
	if w = f.requestIdempotent("POST", path, body, "fresh-preview", f.token); w.Code != 200 {
		t.Fatal("exact offboard retry lost consumed preview receipt", w.Code)
	}
	if w = f.requestIdempotent("POST", path, body, "different-request", f.token); w.Code != 403 {
		t.Fatal("consumed preview replay allowed", w.Code)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND owner_id=$2 AND organization_id=$3`, f.project, f.other, org) != 1 {
		t.Fatal("offboarding silently transferred project ownership")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM vault_lifecycle WHERE secret_id=$1 AND responsible_user_id=$2 AND revision=2`, ownedSecret, f.other) != 1 {
		t.Fatal("offboarding silently reassigned credential responsibility")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org2, f.other) != 1 {
		t.Fatal("offboarding changed unrelated organization")
	}
}

func TestPlatformIdentityOffboardPreviewIncludesInheritedToolAuthority(t *testing.T) {
	f := newIdentityFixture(t)
	org := f.organization(t)
	orgs := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), f.audit)
	if _, e := orgs.AddMember(org, f.owner, f.other, "promoter", ""); e != nil {
		t.Fatal(e)
	}
	if e := orgs.AssignProjectWithAudit(org, f.owner, f.project, ""); e != nil {
		t.Fatal(e)
	}
	p := f.principal(t, f.token)
	artifact, profile, pkg, connection, binding, workload, grant := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	digest := strings.Repeat("a", 64)
	exec := func(query string, values ...any) {
		t.Helper()
		if _, e := f.db.Exec(query, values...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO tool_artifacts(id,project_id,name,digest,source,created_by) VALUES($1,$2,'fixture',$3,'synthetic immutable tool source',$4)`, artifact, f.project, digest, f.owner)
	exec(`INSERT INTO tool_profiles(id,project_id,artifact_id,digest,manifest,created_by,approved_by,approver_epoch,approved_at,approved_until) VALUES($1,$2,$3,$4,'{}',$5,$6,1,NOW(),NOW()+INTERVAL '1 hour')`, profile, f.project, artifact, digest, f.owner, f.other)
	exec(`INSERT INTO tool_profile_packages(id,profile_id,harness,version,format,digest,content,compatibility,created_by,approved_by,approver_epoch,approved_at,approved_until) VALUES($1,$2,'codex','fixture','fixture',$3,'{}','{}',$4,$5,1,NOW(),NOW()+INTERVAL '1 hour')`, pkg, profile, digest, f.other, f.owner)
	exec(`INSERT INTO tool_connections(id,project_id,app_id,installation_id,ciphertext,nonce,created_by) VALUES($1,$2,1,1,'synthetic','synthetic',$3)`, connection, f.project, f.owner)
	exec(`INSERT INTO tool_bindings(id,project_id,connection_id,environment_id,repository_id,owner_name,repository_name,reference) SELECT $1,$2,$3,id,1,'fixture','fixture','main' FROM environments WHERE project_id=$2 AND name='alpha'`, binding, f.project, connection)
	exec(`INSERT INTO tool_workloads(id,project_id,certificate_sha256,image_digest,created_by) VALUES($1,$2,$3,'fixture',$4)`, workload, f.project, digest, f.owner)
	before, e := f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil || before.Runs != 0 {
		t.Fatal("empty inherited preview", before, e)
	}
	exec(`INSERT INTO tool_grants(id,project_id,profile_id,package_id,profile_digest,package_digest,binding_id,workload_id,actor_id,client_id,issued_by,membership_epoch,issuer_epoch,expires_at) VALUES($1,$2,$3,$4,$5,$5,$6,$7,$8,'keepsave-codex-linux-v1',$8,1,1,NOW()+INTERVAL '1 hour')`, grant, f.project, profile, pkg, digest, binding, workload, f.owner)
	for i := 0; i < 2; i++ {
		exec(`INSERT INTO tool_runs(id,project_id,grant_id,actor_id,session_id,client_id,parent_kind,authority_expires_at,state,reference,expires_at,request_key,request_digest) VALUES($1,$2,$3,$4,$5,'keepsave-codex-linux-v1','human',NOW()+INTERVAL '1 hour','active','main',NOW()+INTERVAL '10 minutes',$6,$7)`, uuid.New(), f.project, grant, f.owner, p.SessionID, uuid.NewString(), digest)
	}
	if _, e = f.identity.OffboardWithPreview(context.Background(), p, org, f.other, before.Epoch, "stale-inherited-mint", before.PreviewID); e != identity.ErrConflict {
		t.Fatal("inherited mint failed to invalidate preview", e)
	}
	preview, e := f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil || preview.Runs != 2 {
		t.Fatal("inherited active run count", preview, e)
	}
	if _, e = f.identity.OffboardWithPreview(context.Background(), p, org, f.other, preview.Epoch, "missing-cascade", preview.PreviewID); e != identity.ErrUnavailable {
		t.Fatal("inherited authority allowed partial offboard", e)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org, f.other) != 1 {
		t.Fatal("missing cascade removed member")
	}
	f.identity.EnableCascade(runs.New(f.db, nil, nil, f.audit, runs.Flags{}))
	exec(`UPDATE tool_profiles SET revoked_at=NOW() WHERE id=$1`, profile)
	if _, e = f.identity.OffboardWithPreview(context.Background(), p, org, f.other, preview.Epoch, "stale-approval", preview.PreviewID); e != identity.ErrConflict {
		t.Fatal("approval revocation failed to invalidate preview", e)
	}
	preview, e = f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil {
		t.Fatal(e)
	}
	receipt, e := f.identity.OffboardWithPreview(context.Background(), p, org, f.other, preview.Epoch, "inherited-authority", preview.PreviewID)
	if e != nil || receipt.Runs != 2 {
		t.Fatal("receipt must count active runs, not grants", receipt, e)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM tool_runs WHERE grant_id=$1 AND state='revoked'`, grant) != 2 {
		t.Fatal("inherited child runs remained active")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM tool_grants WHERE id=$1 AND revoked_at IS NOT NULL`, grant) != 1 {
		t.Fatal("inherited grant remained active")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM session_tokens WHERE id=$1 AND NOT revoked`, p.SessionID) != 1 {
		t.Fatal("offboard invalidated unrelated actor session")
	}
}

type identityQueueProbe struct {
	syntheticProofMail
	t        *testing.T
	db       *sql.DB
	delivery uuid.UUID
}

func (s *identityQueueProbe) Send(ctx context.Context, to, subject, body string) identity.DeliveryOutcome {
	s.t.Helper()
	var queueState, proofState string
	if e := s.db.QueryRow(`SELECT j.status,d.state FROM outbox_jobs j JOIN identity_proof_deliveries d ON j.payload->>'delivery_id'=d.id::text WHERE d.id=$1 AND j.kind='identity.proof_delivery'`, s.delivery).Scan(&queueState, &proofState); e != nil || queueState != "dispatched" || proofState != "dispatched" {
		s.t.Fatal("SMTP preceded persisted dispatch", queueState, proofState, e)
	}
	return s.syntheticProofMail.Send(ctx, to, subject, body)
}
func TestPlatformIdentityWorkerDispatchRecoveryAndPayloadCleanup(t *testing.T) {
	t.Run("dispatch commits before SMTP and ack after result", func(t *testing.T) {
		f := newIdentityFixture(t)
		w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"worker@example.invalid"}`, "", f.token)
		if w.Code != 202 {
			t.Fatal(w.Code)
		}
		id := identityRequestID(t, w.Body.String())
		var delivery uuid.UUID
		if e := f.db.QueryRow(`SELECT id FROM identity_proof_deliveries WHERE proof_id=$1`, id).Scan(&delivery); e != nil {
			t.Fatal(e)
		}
		mail := &identityQueueProbe{syntheticProofMail: syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}}, t: t, db: f.db, delivery: delivery}
		if worked, e := f.identity.StepDelivery(context.Background(), uuid.New(), mail); e != nil || !worked {
			t.Fatal("worker failed", worked, e)
		}
		if identityCount(t, f.db, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='identity.proof_delivery' AND status='done'`) != 1 || identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proof_deliveries WHERE id=$1 AND state='sent' AND octet_length(ciphertext)=0`, delivery) != 1 {
			t.Fatal("result not durably acknowledged")
		}
		if worked, e := f.identity.StepDelivery(context.Background(), uuid.New(), mail); e != nil || worked || mail.calls != 1 {
			t.Fatal("completed message replayed", worked, e, mail.calls)
		}
	})
	for _, proofDispatched := range []bool{false, true} {
		t.Run(fmt.Sprintf("crash proofDispatched=%v", proofDispatched), func(t *testing.T) {
			f := newIdentityFixture(t)
			w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"crash-worker@example.invalid"}`, "", f.token)
			if w.Code != 202 {
				t.Fatal(w.Code)
			}
			id := identityRequestID(t, w.Body.String())
			var delivery uuid.UUID
			if e := f.db.QueryRow(`SELECT id FROM identity_proof_deliveries WHERE proof_id=$1`, id).Scan(&delivery); e != nil {
				t.Fatal(e)
			}
			queue := jobs.Queue{DB: f.db}
			job, e := queue.ClaimKinds(context.Background(), uuid.New(), time.Minute, []string{"identity.proof_delivery"})
			if e != nil {
				t.Fatal(e)
			}
			if e = queue.Dispatch(context.Background(), *job); e != nil {
				t.Fatal(e)
			}
			if _, e = f.db.Exec(`UPDATE outbox_jobs SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, job.ID); e != nil {
				t.Fatal(e)
			}
			if proofDispatched {
				if _, e = f.db.Exec(`UPDATE identity_proof_deliveries SET state='dispatched',dispatched_at=NOW()-INTERVAL '2 minutes' WHERE id=$1`, delivery); e != nil {
					t.Fatal(e)
				}
			}
			mail := &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}}
			if worked, e := f.identity.StepDelivery(context.Background(), uuid.New(), mail); e != nil || worked || mail.calls != 0 {
				t.Fatal("crashed dispatch replayed", worked, e, mail.calls)
			}
			if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proof_deliveries WHERE id=$1 AND state='uncertain' AND octet_length(ciphertext)=0`, delivery) != 1 || identityCount(t, f.db, `SELECT COUNT(*) FROM outbox_jobs WHERE id=$1 AND status='uncertain'`, job.ID) != 1 {
				t.Fatal("crash was not settled conservatively")
			}
			if w = f.request("POST", "/api/v1/account/proofs/"+id.String()+"/resend", "", "", f.token); w.Code != 202 {
				t.Fatal("manual fresh proof unavailable", w.Code)
			}
		})
	}
	t.Run("revoked payload cancelled without SMTP", func(t *testing.T) {
		f := newIdentityFixture(t)
		w := f.request("POST", "/api/v1/account/contact-proofs", `{"contact":"revoked-worker@example.invalid"}`, "", f.token)
		if w.Code != 202 {
			t.Fatal(w.Code)
		}
		id := identityRequestID(t, w.Body.String())
		if _, e := f.db.Exec(`UPDATE identity_proofs SET revoked=TRUE WHERE id=$1`, id); e != nil {
			t.Fatal(e)
		}
		mail := &syntheticProofMail{}
		if worked, e := f.identity.StepDelivery(context.Background(), uuid.New(), mail); e != nil || !worked || mail.calls != 0 {
			t.Fatal("revoked proof sent", worked, e, mail.calls)
		}
		if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proof_deliveries WHERE proof_id=$1 AND state='cancelled' AND octet_length(ciphertext)=0`, id) != 1 {
			t.Fatal("revoked payload retained")
		}
	})
}

type identityPausingGuard struct {
	authority.Guard
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *identityPausingGuard) LockSubjects(ctx context.Context, tx *sql.Tx, ids []uuid.UUID, exclusive bool) error {
	if e := g.Guard.LockSubjects(ctx, tx, ids, exclusive); e != nil {
		return e
	}
	if exclusive {
		g.once.Do(func() { close(g.entered); <-g.release })
	}
	return nil
}
func TestPlatformIdentityOffboardingSerializesLegacyGrantMint(t *testing.T) {
	f := newIdentityFixture(t)
	org := f.organization(t)
	orgs := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), f.audit)
	if _, e := orgs.AddMember(org, f.owner, f.other, "editor", ""); e != nil {
		t.Fatal(e)
	}
	if e := orgs.AssignProjectWithAudit(org, f.owner, f.project, ""); e != nil {
		t.Fatal(e)
	}
	key := uuid.New()
	if _, e := f.db.Exec(`INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes) VALUES($1,'mint-race',$2,$3,$4,ARRAY['read'])`, key, uuid.NewString(), f.other, f.project); e != nil {
		t.Fatal(e)
	}
	p := f.principal(t, f.token)
	preview, e := f.identity.PreviewOffboarding(context.Background(), p, org, f.other)
	if e != nil {
		t.Fatal(e)
	}
	guard := &identityPausingGuard{Guard: authority.Guard{DB: f.db, Dialect: f.d}, entered: make(chan struct{}), release: make(chan struct{})}
	paused := identity.New(f.db, identity.Config{Enabled: true, PostgreSQL: true}, f.audit, nil, guard)
	offboard := make(chan error, 1)
	go func() {
		_, err := paused.OffboardWithPreview(context.Background(), p, org, f.other, preview.Epoch, "mint-race", preview.PreviewID)
		offboard <- err
	}()
	select {
	case <-guard.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("offboard did not acquire barrier")
	}
	ls := service.NewLeaseService(f.db, f.d, f.audit)
	ls.EnableSessions(f.sessions)
	mint := make(chan error, 1)
	go func() {
		_, err := ls.CreateLeaseAuthorized(context.Background(), policy.Principal{Kind: policy.APIKey, SubjectID: key, ActorID: f.other}, f.project, "alpha", []string{"DB_URL"}, time.Minute, "")
		mint <- err
	}()
	select {
	case err := <-mint:
		close(guard.release)
		t.Fatalf("mint bypassed held authority barrier: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(guard.release)
	if err := <-offboard; err != nil {
		t.Fatal(err)
	}
	if err := <-mint; err == nil {
		t.Fatal("mint succeeded after committed offboard")
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM secret_leases WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("offboard/mint race left active delegation")
	}
}
