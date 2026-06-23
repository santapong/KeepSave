package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretsToEnv(t *testing.T) {
	base := []string{"PATH=/bin", "FOO=old"}
	secrets := []interface{}{
		map[string]interface{}{"key": "FOO", "value": "new"},
		map[string]interface{}{"key": "BAR", "value": "baz"},
		map[string]interface{}{"key": "", "value": "skip-empty-key"},
		"not-a-map",
	}
	env := secretsToEnv(base, secrets)

	// base (2) + FOO + BAR; the empty-key entry and non-map are skipped.
	if len(env) != 4 {
		t.Fatalf("env len = %d, want 4: %v", len(env), env)
	}
	var sawFooNew, sawBar bool
	fooNewIdx, fooOldIdx := -1, -1
	for i, e := range env {
		switch {
		case e == "FOO=new":
			sawFooNew = true
			fooNewIdx = i
		case e == "FOO=old":
			fooOldIdx = i
		case e == "BAR=baz":
			sawBar = true
		case strings.HasPrefix(e, "=") || strings.Contains(e, "skip-empty-key"):
			t.Errorf("empty-key secret leaked into env: %q", e)
		}
	}
	if !sawFooNew || !sawBar {
		t.Errorf("missing FOO=new (%v) or BAR=baz (%v)", sawFooNew, sawBar)
	}
	// The secret overlay must come AFTER the inherited value so it shadows it.
	if fooNewIdx < fooOldIdx {
		t.Errorf("FOO=new (idx %d) must come after FOO=old (idx %d) to shadow it", fooNewIdx, fooOldIdx)
	}
}

func TestCmdRun_InjectsSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"secrets": []map[string]interface{}{{"key": "FOO", "value": "bar123"}},
		})
	}))
	defer srv.Close()

	c := &CLI{apiURL: srv.URL, client: &http.Client{}}
	out := filepath.Join(t.TempDir(), "out.txt")
	err := c.cmdRun([]string{"--project", "p1", "--env", "alpha", "--", "sh", "-c", "printf %s \"$FOO\" > " + out})
	if err != nil {
		t.Fatalf("cmdRun: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read child output: %v", err)
	}
	if string(got) != "bar123" {
		t.Errorf("child saw FOO=%q, want bar123", got)
	}
}

func TestCmdRun_RequiresCommand(t *testing.T) {
	c := &CLI{apiURL: "http://unused", client: &http.Client{}, projectID: "p1"}
	if err := c.cmdRun([]string{"--env", "alpha"}); err == nil {
		t.Error("cmdRun without a -- command should error")
	}
}
