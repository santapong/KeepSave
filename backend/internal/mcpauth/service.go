package mcpauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"net/url"
	"time"
)

type Service struct {
	db       *sql.DB
	audit    *repository.AuditRepository
	sessions SessionAuthority
	cfg      Config
}

func New(db *sql.DB, d repository.Dialect, audit *repository.AuditRepository, sessions SessionAuthority, cfg Config) (*Service, error) {
	if db == nil || d == nil || d.DBType() != repository.DBTypePostgres || audit == nil || sessions == nil {
		return nil, ErrUnavailable
	}
	if e := cfg.Validate(); e != nil {
		return nil, e
	}
	return &Service{db: db, audit: audit, sessions: sessions, cfg: cfg}, nil
}
func (s *Service) Config() Config {
	c := s.cfg
	c.AllowedOrigins = append([]string(nil), c.AllowedOrigins...)
	return c
}
func (s *Service) AuthorizationMetadata() map[string]any {
	return map[string]any{"issuer": s.cfg.Issuer, "authorization_endpoint": s.cfg.AppURL + "/oauth/mcp/authorize", "token_endpoint": s.cfg.AppURL + "/oauth/mcp/token", "revocation_endpoint": s.cfg.AppURL + "/oauth/mcp/revoke", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{Scope, "offline_access"}, "authorization_response_iss_parameter_supported": true}
}
func (s *Service) ProtectedMetadata() map[string]any {
	return map[string]any{"resource": s.cfg.Resource, "authorization_servers": []string{s.cfg.Issuer}, "scopes_supported": []string{Scope}, "bearer_methods_supported": []string{"header"}}
}
func (s *Service) auditTx(tx *sql.Tx, user uuid.UUID, action string, details models.JSONMap) error {
	return s.audit.CreateTx(tx, &user, nil, action, "", details, "")
}
func (s *Service) begin(ctx context.Context) (*sql.Tx, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s';SET LOCAL statement_timeout='10s'`); e != nil {
		tx.Rollback()
		return nil, ErrUnavailable
	}
	return tx, nil
}

