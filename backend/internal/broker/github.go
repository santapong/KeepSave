// Package broker owns credential custody and structured GitHub reads. Neither
// callers nor connectors can choose upstream URLs, headers or HTTP methods.
package broker

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/santapong/KeepSave/backend/internal/crypto"
)

var ErrProvider = errors.New("provider read failed")
var ErrUncertain = errors.New("provider outcome uncertain")

const MaxResultBytes = 4 << 20
const APIVersion = "2026-03-10"

type Custody interface {
	Seal([]byte) ([]byte, []byte, error)
	Open([]byte, []byte) ([]byte, error)
}
type CryptoCustody struct{ Service *crypto.Service }

func (c CryptoCustody) Seal(b []byte) ([]byte, []byte, error) {
	return c.Service.EncryptServiceSecret(b)
}
func (c CryptoCustody) Open(b, n []byte) ([]byte, error) { return c.Service.DecryptServiceSecret(b, n) }

type Service struct {
	custody Custody
	client  *http.Client
	base    string
}

func New(c Custody) *Service {
	return &Service{c, &http.Client{Timeout: 30 * time.Second, CheckRedirect: noRedirect}, "https://api.github.com"}
}
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// NewFixture cannot be selected through production configuration.
func NewFixture(c Custody, client *http.Client, endpoint string) (*Service, error) {
	u, e := url.Parse(endpoint)
	if e != nil || client == nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, ErrProvider
	}
	safe := *client
	safe.Timeout = 30 * time.Second
	safe.CheckRedirect = noRedirect
	return &Service{c, &safe, strings.TrimSuffix(endpoint, "/")}, nil
}
func (s *Service) Seal(b []byte) ([]byte, []byte, error) {
	if s == nil || s.custody == nil {
		return nil, nil, ErrProvider
	}
	return s.custody.Seal(b)
}

// WithOpened confines decrypted material to the trusted broker callback.
func (s *Service) WithOpened(b, n []byte, fn func([]byte) error) error {
	if s == nil || s.custody == nil || fn == nil {
		return ErrProvider
	}
	p, e := s.custody.Open(b, n)
	if e != nil {
		return ErrProvider
	}
	defer crypto.SecureZero(p)
	return fn(p)
}

type Connection struct {
	AppID, InstallationID int64
	Ciphertext, Nonce     []byte
}
type Target struct {
	RepositoryID int64  `json:"repository_id"`
	Owner        string `json:"owner"`
	Repository   string `json:"repository"`
	Reference    string `json:"reference,omitempty"`
	Commit       string `json:"commit,omitempty"`
	TreeSHA      string `json:"tree_sha,omitempty"`
}

