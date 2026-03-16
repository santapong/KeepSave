package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type OAuthRepository struct {
	db      *sql.DB
	dialect Dialect
}

func NewOAuthRepository(db *sql.DB, dialect Dialect) *OAuthRepository {
	return &OAuthRepository{db: db, dialect: dialect}
}

// Client operations

func (r *OAuthRepository) CreateClient(client *models.OAuthClient) error {
	id := uuid.New()
	scopes, _ := r.dialect.ArrayParam(client.Scopes)
	redirectURIs, _ := r.dialect.ArrayParam(client.RedirectURIs)
	grantTypes, _ := r.dialect.ArrayParam(client.GrantTypes)

	query := Q(r.dialect, `INSERT INTO oauth_clients (id, client_id, client_secret_hash, name, description, owner_id, redirect_uris, scopes, grant_types, logo_url, homepage_url, is_public)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`)

	_, err := r.db.Exec(query, id, client.ClientID, client.ClientSecretHash, client.Name, client.Description, client.OwnerID, redirectURIs, scopes, grantTypes, client.LogoURL, client.HomepageURL, client.IsPublic)
	if err != nil {
		return fmt.Errorf("creating oauth client: %w", err)
	}
	client.ID = id
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()
	return nil
}

func (r *OAuthRepository) GetClientByClientID(clientID string) (*models.OAuthClient, error) {
	client := &models.OAuthClient{}
	query := Q(r.dialect, `SELECT id, client_id, client_secret_hash, name, description, owner_id, redirect_uris, scopes, grant_types, logo_url, homepage_url, is_public, created_at, updated_at
		FROM oauth_clients WHERE client_id = $1`)
	err := r.db.QueryRow(query, clientID).Scan(
		&client.ID, &client.ClientID, &client.ClientSecretHash, &client.Name, &client.Description,
		&client.OwnerID, &client.RedirectURIs, &client.Scopes, &client.GrantTypes,
		&client.LogoURL, &client.HomepageURL, &client.IsPublic, &client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting oauth client: %w", err)
	}
	return client, nil
}

func (r *OAuthRepository) GetClientByID(id uuid.UUID) (*models.OAuthClient, error) {
	client := &models.OAuthClient{}
	query := Q(r.dialect, `SELECT id, client_id, client_secret_hash, name, description, owner_id, redirect_uris, scopes, grant_types, logo_url, homepage_url, is_public, created_at, updated_at
		FROM oauth_clients WHERE id = $1`)
	err := r.db.QueryRow(query, id).Scan(
		&client.ID, &client.ClientID, &client.ClientSecretHash, &client.Name, &client.Description,
		&client.OwnerID, &client.RedirectURIs, &client.Scopes, &client.GrantTypes,
		&client.LogoURL, &client.HomepageURL, &client.IsPublic, &client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting oauth client by id: %w", err)
	}
	return client, nil
}

func (r *OAuthRepository) ListClientsByOwner(ownerID uuid.UUID) ([]models.OAuthClient, error) {
	query := Q(r.dialect, `SELECT id, client_id, client_secret_hash, name, description, owner_id, redirect_uris, scopes, grant_types, logo_url, homepage_url, is_public, created_at, updated_at
		FROM oauth_clients WHERE owner_id = $1 ORDER BY created_at DESC`)
	rows, err := r.db.Query(query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("listing oauth clients: %w", err)
	}
	defer rows.Close()

	var clients []models.OAuthClient
	for rows.Next() {
		var c models.OAuthClient
		if err := rows.Scan(
			&c.ID, &c.ClientID, &c.ClientSecretHash, &c.Name, &c.Description,
			&c.OwnerID, &c.RedirectURIs, &c.Scopes, &c.GrantTypes,
			&c.LogoURL, &c.HomepageURL, &c.IsPublic, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning oauth client: %w", err)
		}
		clients = append(clients, c)
	}
	return clients, nil
}

