package broker

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const fixtureToken = "synthetic-installation-token-canary"
const fixtureCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const fixtureTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

type fixtureCustody struct {
	key    []byte
	opens  atomic.Int32
	opened []byte
}

func (c *fixtureCustody) Seal(b []byte) ([]byte, []byte, error) {
	return []byte("synthetic-ciphertext"), []byte("synthetic-nonce"), nil
}
func (c *fixtureCustody) Open([]byte, []byte) ([]byte, error) {
	c.opens.Add(1)
	c.opened = append([]byte(nil), c.key...)
	return c.opened, nil
}
func fixtureRSA(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}
func blobDigest(content []byte) string {
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "blob %d%c", len(content), 0)
	_, _ = h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// All calls are to this loopback fixture. The generated key/token and provider
// response data are synthetic; no real provider connection is configured.
func fixtureBroker(t *testing.T, key *rsa.PrivateKey, keyPEM []byte, mode string) (*Service, *fixtureCustody, *atomic.Int32, Target) {
	t.Helper()
	custody := &fixtureCustody{key: keyPEM}
	requests := &atomic.Int32{}
	content := []byte("# Synthetic review\n")
	switch mode {
	case "non_utf8":
		content = []byte{0xff, 0xfe}
	case "lfs":
		content = []byte("version https://git-lfs.github.com/spec/v1\noid sha256:synthetic\n")
	case "control":
		content = []byte("bad\x00content")
	case "token_content":
		content = []byte(fixtureToken)
	}
	sha := blobDigest(content)
	target := Target{RepositoryID: 42, Owner: "acme", Repository: "demo", Commit: fixtureCommit, TreeSHA: fixtureTree}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("X-GitHub-Api-Version") != APIVersion || r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Error("provider headers not fixed")
		}
		if r.URL.Path == "/app/installations/9/access_tokens" {
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("token call method/schema")
			}
			var body struct {
				RepositoryIDs []int64           `json:"repository_ids"`
				Permissions   map[string]string `json:"permissions"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.RepositoryIDs) != 1 || body.RepositoryIDs[0] != 42 || len(body.Permissions) != 1 || body.Permissions["contents"] != "read" {
				t.Error("token scope broadened")
			}
			parsed, e := jwt.Parse(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), func(token *jwt.Token) (any, error) {
				if token.Method.Alg() != "RS256" {
					return nil, errors.New("unexpected signing method")
				}
				return &key.PublicKey, nil
			}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("7"))
			if e != nil || !parsed.Valid {
				t.Error("invalid app JWT")
			}
			permissions := map[string]string{"contents": "read", "metadata": "read"}
			repos := []map[string]int64{{"id": 42}}
			expiry := time.Now().Add(time.Minute)
			switch mode {
			case "token_write":
				permissions["contents"] = "write"
			case "token_extra":
				permissions["issues"] = "read"
			case "token_other_repo":
				repos[0]["id"] = 43
			case "token_two_repos":
				repos = append(repos, map[string]int64{"id": 43})
			case "token_expired":
				expiry = time.Now().Add(-time.Minute)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"token": fixtureToken, "expires_at": expiry, "permissions": permissions, "repositories": repos})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken || r.Method != "GET" {
			t.Error("read not authorized with broker token")
		}
		switch r.URL.Path {
		case "/repositories/42":
			name := "acme/demo"
			if mode == "wrong_repository" {
				name = "acme/other"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "full_name": name, "private_debug_token": fixtureToken})
		case "/repos/acme/demo/commits/" + fixtureCommit, "/repos/acme/demo/commits/main":
			commit := fixtureCommit
			if mode == "wrong_commit" {
				commit = strings.Repeat("c", 40)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]string{"sha": fixtureTree}}, "private_debug_token": fixtureToken})
		case "/repos/acme/demo/git/trees/" + fixtureTree:
			if r.URL.RawQuery != "recursive=1" {
				t.Error("tree not pinned/recursive")
			}
			entry := TreeEntry{Path: "README.md", Type: "blob", Mode: "100644", SHA: sha, Size: int64(len(content))}
			treeSHA := fixtureTree
			switch mode {
			case "symlink":
				entry.Mode = "120000"
			case "submodule":
				entry.Type = "commit"
				entry.Mode = "160000"
			case "path_traversal":
				entry.Path = "../secret"
			case "negative_size":
				entry.Size = -1
			case "wrong_tree":
				treeSHA = strings.Repeat("c", 40)
			}
			entries := []TreeEntry{entry}
			if mode == "duplicate_path" {
				entries = append(entries, entry)
			}
			if mode == "tree_cardinality" {
				entries = make([]TreeEntry, 10001)
				for i := range entries {
					entries[i] = entry
					entries[i].Path = fmt.Sprint("file", i)
				}
			}
			if mode == "tree_oversize" {
				_, _ = w.Write([]byte(strings.Repeat("x", 4097)))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": treeSHA, "truncated": mode == "truncated", "tree": entries, "private_debug_token": fixtureToken})
		case "/repos/acme/demo/git/blobs/" + sha:
			outSHA := sha
			encoding := "base64"
			size := int64(len(content))
			encoded := base64.StdEncoding.EncodeToString(content)
			switch mode {
			case "blob_wrong_sha":
				outSHA = strings.Repeat("c", 40)
			case "blob_wrong_size":
				size++
			case "blob_wrong_encoding":
				encoding = "utf-8"
			case "blob_bad_base64":
				encoded = "not base64%%%"
			case "blob_checksum":
				encoded = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("x"), len(content)))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": outSHA, "encoding": encoding, "size": size, "content": encoded, "private_debug_token": fixtureToken})
		default:
			t.Errorf("unexpected provider path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(s.Close)
	service, e := NewFixture(custody, s.Client(), s.URL)
	if e != nil {
		t.Fatal(e)
	}
	return service, custody, requests, target
}
func fixtureConnection() Connection {
	return Connection{AppID: 7, InstallationID: 9, Ciphertext: []byte("synthetic-ciphertext"), Nonce: []byte("synthetic-nonce")}
}

func TestGitHubPinnedReadAndProviderCustody(t *testing.T) {
	key, keyPEM := fixtureRSA(t)
	s, c, requests, target := fixtureBroker(t, key, keyPEM, "")
	spec := target
	spec.Reference = "main"
	spec.Commit = ""
	spec.TreeSHA = ""
	resolved, e := s.ResolveAuthorized(context.Background(), fixtureConnection(), spec, func(context.Context, string) error { return nil })
	if e != nil || resolved.Commit != fixtureCommit || resolved.TreeSHA != fixtureTree {
		t.Fatal("reference did not pin commit/tree")
	}
	if requests.Load() != 3 {
		t.Fatal("unexpected binding request count")
	}
	for _, operation := range []struct{ kind, path string }{{"repository_tree", ""}, {"read_file", "README.md"}} {
		result, e := s.ExecuteAuthorized(context.Background(), fixtureConnection(), resolved, operation.kind, operation.path, 4096, func(context.Context, string) error { return nil })
		if e != nil || !json.Valid(result) || bytes.Contains(result, []byte(fixtureToken)) || bytes.Contains(result, keyPEM) {
			t.Fatal("read result or custody unsafe")
		}
		if !bytes.Contains(result, []byte(fixtureCommit)) {
			t.Fatal("result omitted pinned commit")
		}
	}
	for _, b := range c.opened {
		if b != 0 {
			t.Fatal("unwrapped app key was not zeroed")
		}
	}
}

func TestGitHubRejectsBroaderTokensAndWrongResolvedTarget(t *testing.T) {
	key, keyPEM := fixtureRSA(t)
	for _, mode := range []string{"token_write", "token_extra", "token_other_repo", "token_two_repos", "token_expired", "wrong_repository", "wrong_commit"} {
		t.Run(mode, func(t *testing.T) {
			s, _, _, target := fixtureBroker(t, key, keyPEM, mode)
			target.TreeSHA = ""
			if _, e := s.Resolve(context.Background(), fixtureConnection(), target); e == nil {
				t.Fatal("unsafe token/target admitted")
			}
		})
	}
}

func TestGitHubRejectsInvalidTreeBlobAndCanaryResponses(t *testing.T) {
	key, keyPEM := fixtureRSA(t)
	for _, mode := range []string{"truncated", "symlink", "submodule", "path_traversal", "negative_size", "wrong_tree", "duplicate_path", "tree_cardinality", "tree_oversize", "blob_wrong_sha", "blob_wrong_size", "blob_wrong_encoding", "blob_bad_base64", "blob_checksum", "non_utf8", "lfs", "control", "token_content"} {
		t.Run(mode, func(t *testing.T) {
			s, _, _, target := fixtureBroker(t, key, keyPEM, mode)
			max := 4096
			if mode == "tree_cardinality" {
				max = MaxResultBytes
			}
			result, e := s.Execute(context.Background(), fixtureConnection(), target, "read_file", "README.md", max)
			if e == nil || len(result) != 0 || strings.Contains(e.Error(), fixtureToken) {
				t.Fatal("unsafe provider content released")
			}
		})
	}
}

func TestAdmissionDeniesBeforeUnwrapAndEveryProviderRequest(t *testing.T) {
	key, keyPEM := fixtureRSA(t)
	for _, tc := range []struct {
		name            string
		denyProvider    int
		denyCredential  bool
		opens, requests int32
	}{{"credential", 0, true, 0, 0}, {"mint", 1, false, 1, 0}, {"tree", 2, false, 1, 1}, {"blob", 3, false, 1, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, requests, target := fixtureBroker(t, key, keyPEM, "")
			provider := 0
			deny := errors.New("synthetic authority denied")
			admission := func(_ context.Context, stage string) error {
				if stage == "credential" && tc.denyCredential {
					return deny
				}
				if stage == "provider_request" {
					provider++
					if provider == tc.denyProvider {
						return deny
					}
				}
				return nil
			}
			result, e := s.ExecuteAuthorized(context.Background(), fixtureConnection(), target, "read_file", "README.md", 4096, admission)
			if e == nil || len(result) > 0 || c.opens.Load() != tc.opens || requests.Load() != tc.requests {
				t.Fatal("admission did not guard custody/network")
			}
			for _, b := range c.opened {
				if b != 0 {
					t.Fatal("denied opened key retained")
				}
			}
		})
	}
}

func TestBrokerInvalidArgumentsMakeNoCredentialOrNetworkCall(t *testing.T) {
	key, keyPEM := fixtureRSA(t)
	s, c, requests, target := fixtureBroker(t, key, keyPEM, "")
	for _, tc := range []struct {
		kind, path string
		max        int
	}{{"shell", "", 4096}, {"repository_tree", "README.md", 4096}, {"read_file", "../token", 4096}, {"read_file", "README.md", 1023}, {"read_file", "README.md", MaxResultBytes + 1}} {
		if _, e := s.Execute(context.Background(), fixtureConnection(), target, tc.kind, tc.path, tc.max); e == nil {
			t.Fatal("invalid operation accepted")
		}
	}
	if c.opens.Load() != 0 || requests.Load() != 0 {
		t.Fatal("invalid request reached custody")
	}
}

func TestBrokerFixtureAndPrivateKeyBounds(t *testing.T) {
	for _, endpoint := range []string{"https://127.0.0.1", "http://provider.example", "http://user:pass@127.0.0.1", "http://127.0.0.1/path", "http://127.0.0.1?x=y"} {
		if _, e := NewFixture(nil, &http.Client{}, endpoint); e == nil {
			t.Fatal("nonfixture destination accepted")
		}
	}
	key, keyPEM := fixtureRSA(t)
	if ValidateKey(keyPEM) != nil {
		t.Fatal("generated RSA2048 denied")
	}
	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	weakPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(weak)})
	for _, b := range [][]byte{weakPEM, append(append([]byte(nil), keyPEM...), []byte("extra")...), []byte("not a key"), bytes.Repeat([]byte("x"), 16<<10+1)} {
		if ValidateKey(b) == nil {
			t.Fatal("unsupported key accepted")
		}
	}
	_ = key
}