var slug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
var commit = regexp.MustCompile(`^[a-f0-9]{40}$`)
var ref = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]{0,199}$`)

func ValidReference(v string) bool {
	return ref.MatchString(v) && !strings.Contains(v, "..") && !strings.Contains(v, "//") && !strings.HasSuffix(v, "/") && !strings.HasSuffix(v, ".") && !strings.HasSuffix(v, ".lock")
}
func ValidTargetSpec(t Target) bool {
	return t.RepositoryID > 0 && slug.MatchString(t.Owner) && slug.MatchString(t.Repository) && (ValidReference(t.Reference) || t.Reference == "" && commit.MatchString(t.Commit))
}
func ValidTarget(t Target) bool {
	return ValidTargetSpec(t) && commit.MatchString(t.Commit) && commit.MatchString(t.TreeSHA)
}
func privateKey(b []byte) (*rsa.PrivateKey, error) {
	p, rest := pem.Decode(b)
	if p == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrProvider
	}
	if k, e := x509.ParsePKCS1PrivateKey(p.Bytes); e == nil {
		if k.Validate() != nil {
			return nil, ErrProvider
		}
		return k, nil
	}
	k, e := x509.ParsePKCS8PrivateKey(p.Bytes)
	if e != nil {
		return nil, ErrProvider
	}
	r, ok := k.(*rsa.PrivateKey)
	if !ok || r.Validate() != nil {
		return nil, ErrProvider
	}
	return r, nil
}
func ValidateKey(b []byte) error {
	k, e := privateKey(b)
	if e != nil || k.N.BitLen() < 2048 || len(b) > 16<<10 {
		return ErrProvider
	}
	return nil
}

// Admission is called before credential access and before every upstream call.
// The operation owner rechecks database authority and charges actual HTTP calls.
type Admission func(context.Context, string) error

func admit(ctx context.Context, a Admission, stage string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if a != nil {
		return a(ctx, stage)
	}
	return nil
}
func (s *Service) read(ctx context.Context, a Admission, method, path, token string, body []byte, max int64) ([]byte, error) {
	if e := admit(ctx, a, "provider_request"); e != nil {
		return nil, e
	}
	r, e := http.NewRequestWithContext(ctx, method, s.base+path, bytes.NewReader(body))
	if e != nil {
		return nil, ErrProvider
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("X-GitHub-Api-Version", APIVersion)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	resp, e := s.client.Do(r)
	if e != nil {
		return nil, ErrUncertain
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrProvider
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if e != nil || int64(len(b)) > max {
		return nil, ErrProvider
	}
	return b, nil
}
func (s *Service) token(ctx context.Context, a Admission, c Connection, repo int64) (string, error) {
	if c.AppID < 1 || c.InstallationID < 1 || repo < 1 {
		return "", ErrProvider
	}
	if e := admit(ctx, a, "credential"); e != nil {
		return "", e
	}
	var signed string
	e := s.WithOpened(c.Ciphertext, c.Nonce, func(key []byte) error {
		rsaKey, e := privateKey(key)
		if e != nil {
			return e
		}
		now := time.Now()
		claims := jwt.RegisteredClaims{Issuer: fmt.Sprint(c.AppID), IssuedAt: jwt.NewNumericDate(now.Add(-30 * time.Second)), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}
		signed, e = jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(rsaKey)
		return e
	})
	if e != nil {
		return "", ErrProvider
	}
	body, _ := json.Marshal(map[string]any{"repository_ids": []int64{repo}, "permissions": map[string]string{"contents": "read"}})
	raw, e := s.read(ctx, a, "POST", fmt.Sprintf("/app/installations/%d/access_tokens", c.InstallationID), signed, body, 64<<10)
	if e != nil {
		return "", e
	}
	defer crypto.SecureZero(raw)
	var out struct {
		Token        string            `json:"token"`
		ExpiresAt    time.Time         `json:"expires_at"`
		Permissions  map[string]string `json:"permissions"`
		Repositories []struct {
			ID int64 `json:"id"`
		} `json:"repositories"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Token == "" || len(out.Token) > 4096 || !out.ExpiresAt.After(time.Now()) || out.Permissions["contents"] != "read" || len(out.Repositories) != 1 || out.Repositories[0].ID != repo {
		return "", ErrProvider
	}
	for k, v := range out.Permissions {
		if v != "read" || (k != "contents" && k != "metadata") {
			return "", ErrProvider
		}
	}
	return out.Token, nil
}
func (s *Service) Resolve(ctx context.Context, c Connection, t Target) (Target, error) {
	return s.ResolveAuthorized(ctx, c, t, nil)
}
func (s *Service) ResolveAuthorized(ctx context.Context, c Connection, t Target, a Admission) (Target, error) {
	if !ValidTargetSpec(t) {
		return Target{}, ErrProvider
	}
	token, e := s.token(ctx, a, c, t.RepositoryID)
	if e != nil {
		return Target{}, e
	}
	raw, e := s.read(ctx, a, "GET", fmt.Sprintf("/repositories/%d", t.RepositoryID), token, nil, 64<<10)
	if e != nil {
		return Target{}, e
	}
	var repo struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	}
	if json.Unmarshal(raw, &repo) != nil || repo.ID != t.RepositoryID || !strings.EqualFold(repo.FullName, t.Owner+"/"+t.Repository) {
		return Target{}, ErrProvider
	}
	reference := t.Reference
	if reference == "" {
		reference = t.Commit
	}
	raw, e = s.read(ctx, a, "GET", "/repos/"+url.PathEscape(t.Owner)+"/"+url.PathEscape(t.Repository)+"/commits/"+url.PathEscape(reference), token, nil, 512<<10)
	if e != nil {
		return Target{}, e
	}
	var resolved struct {
		SHA    string `json:"sha"`
		Commit struct {
			Tree struct {
				SHA string `json:"sha"`
			} `json:"tree"`
		} `json:"commit"`
	}
	if json.Unmarshal(raw, &resolved) != nil || !commit.MatchString(resolved.SHA) || !commit.MatchString(resolved.Commit.Tree.SHA) || commit.MatchString(reference) && resolved.SHA != reference {
		return Target{}, ErrProvider
	}
	t.Commit = resolved.SHA
	t.TreeSHA = resolved.Commit.Tree.SHA
	return t, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	SHA  string `json:"sha"`
	Size int64  `json:"size,omitempty"`
}
type Tree struct {
	Commit  string      `json:"commit"`
	Entries []TreeEntry `json:"entries"`
}

