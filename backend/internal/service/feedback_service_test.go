package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// fakeGitHub stands up an httptest server that mimics the GitHub "create
// issue" endpoint. It records the last request (path, headers, decoded body)
// and replies with the configured status + body.
type fakeGitHub struct {
	server     *httptest.Server
	status     int
	respBody   string
	gotPath    string
	gotAuth    string
	gotAccept  string
	gotVersion string
	gotBody    map[string]interface{}
}

func newFakeGitHub(t *testing.T, status int, respBody string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{status: status, respBody: respBody}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotPath = r.URL.Path
		f.gotAuth = r.Header.Get("Authorization")
		f.gotAccept = r.Header.Get("Accept")
		f.gotVersion = r.Header.Get("X-GitHub-Api-Version")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &f.gotBody)
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.respBody)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// TestFeedbackService_Submit_HappyPath verifies the request shape sent to
// GitHub, the 201 parsing, and that an audit row is written without the
// message text.
func TestFeedbackService_Submit_HappyPath(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusCreated,
		`{"html_url":"https://github.com/santapong/KeepSave/issues/42","number":42}`)
	repo, db := newAuditTestRepo(t)
	svc := NewFeedbackService("secret-token", "santapong/KeepSave", repo)
	svc.SetAPIBase(fake.server.URL)

	actor := uuid.New()
	const message = "the deploy button is broken"
	url, number, err := svc.Submit(actor, "user@example.com", "bug", message,
		"https://app.example/home", "Mozilla/5.0", "10.0.0.1")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if url != "https://github.com/santapong/KeepSave/issues/42" {
		t.Errorf("url = %q", url)
	}
	if number != 42 {
		t.Errorf("number = %d, want 42", number)
	}

	// Request shape.
	if fake.gotPath != "/repos/santapong/KeepSave/issues" {
		t.Errorf("path = %q, want /repos/santapong/KeepSave/issues", fake.gotPath)
	}
	if fake.gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q", fake.gotAuth)
	}
	if fake.gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept = %q", fake.gotAccept)
	}
	if fake.gotVersion != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q", fake.gotVersion)
	}

	// Labels ["feedback","bug"].
	labelsRaw, _ := fake.gotBody["labels"].([]interface{})
	if len(labelsRaw) != 2 || labelsRaw[0] != "feedback" || labelsRaw[1] != "bug" {
		t.Errorf("labels = %v, want [feedback bug]", labelsRaw)
	}

	// Body carries the message and metadata.
	body, _ := fake.gotBody["body"].(string)
	if !strings.Contains(body, message) {
		t.Errorf("body missing message: %q", body)
	}
	if !strings.Contains(body, "user@example.com") || !strings.Contains(body, actor.String()) {
		t.Errorf("body missing identity metadata: %q", body)
	}

	// Audit row written, message text NOT present in details.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, "feedback.submitted").Scan(&count); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit rows = %d, want 1", count)
	}
	var details, userID string
	if err := db.QueryRow(`SELECT details, user_id FROM audit_log WHERE action = ?`, "feedback.submitted").Scan(&details, &userID); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if userID != actor.String() {
		t.Errorf("audit user_id = %q, want %q", userID, actor.String())
	}
	if strings.Contains(details, message) {
		t.Errorf("audit details leaked message text: %q", details)
	}
	if !strings.Contains(details, "bug") || !strings.Contains(details, "santapong/KeepSave") {
		t.Errorf("audit details missing category/repo: %q", details)
	}
}

// TestFeedbackService_Submit_FenceContainsBackticks verifies that a message
// containing a triple-backtick cannot escape the code fence: the fence is
// widened and the raw message is still fully contained.
func TestFeedbackService_Submit_FenceContainsBackticks(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusCreated, `{"html_url":"u","number":1}`)
	repo, _ := newAuditTestRepo(t)
	svc := NewFeedbackService("tok", "santapong/KeepSave", repo)
	svc.SetAPIBase(fake.server.URL)

	message := "here is code:\n```\nrm -rf /\n``` @maintainer"
	if _, _, err := svc.Submit(uuid.New(), "u@e.com", "idea", message, "", "UA", "1.2.3.4"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	body, _ := fake.gotBody["body"].(string)
	// A four-backtick fence must wrap the message so the inner ``` is inert.
	if !strings.Contains(body, "````") {
		t.Errorf("expected widened fence (```` ) in body: %q", body)
	}
	if !strings.Contains(body, message) {
		t.Errorf("body must contain the raw message verbatim: %q", body)
	}
}

