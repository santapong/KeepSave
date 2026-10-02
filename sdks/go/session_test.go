package keepsave

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCredentialSwitchAndProtectedDenialClearCache(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			fmt.Fprint(w, `{"user":{"id":"new"},"token":"new-session"}`)
		case "/api/v1/projects/p/secrets":
			count++
			fmt.Fprintf(w, `{"secrets":[{"key":"VALUE","value":"revision-%d"}]}`, count)
		default:
			w.WriteHeader(401)
			fmt.Fprint(w, "denied")
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, WithToken("old-session"), WithMaxRetries(0))
	ctx := context.Background()
	_, _ = client.ListSecrets(ctx, "p", "alpha")
	_, _ = client.ListSecrets(ctx, "p", "alpha")
	if count != 1 {
		t.Fatal("cache fixture failed")
	}
	if _, err := client.Login(ctx, "synthetic@example.invalid", "synthetic"); err != nil {
		t.Fatal(err)
	}
	_, _ = client.ListSecrets(ctx, "p", "alpha")
	if count != 2 {
		t.Fatal("account switch reused old data")
	}
	if _, err := client.ListProjects(ctx); err == nil {
		t.Fatal("denial was accepted")
	}
	if client.token != "" || len(client.cache.store) != 0 {
		t.Fatal("protected denial retained identity/cache")
	}
}

func TestHumanSessionHelpersAndConfirmedLogout(t *testing.T) {
	unavailable := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer human-session" || r.Header.Get("X-API-Key") != "" {
			t.Error("session endpoint used API key")
		}
		if r.URL.Path == "/api/v1/auth/logout" {
			if unavailable {
				w.WriteHeader(503)
				fmt.Fprint(w, `{"error":{"code":503,"message":"unavailable"}}`)
				return
			}
			w.WriteHeader(204)
			return
		}
		fmt.Fprint(w, `{"sessions":[{"id":"s","current":true,"status":"active"}]}`)
	}))
	defer server.Close()
	client := NewClient(server.URL, WithToken("human-session"), WithAPIKey("separate-api-key"), WithMaxRetries(0))
	ctx := context.Background()
	rows, err := client.ListSessions(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatal("session listing", err)
	}
	client.cache.set("fixture", []Secret{{Key: "synthetic"}})
	if err = client.Logout(ctx); err == nil {
		t.Fatal("failed logout claimed success")
	}
	_, cached := client.cache.get("fixture")
	if client.token != "human-session" || !cached {
		t.Fatal("failed logout cleared local session")
	}
	unavailable = false
	if err = client.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if client.token != "" || client.apiKey != "separate-api-key" || len(client.cache.store) != 0 {
		t.Fatal("logout credentials/cache incorrect")
	}
}