func (r *OAuthRepository) DeleteClient(id uuid.UUID, ownerID uuid.UUID) error {
	query := Q(r.dialect, `DELETE FROM oauth_clients WHERE id = $1 AND owner_id = $2`)
	result, err := r.db.Exec(query, id, ownerID)
	if err != nil {
		return fmt.Errorf("deleting oauth client: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("oauth client not found")
	}
	return nil
}

// Authorization Code operations

func (r *OAuthRepository) CreateAuthorizationCode(code *models.OAuthAuthorizationCode) error {
	id := uuid.New()
	scopes, _ := r.dialect.ArrayParam(code.Scopes)
	query := Q(r.dialect, `INSERT INTO oauth_authorization_codes (id, code, client_id, user_id, redirect_uri, scopes, code_challenge, code_challenge_method, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`)
	_, err := r.db.Exec(query, id, code.Code, code.ClientID, code.UserID, code.RedirectURI, scopes, code.CodeChallenge, code.CodeChallengeMethod, code.ExpiresAt)
	if err != nil {
		return fmt.Errorf("creating authorization code: %w", err)
	}
	code.ID = id
	return nil
}

func (r *OAuthRepository) GetAuthorizationCode(code string) (*models.OAuthAuthorizationCode, error) {
	ac := &models.OAuthAuthorizationCode{}
	query := Q(r.dialect, `SELECT id, code, client_id, user_id, redirect_uri, scopes, code_challenge, code_challenge_method, expires_at, used, created_at
		FROM oauth_authorization_codes WHERE code = $1`)
	err := r.db.QueryRow(query, code).Scan(
		&ac.ID, &ac.Code, &ac.ClientID, &ac.UserID, &ac.RedirectURI, &ac.Scopes,
		&ac.CodeChallenge, &ac.CodeChallengeMethod, &ac.ExpiresAt, &ac.Used, &ac.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting authorization code: %w", err)
	}
	return ac, nil
}

func (r *OAuthRepository) MarkCodeUsed(id uuid.UUID) error {
	query := Q(r.dialect, `UPDATE oauth_authorization_codes SET used = TRUE WHERE id = $1`)
	_, err := r.db.Exec(query, id)
	return err
}

// Token operations

func (r *OAuthRepository) CreateToken(token *models.OAuthToken) error {
	id := uuid.New()
	scopes, _ := r.dialect.ArrayParam(token.Scopes)
	query := Q(r.dialect, `INSERT INTO oauth_tokens (id, access_token_hash, refresh_token_hash, client_id, user_id, scopes, token_type, expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`)
	_, err := r.db.Exec(query, id, token.AccessTokenHash, token.RefreshTokenHash, token.ClientID, token.UserID, scopes, token.TokenType, token.ExpiresAt, token.RefreshExpiresAt)
	if err != nil {
		return fmt.Errorf("creating oauth token: %w", err)
	}
	token.ID = id
	return nil
}

func (r *OAuthRepository) GetTokenByAccessHash(hash string) (*models.OAuthToken, error) {
	token := &models.OAuthToken{}
	query := Q(r.dialect, `SELECT id, access_token_hash, refresh_token_hash, client_id, user_id, scopes, token_type, expires_at, refresh_expires_at, revoked, created_at
		FROM oauth_tokens WHERE access_token_hash = $1`)
	err := r.db.QueryRow(query, hash).Scan(
		&token.ID, &token.AccessTokenHash, &token.RefreshTokenHash, &token.ClientID, &token.UserID,
		&token.Scopes, &token.TokenType, &token.ExpiresAt, &token.RefreshExpiresAt, &token.Revoked, &token.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting token by access hash: %w", err)
	}
	return token, nil
}

func (r *OAuthRepository) GetTokenByRefreshHash(hash string) (*models.OAuthToken, error) {
	token := &models.OAuthToken{}
	query := Q(r.dialect, `SELECT id, access_token_hash, refresh_token_hash, client_id, user_id, scopes, token_type, expires_at, refresh_expires_at, revoked, created_at
		FROM oauth_tokens WHERE refresh_token_hash = $1`)
	err := r.db.QueryRow(query, hash).Scan(
		&token.ID, &token.AccessTokenHash, &token.RefreshTokenHash, &token.ClientID, &token.UserID,
		&token.Scopes, &token.TokenType, &token.ExpiresAt, &token.RefreshExpiresAt, &token.Revoked, &token.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting token by refresh hash: %w", err)
	}
	return token, nil
}

func (r *OAuthRepository) RevokeToken(id uuid.UUID) error {
	query := Q(r.dialect, `UPDATE oauth_tokens SET revoked = TRUE WHERE id = $1`)
	_, err := r.db.Exec(query, id)
	return err
}

func (r *OAuthRepository) RevokeAllTokensForClient(clientID uuid.UUID) error {
	query := Q(r.dialect, `UPDATE oauth_tokens SET revoked = TRUE WHERE client_id = $1`)
	_, err := r.db.Exec(query, clientID)
	return err
}

func (r *OAuthRepository) CleanupExpiredCodes() error {
	query := Q(r.dialect, `DELETE FROM oauth_authorization_codes WHERE expires_at < $1`)
	_, err := r.db.Exec(query, time.Now())
	return err
}

func (r *OAuthRepository) CleanupExpiredTokens() error {
	query := Q(r.dialect, `DELETE FROM oauth_tokens WHERE expires_at < $1 AND revoked = TRUE`)
	_, err := r.db.Exec(query, time.Now())
	return err
}