// TestFeedbackService_Submit_MetaInjection verifies that a client-supplied
// page_url (and User-Agent) cannot inject markdown/@-mentions into the issue
// body: newlines are collapsed and the value is wrapped in an inline code span
// so it stays a single, inert metadata line.
func TestFeedbackService_Submit_MetaInjection(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusCreated, `{"html_url":"u","number":1}`)
	repo, _ := newAuditTestRepo(t)
	svc := NewFeedbackService("tok", "santapong/KeepSave", repo)
	svc.SetAPIBase(fake.server.URL)

	pageURL := "https://app/x\n\n@maintainer ping\n\n## injected"
	ua := "Mozilla/5.0\n@evil"
	if _, _, err := svc.Submit(uuid.New(), "u@e.com", "bug", "msg", pageURL, ua, "1.2.3.4"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	body, _ := fake.gotBody["body"].(string)

	// No injected line: the smuggled newlines must not survive to start a new
	// markdown line (@-mention, heading, spoofed metadata).
	for _, bad := range []string{"\n@maintainer", "\n## injected", "\n@evil"} {
		if strings.Contains(body, bad) {
			t.Errorf("body leaked injected line %q: %q", bad, body)
		}
	}
	// The metadata lines still carry the (now inert) values.
	if !strings.Contains(body, "- Page URL: ") || !strings.Contains(body, "- User-Agent: ") {
		t.Errorf("body missing metadata lines: %q", body)
	}
	// Each metadata line remains a single line (leading "- " marker per line).
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "@") || strings.HasPrefix(line, "#") {
			t.Errorf("body has an unfenced injected line: %q", line)
		}
	}
}

// TestFeedbackService_Submit_UpstreamError verifies GitHub non-201 responses
// surface as an error (and no issue is created).
func TestFeedbackService_Submit_UpstreamError(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		fake := newFakeGitHub(t, status, `{"message":"boom"}`)
		repo, _ := newAuditTestRepo(t)
		svc := NewFeedbackService("tok", "santapong/KeepSave", repo)
		svc.SetAPIBase(fake.server.URL)

		_, _, err := svc.Submit(uuid.New(), "u@e.com", "other", "msg", "", "UA", "1.2.3.4")
		if err == nil {
			t.Fatalf("status %d: expected error", status)
		}
	}
}

// TestFeedbackService_Submit_Disabled verifies an unconfigured service refuses
// to submit.
func TestFeedbackService_Submit_Disabled(t *testing.T) {
	repo, _ := newAuditTestRepo(t)
	svc := NewFeedbackService("", "santapong/KeepSave", repo)
	if svc.Enabled() {
		t.Fatal("Enabled() = true with empty token")
	}
	if _, _, err := svc.Submit(uuid.New(), "u@e.com", "bug", "msg", "", "UA", "1.2.3.4"); err == nil {
		t.Fatal("expected error when disabled")
	}
}

// TestFeedbackFence exercises the dynamic-fence helper directly.
func TestFeedbackFence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no backticks", "plain text", "```"},
		{"single backtick", "a `code` b", "```"},
		{"triple backtick", "```", "````"},
		{"quad backtick run", "````x````", "`````"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := feedbackFence(tc.in); got != tc.want {
				t.Errorf("feedbackFence(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFeedbackTitle verifies truncation + newline collapsing.
func TestFeedbackTitle(t *testing.T) {
	long := strings.Repeat("x", 100)
	title := feedbackTitle("bug", long)
	if !strings.HasPrefix(title, "[feedback] bug: ") {
		t.Errorf("prefix wrong: %q", title)
	}
	if !strings.HasSuffix(title, "…") {
		t.Errorf("expected truncation ellipsis: %q", title)
	}
	multi := feedbackTitle("idea", "line one\nline two")
	if strings.Contains(multi, "\n") {
		t.Errorf("newlines not collapsed: %q", multi)
	}
}