func (s *Service) tree(ctx context.Context, a Admission, token string, t Target, max int) (Tree, error) {
	prefix := "/repos/" + url.PathEscape(t.Owner) + "/" + url.PathEscape(t.Repository)
	raw, e := s.read(ctx, a, "GET", prefix+"/git/trees/"+t.TreeSHA+"?recursive=1", token, nil, int64(max))
	if e != nil {
		return Tree{}, e
	}
	var value struct {
		SHA       string      `json:"sha"`
		Truncated bool        `json:"truncated"`
		Entries   []TreeEntry `json:"tree"`
	}
	if json.Unmarshal(raw, &value) != nil || value.SHA != t.TreeSHA || value.Truncated || len(value.Entries) > 10000 {
		return Tree{}, ErrProvider
	}
	seen := map[string]bool{}
	for _, entry := range value.Entries {
		if !ValidPath(entry.Path) || !commit.MatchString(entry.SHA) || seen[entry.Path] || entry.Size < 0 {
			return Tree{}, ErrProvider
		}
		seen[entry.Path] = true
		if entry.Type == "tree" && entry.Mode == "040000" {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return Tree{}, ErrProvider
		}
	}
	if value.Entries == nil {
		value.Entries = []TreeEntry{}
	}
	return Tree{t.Commit, value.Entries}, nil
}
func (s *Service) Execute(ctx context.Context, c Connection, t Target, kind, path string, max int) (json.RawMessage, error) {
	return s.ExecuteAuthorized(ctx, c, t, kind, path, max, nil)
}
func (s *Service) ExecuteAuthorized(ctx context.Context, c Connection, t Target, kind, path string, max int, a Admission) (json.RawMessage, error) {
	if !ValidTarget(t) || max < 1024 || max > MaxResultBytes || kind != "repository_tree" && kind != "read_file" || kind == "repository_tree" && path != "" || kind == "read_file" && !ValidPath(path) {
		return nil, ErrProvider
	}
	token, e := s.token(ctx, a, c, t.RepositoryID)
	if e != nil {
		return nil, e
	}
	tree, e := s.tree(ctx, a, token, t, max)
	if e != nil {
		return nil, e
	}
	var result any = tree
	if kind == "read_file" {
		var selected *TreeEntry
		for i := range tree.Entries {
			if tree.Entries[i].Path == path {
				selected = &tree.Entries[i]
				break
			}
		}
		if selected == nil || selected.Type != "blob" || selected.Size > int64(max) {
			return nil, ErrProvider
		}
		prefix := "/repos/" + url.PathEscape(t.Owner) + "/" + url.PathEscape(t.Repository)
		raw, e := s.read(ctx, a, "GET", prefix+"/git/blobs/"+selected.SHA, token, nil, int64(max))
		if e != nil {
			return nil, e
		}
		var file struct {
			SHA      string `json:"sha"`
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
			Size     int64  `json:"size"`
		}
		if json.Unmarshal(raw, &file) != nil || file.SHA != selected.SHA || file.Encoding != "base64" || file.Size != selected.Size || file.Size > int64(max) {
			return nil, ErrProvider
		}
		plain, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if e != nil || int64(len(plain)) != file.Size || !utf8.Valid(plain) || strings.HasPrefix(string(plain), "version https://git-lfs.github.com/spec/") {
			return nil, ErrProvider
		}
		for _, r := range string(plain) {
			if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
				return nil, ErrProvider
			}
		}
		h := sha1.New()
		fmt.Fprintf(h, "blob %d%c", len(plain), 0)
		h.Write(plain)
		if hex.EncodeToString(h.Sum(nil)) != file.SHA {
			return nil, ErrProvider
		}
		result = map[string]any{"commit": t.Commit, "path": path, "blob_sha": file.SHA, "content": string(plain)}
	}
	b, e := json.Marshal(result)
	if e != nil || len(b) > max || bytes.Contains(b, []byte(token)) {
		return nil, ErrProvider
	}
	if e = admit(ctx, a, "credential"); e != nil {
		return nil, e
	}
	if e = s.WithOpened(c.Ciphertext, c.Nonce, func(key []byte) error {
		encoded, _ := json.Marshal(string(key))
		if bytes.Contains(b, key) || len(encoded) > 2 && bytes.Contains(b, encoded[1:len(encoded)-1]) {
			return ErrProvider
		}
		return nil
	}); e != nil {
		return nil, ErrProvider
	}
	return b, nil
}
func ValidPath(p string) bool {
	if !utf8.ValidString(p) || len(p) < 1 || len(p) > 512 || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00\r\n") {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, v := range strings.Split(p, "/") {
		if v == "" || v == "." || v == ".." {
			return false
		}
	}
	return true
}
