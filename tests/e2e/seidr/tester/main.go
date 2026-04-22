// Tester binary for the KeepSave <-> Seidr E2E harness.
//
// Exits 0 on success, non-zero on any failure. Stdlib-only so the
// standalone Go module needs no go.sum and builds in a slim Alpine
// image without external deps.
//
// The sequence matches what Seidr's KeepSaveSecretProvider issues at
// startup: register/login to get a JWT, create a project, store a
// marker secret in alpha, mint a scoped API key, then fetch the secret
// back with the API key and confirm the plaintext round-trips.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	testEmail       = "e2e-tester@keepsave.local"
	testPassword    = "SeidrE2E-Pass-9xq!"
	testSecretKey   = "SEIDR_E2E_MARKER"
	testSecretValue = "the-plaintext-that-must-round-trip"
)

func main() {
	baseURL := os.Getenv("KEEPSAVE_URL")
	if baseURL == "" {
		baseURL = "http://keepsave:8080"
	}
	logf("keepsave url: %s", baseURL)

	waitReady(baseURL)

	token := registerAndLogin(baseURL)
	projectID := createProject(baseURL, token)
	logf("project: %s", projectID)

	storeSecret(baseURL, token, projectID)
	apiKey := createAPIKey(baseURL, token, projectID)
	logf("api key created")

	got := fetchSecretAsAgent(baseURL, apiKey, projectID)
	if got != testSecretValue {
		fatalf("secret round-trip failed: got %q, want %q", got, testSecretValue)
	}
	logf("[OK] secret round-trip via scoped API key succeeded")

	fmt.Println("\n[OK] KeepSave <-> Seidr-style E2E passed.")
}

func waitReady(base string) {
	for i := 0; i < 60; i++ {
		resp, err := http.Get(base + "/readyz")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			logf("keepsave ready after %ds", i)
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	fatalf("keepsave /readyz did not return 200 within 60s")
}

func registerAndLogin(base string) string {
	// Ignore any error on register: if the user already exists (re-run),
	// login still succeeds below.
	_, _ = doJSON("POST", base+"/api/v1/auth/register", "", map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})

	var lr struct {
		Token string `json:"token"`
	}
	mustJSON("POST", base+"/api/v1/auth/login", "", map[string]string{
		"email":    testEmail,
		"password": testPassword,
	}, &lr)
	if lr.Token == "" {
		fatalf("empty JWT from /auth/login")
	}
	return lr.Token
}

func createProject(base, token string) string {
	var resp struct {
		ID string `json:"id"`
	}
	mustJSON("POST", base+"/api/v1/projects", "Bearer "+token, map[string]string{
		"name":        "seidr-e2e",
		"description": "KeepSave<->Seidr end-to-end harness project",
	}, &resp)
	if resp.ID == "" {
		fatalf("project create returned empty id")
	}
	return resp.ID
}

func storeSecret(base, token, projectID string) {
	mustJSON("POST", base+"/api/v1/projects/"+projectID+"/secrets", "Bearer "+token, map[string]string{
		"key":         testSecretKey,
		"value":       testSecretValue,
		"environment": "alpha",
	}, nil)
}

func createAPIKey(base, token, projectID string) string {
	var resp struct {
		RawKey string `json:"raw_key"`
		Key    string `json:"key"`
	}
	mustJSON("POST", base+"/api/v1/api-keys", "Bearer "+token, map[string]interface{}{
		"name":        "seidr-e2e-agent",
		"project_id":  projectID,
		"scopes":      []string{"read"},
		"environment": "alpha",
	}, &resp)
	if resp.RawKey != "" {
		return resp.RawKey
	}
	if resp.Key != "" {
		return resp.Key
	}
	fatalf("api-key response had neither raw_key nor key")
	return ""
}

func fetchSecretAsAgent(base, apiKey, projectID string) string {
	req, _ := http.NewRequest("GET", base+"/api/v1/projects/"+projectID+"/secrets?environment=alpha", nil)
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("fetch secret: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatalf("fetch secret: HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Response shape varies between wrappers. Try the wrapped form first,
	// then the bare array.
	var wrapped struct {
		Secrets []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"secrets"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Secrets) > 0 {
		for _, s := range wrapped.Secrets {
			if s.Key == testSecretKey {
				return s.Value
			}
		}
	}

	var bare []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &bare); err == nil {
		for _, s := range bare {
			if s.Key == testSecretKey {
				return s.Value
			}
		}
	}

	fatalf("secret %q not found in response: %s", testSecretKey, string(body))
	return ""
}

func doJSON(method, url, auth string, payload interface{}) ([]byte, int) {
	var body io.Reader
	if payload != nil {
		buf, _ := json.Marshal(payload)
		body = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return out, resp.StatusCode
}

func mustJSON(method, url, auth string, payload, out interface{}) {
	body, status := doJSON(method, url, auth, payload)
	if status < 200 || status >= 300 {
		fatalf("%s %s: HTTP %d: %s", method, url, status, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			fatalf("decoding %s: %v (body=%s)", url, err, string(body))
		}
	}
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[e2e] "+format+"\n", args...)
}

func fatalf(format string, args ...interface{}) {
	logf(format, args...)
	os.Exit(1)
}