// Lock order: user authority barrier, parent session, consent, family, token.
func (s *Service) parent(ctx context.Context, tx *sql.Tx, user, sid uuid.UUID, recent bool) (time.Time, error) {
	var e error
	if recent {
		e = s.sessions.RequireRecentTx(ctx, tx, user, sid)
	} else {
		e = s.sessions.RequireActiveTx(ctx, tx, user, sid)
	}
	if e != nil {
		return time.Time{}, ErrDenied
	}
	var expiry time.Time
	e = tx.QueryRowContext(ctx, `SELECT expires_at FROM session_tokens WHERE id=$1 AND user_id=$2`, sid, user).Scan(&expiry)
	if e != nil {
		return time.Time{}, ErrUnavailable
	}
	return expiry, nil
}
func (s *Service) client(ctx context.Context, tx *sql.Tx, id, redirect string) (string, error) {
	var harness, uri string
	var enabled bool
	var e error
	if tx != nil {
		e = tx.QueryRowContext(ctx, `SELECT harness,redirect_uri,enabled FROM mcp_oauth_clients WHERE client_id=$1 FOR SHARE`, id).Scan(&harness, &uri, &enabled)
	} else {
		e = s.db.QueryRowContext(ctx, `SELECT harness,redirect_uri,enabled FROM mcp_oauth_clients WHERE client_id=$1`, id).Scan(&harness, &uri, &enabled)
	}
	if errors.Is(e, sql.ErrNoRows) {
		return "", ErrInvalid
	}
	if e != nil {
		return "", ErrUnavailable
	}
	if !enabled || redirect != "" && uri != redirect {
		return "", ErrInvalid
	}
	return harness, nil
}
func (s *Service) BeginAuthorization(ctx context.Context, values url.Values) (AuthorizationRequest, error) {
	// Reject ambiguous security parameters rather than inheriting net/url's first-value rule.
	for _, key := range []string{"client_id", "redirect_uri", "response_type", "scope", "state", "code_challenge", "code_challenge_method", "resource"} {
		if len(values[key]) != 1 {
			return AuthorizationRequest{}, ErrInvalid
		}
	}
	if values.Get("response_type") != "code" || values.Get("resource") != s.cfg.Resource || values.Get("code_challenge_method") != "S256" || !validChallenge(values.Get("code_challenge")) || len(values.Get("state")) < 16 || len(values.Get("state")) > 1024 {
		return AuthorizationRequest{}, ErrInvalid
	}
	scope, e := canonicalScope(values.Get("scope"))
	if e != nil {
		return AuthorizationRequest{}, e
	}
	harness, e := s.client(ctx, nil, values.Get("client_id"), values.Get("redirect_uri"))
	if e != nil {
		return AuthorizationRequest{}, e
	}
	req := AuthorizationRequest{ID: uuid.New(), ClientID: values.Get("client_id"), Harness: harness, Resource: s.cfg.Resource, RedirectURI: values.Get("redirect_uri"), Scope: scope, State: values.Get("state"), Challenge: values.Get("code_challenge"), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}
	_, e = s.db.ExecContext(ctx, `INSERT INTO mcp_oauth_requests(id,client_id,resource,redirect_uri,scope,state,challenge,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, req.ID, req.ClientID, req.Resource, req.RedirectURI, req.Scope, req.State, req.Challenge, req.ExpiresAt)
	if e != nil {
		return AuthorizationRequest{}, ErrUnavailable
	}
	return req, nil
}
func (s *Service) loadRequest(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id uuid.UUID, lock bool) (AuthorizationRequest, error) {
	var r AuthorizationRequest
	var consumed sql.NullTime
	query := `SELECT r.id,r.client_id,c.harness,r.resource,r.redirect_uri,r.scope,r.state,r.challenge,r.expires_at,r.consumed_at FROM mcp_oauth_requests r JOIN mcp_oauth_clients c ON c.client_id=r.client_id WHERE r.id=$1 AND c.enabled=TRUE`
	if lock {
		query += ` FOR UPDATE OF r`
	}
	e := q.QueryRowContext(ctx, query, id).Scan(&r.ID, &r.ClientID, &r.Harness, &r.Resource, &r.RedirectURI, &r.Scope, &r.State, &r.Challenge, &r.ExpiresAt, &consumed)
	if errors.Is(e, sql.ErrNoRows) || consumed.Valid || !r.ExpiresAt.After(time.Now()) || r.Resource != s.cfg.Resource {
		return r, ErrInvalid
	}
	if e != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s *Service) Preview(ctx context.Context, user, sid, id uuid.UUID) (AuthorizationRequest, error) {
	tx, e := s.begin(ctx)
	if e != nil {
		return AuthorizationRequest{}, ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, user, sid, false); e != nil {
		return AuthorizationRequest{}, e
	}
	return s.loadRequest(ctx, tx, id, false)
}
func (s *Service) Decide(ctx context.Context, user, sid, id uuid.UUID, approve bool) (string, error) {
	tx, e := s.begin(ctx)
	if e != nil {
		return "", ErrUnavailable
	}
	defer tx.Rollback()
	parentExpiry, e := s.parent(ctx, tx, user, sid, true)
	if e != nil {
		return "", e
	}
	r, e := s.loadRequest(ctx, tx, id, true)
	if e != nil {
		return "", e
	}
	if _, e = s.client(ctx, tx, r.ClientID, r.RedirectURI); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_requests SET consumed_at=NOW() WHERE id=$1`, id); e != nil {
		return "", ErrUnavailable
	}
	redirect, _ := url.Parse(r.RedirectURI)
	query := redirect.Query()
	query.Set("state", r.State)
	query.Set("iss", s.cfg.Issuer)
	action := "mcp.consent_denied"
	details := models.JSONMap{"client_id": r.ClientID, "request_id": id.String(), "resource": s.cfg.Resource}
	if approve {
		action = "mcp.consent_approved"
		consent := uuid.New()
		expires := time.Now().UTC().Add(8 * time.Hour)
		if parentExpiry.Before(expires) {
			expires = parentExpiry
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO mcp_oauth_consents(id,user_id,session_id,client_id,resource,scope,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, consent, user, sid, r.ClientID, s.cfg.Resource, r.Scope, expires)
		if e != nil {
			return "", ErrUnavailable
		}
		code, e := token()
		if e != nil {
			return "", ErrUnavailable
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO mcp_oauth_codes(code_hash,consent_id,redirect_uri,challenge,expires_at) VALUES($1,$2,$3,$4,$5)`, hash(code), consent, r.RedirectURI, r.Challenge, time.Now().UTC().Add(60*time.Second))
		if e != nil {
			return "", ErrUnavailable
		}
		query.Set("code", code)
		details["consent_id"] = consent.String()
	} else {
		query.Set("error", "access_denied")
	}
	if e = s.auditTx(tx, user, action, details); e != nil {
		return "", ErrUnavailable
	}
	if e = tx.Commit(); e != nil {
		return "", ErrUnavailable
	}
	redirect.RawQuery = query.Encode()
	return redirect.String(), nil
}

type lineage struct {
	consent, user, sid, family, token uuid.UUID
	client, scope                     string
	expires                           time.Time
	revoked                           sql.NullTime
}

func (s *Service) discover(ctx context.Context, hashValue string, refresh bool) (lineage, error) {
	var l lineage
	column := "access_hash"
	if refresh {
		column = "refresh_hash"
	}
	e := s.db.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.session_id,c.client_id,c.scope,c.expires_at,t.family_id,t.id FROM mcp_oauth_tokens t JOIN mcp_oauth_families f ON f.id=t.family_id JOIN mcp_oauth_consents c ON c.id=f.consent_id WHERE t.`+column+`=$1`, hashValue).Scan(&l.consent, &l.user, &l.sid, &l.client, &l.scope, &l.expires, &l.family, &l.token)
	if errors.Is(e, sql.ErrNoRows) {
		return l, ErrDenied
	}
	if e != nil {
		return l, ErrUnavailable
	}
	return l, nil
}
func (s *Service) lockConsent(ctx context.Context, tx *sql.Tx, l *lineage) error {
	var resource string
	e := tx.QueryRowContext(ctx, `SELECT client_id,resource,scope,expires_at,revoked_at FROM mcp_oauth_consents WHERE id=$1 AND user_id=$2 AND session_id=$3 FOR UPDATE`, l.consent, l.user, l.sid).Scan(&l.client, &resource, &l.scope, &l.expires, &l.revoked)
	if errors.Is(e, sql.ErrNoRows) || l.revoked.Valid || resource != s.cfg.Resource || !l.expires.After(time.Now()) {
		return ErrDenied
	}
	if e != nil {
		return ErrUnavailable
	}
	_, e = s.client(ctx, tx, l.client, "")
	return e
}
func (s *Service) issue(ctx context.Context, tx *sql.Tx, l lineage) (TokenResponse, error) {
	access, e := token()
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	refresh, e := token()
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if l.expires.Before(expires) {
		expires = l.expires
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO mcp_oauth_tokens(id,family_id,access_hash,refresh_hash,expires_at) VALUES($1,$2,$3,$4,$5)`, uuid.New(), l.family, hash(access), hash(refresh), expires)
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	return TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(time.Until(expires).Seconds()), RefreshToken: refresh, Scope: l.scope}, nil
}
func (s *Service) Exchange(ctx context.Context, r TokenRequest) (TokenResponse, error) {
	if r.Resource != s.cfg.Resource || r.ClientID == "" {
		return TokenResponse{}, ErrInvalid
	}
	if r.GrantType == "refresh_token" {
		return s.refresh(ctx, r)
	}
	if r.GrantType != "authorization_code" || !validVerifier(r.Verifier) || r.Code == "" {
		return TokenResponse{}, ErrInvalid
	}
	var l lineage
	e := s.db.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.session_id FROM mcp_oauth_codes k JOIN mcp_oauth_consents c ON c.id=k.consent_id WHERE k.code_hash=$1`, hash(r.Code)).Scan(&l.consent, &l.user, &l.sid)
	if errors.Is(e, sql.ErrNoRows) {
		return TokenResponse{}, ErrDenied
	}
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, l.user, l.sid, false); e != nil {
		return TokenResponse{}, e
	}
	if e = s.lockConsent(ctx, tx, &l); e != nil {
		return TokenResponse{}, e
	}
	var redirect, challenge string
	var expires time.Time
	var used sql.NullTime
	e = tx.QueryRowContext(ctx, `SELECT redirect_uri,challenge,expires_at,consumed_at FROM mcp_oauth_codes WHERE code_hash=$1 FOR UPDATE`, hash(r.Code)).Scan(&redirect, &challenge, &expires, &used)
	digest := sha256.Sum256([]byte(r.Verifier))
	actual := base64.RawURLEncoding.EncodeToString(digest[:])
	if e != nil || used.Valid || !expires.After(time.Now()) || l.client != r.ClientID || redirect != r.RedirectURI || subtle.ConstantTimeCompare([]byte(actual), []byte(challenge)) != 1 {
		return TokenResponse{}, ErrDenied
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_codes SET consumed_at=NOW() WHERE code_hash=$1`, hash(r.Code)); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	l.family = uuid.New()
	if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_oauth_families(id,consent_id,expires_at) VALUES($1,$2,$3)`, l.family, l.consent, l.expires); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	result, e := s.issue(ctx, tx, l)
	if e != nil {
		return TokenResponse{}, e
	}
	if e = s.auditTx(tx, l.user, "mcp.token_issued", models.JSONMap{"family_id": l.family.String(), "client_id": l.client, "consent_id": l.consent.String()}); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	if e = tx.Commit(); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	return result, nil
}
func (s *Service) refresh(ctx context.Context, r TokenRequest) (TokenResponse, error) {
	if r.RefreshToken == "" {
		return TokenResponse{}, ErrInvalid
	}
	l, e := s.discover(ctx, hash(r.RefreshToken), true)
	if e != nil {
		return TokenResponse{}, e
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, l.user, l.sid, false); e != nil {
		return TokenResponse{}, e
	}
	if e = s.lockConsent(ctx, tx, &l); e != nil {
		return TokenResponse{}, e
	}
	if r.ClientID != l.client || r.Scope != "" && r.Scope != l.scope {
		return TokenResponse{}, ErrDenied
	}
	var familyExpiry time.Time
	var revoked, rotated sql.NullTime
	e = tx.QueryRowContext(ctx, `SELECT expires_at,revoked_at FROM mcp_oauth_families WHERE id=$1 FOR UPDATE`, l.family).Scan(&familyExpiry, &revoked)
	if e != nil || revoked.Valid || !familyExpiry.After(time.Now()) {
		return TokenResponse{}, ErrDenied
	}
	var id uuid.UUID
	e = tx.QueryRowContext(ctx, `SELECT id,rotated_at FROM mcp_oauth_tokens WHERE refresh_hash=$1 AND family_id=$2 FOR UPDATE`, hash(r.RefreshToken), l.family).Scan(&id, &rotated)
	if e != nil {
		return TokenResponse{}, ErrDenied
	}
	if rotated.Valid {
		if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_families SET revoked_at=NOW() WHERE id=$1`, l.family); e != nil {
			return TokenResponse{}, ErrUnavailable
		}
		if e = s.auditTx(tx, l.user, "mcp.refresh_replay", models.JSONMap{"family_id": l.family.String(), "client_id": l.client}); e != nil {
			return TokenResponse{}, ErrUnavailable
		}
		if e = tx.Commit(); e != nil {
			return TokenResponse{}, ErrUnavailable
		}
		return TokenResponse{}, ErrDenied
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_tokens SET rotated_at=NOW() WHERE id=$1`, id); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	if familyExpiry.Before(l.expires) {
		l.expires = familyExpiry
	}
	result, e := s.issue(ctx, tx, l)
	if e != nil {
		return TokenResponse{}, e
	}
	if e = s.auditTx(tx, l.user, "mcp.token_refreshed", models.JSONMap{"family_id": l.family.String(), "client_id": l.client}); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	if e = tx.Commit(); e != nil {
		return TokenResponse{}, ErrUnavailable
	}
	return result, nil
}
func (s *Service) Validate(ctx context.Context, raw string) (Principal, error) {
	if len(raw) != 43 {
		return Principal{}, ErrDenied
	}
	l, e := s.discover(ctx, hash(raw), false)
	if e != nil {
		return Principal{}, e
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return Principal{}, ErrUnavailable
	}
	defer tx.Rollback()
	parentExpiry, e := s.parent(ctx, tx, l.user, l.sid, false)
	if e != nil {
		return Principal{}, e
	}
	if e = s.lockConsent(ctx, tx, &l); e != nil {
		return Principal{}, e
	}
	var accessExpiry, familyExpiry time.Time
	var rotated, revoked sql.NullTime
	e = tx.QueryRowContext(ctx, `SELECT t.expires_at,t.rotated_at,f.expires_at,f.revoked_at FROM mcp_oauth_tokens t JOIN mcp_oauth_families f ON f.id=t.family_id WHERE t.access_hash=$1 AND f.id=$2`, hash(raw), l.family).Scan(&accessExpiry, &rotated, &familyExpiry, &revoked)
	if e != nil || rotated.Valid || revoked.Valid || !accessExpiry.After(time.Now()) || !familyExpiry.After(time.Now()) {
		return Principal{}, ErrDenied
	}
	for _, limit := range []time.Time{parentExpiry, l.expires, familyExpiry} {
		if limit.Before(accessExpiry) {
			accessExpiry = limit
		}
	}
	return Principal{UserID: l.user, SessionID: l.sid, FamilyID: l.family, TokenID: l.token, ClientID: l.client, Scope: l.scope, ExpiresAt: accessExpiry}, nil
}
func (s *Service) Revoke(ctx context.Context, raw, client string) error {
	if len(raw) != 43 {
		return nil
	}
	l, e := s.discover(ctx, hash(raw), true)
	if errors.Is(e, ErrDenied) {
		l, e = s.discover(ctx, hash(raw), false)
	}
	if errors.Is(e, ErrDenied) {
		return nil
	}
	if e != nil {
		return e
	}
	if client != l.client {
		return nil
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = s.parent(ctx, tx, l.user, l.sid, false); e != nil {
		if errors.Is(e, ErrDenied) {
			return nil
		}
		return e
	}
	if e = s.lockConsent(ctx, tx, &l); e != nil {
		if errors.Is(e, ErrDenied) {
			return nil
		}
		return e
	}
	var revoked sql.NullTime
	if e = tx.QueryRowContext(ctx, `SELECT revoked_at FROM mcp_oauth_families WHERE id=$1 FOR UPDATE`, l.family).Scan(&revoked); e != nil {
		return ErrUnavailable
	}
	if revoked.Valid {
		return nil
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_oauth_families SET revoked_at=NOW() WHERE id=$1`, l.family); e != nil {
		return ErrUnavailable
	}
	if e = s.auditTx(tx, l.user, "mcp.token_revoked", models.JSONMap{"family_id": l.family.String(), "client_id": l.client}); e != nil {
		return ErrUnavailable
	}
	if e = tx.Commit(); e != nil {
		return ErrUnavailable
	}
	return nil
}

// RequireGrantTx is the domain admission/delivery port. The caller first locks
// the complete subject set and resource/membership/session authority using the
// shared Guard; it then calls this before run/operation locks. ParentGrantID is
// the stored OAuth family, not a client declaration or a provider credential.
func (s *Service) RequireGrantTx(ctx context.Context, tx *sql.Tx, p policy.Principal, client string) error {
	if tx == nil || p.Kind != policy.OAuthDelegation || p.SubjectID == uuid.Nil || p.SessionID == uuid.Nil || p.ParentGrantID == uuid.Nil || !p.ExpiresAt.After(time.Now()) {
		return ErrDenied
	}
	var consent uuid.UUID
	var user, sid uuid.UUID
	e := tx.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.session_id FROM mcp_oauth_families f JOIN mcp_oauth_consents c ON c.id=f.consent_id WHERE f.id=$1`, p.ParentGrantID).Scan(&consent, &user, &sid)
	if e != nil || user != p.SubjectID || sid != p.SessionID {
		return ErrDenied
	}
	var resource, storedClient string
	var expires time.Time
	var revoked sql.NullTime
	e = tx.QueryRowContext(ctx, `SELECT resource,client_id,expires_at,revoked_at FROM mcp_oauth_consents WHERE id=$1 FOR SHARE`, consent).Scan(&resource, &storedClient, &expires, &revoked)
	if e != nil || resource != s.cfg.Resource || storedClient != client || revoked.Valid || !expires.After(time.Now()) {
		return ErrDenied
	}
	e = tx.QueryRowContext(ctx, `SELECT expires_at,revoked_at FROM mcp_oauth_families WHERE id=$1 FOR SHARE`, p.ParentGrantID).Scan(&expires, &revoked)
	if e != nil || revoked.Valid || !expires.After(time.Now()) {
		return ErrDenied
	}
	tokenID, e := uuid.Parse(p.TokenID)
	if e != nil || tokenID == uuid.Nil {
		return ErrDenied
	}
	var id uuid.UUID
	if e = tx.QueryRowContext(ctx, `SELECT id FROM mcp_oauth_tokens WHERE id=$1 AND family_id=$2 AND rotated_at IS NULL AND expires_at>NOW() FOR SHARE`, tokenID, p.ParentGrantID).Scan(&id); e != nil {
		return ErrDenied
	}
	_, e = s.client(ctx, tx, client, "")
	return e
}
