package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// FeedbackService files in-app user feedback as GitHub issues. It is enabled
// only when a GitHub token is configured (FEEDBACK_GITHUB_TOKEN); otherwise
// the endpoint returns 503. User identity and User-Agent are supplied by the
// handler from the auth context / request headers, never from client JSON.
type FeedbackService struct {
	token     string
	repo      string // "owner/repo"
	apiBase   string // default https://api.github.com; tests point it at httptest
	client    *http.Client
	auditRepo *repository.AuditRepository
}

func NewFeedbackService(token, repo string, auditRepo *repository.AuditRepository) *FeedbackService {
	return &FeedbackService{
		token:     token,
		repo:      repo,
		apiBase:   "https://api.github.com",
		client:    &http.Client{Timeout: 10 * time.Second},
		auditRepo: auditRepo,
	}
}

// SetAPIBase overrides the GitHub API base URL. Used by tests to point the
// service at an httptest.Server; production code never calls it.
func (s *FeedbackService) SetAPIBase(base string) { s.apiBase = base }

// Enabled reports whether feedback submission is configured.
func (s *FeedbackService) Enabled() bool { return s.token != "" }

// Submit files the feedback as a GitHub issue and, on success, emits the
// feedback.submitted audit event. The returned error may contain the GitHub
// response body — callers MUST NOT forward it to clients (log-only detail).
func (s *FeedbackService) Submit(actorID uuid.UUID, actorEmail, category, message, pageURL, userAgent, ip string) (string, int, error) {
	if !s.Enabled() {
		return "", 0, fmt.Errorf("feedback: not configured (FEEDBACK_GITHUB_TOKEN empty)")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"title":  feedbackTitle(category, message),
		"body":   feedbackBody("KeepSave", category, actorID.String(), actorEmail, pageURL, userAgent, message),
		"labels": []string{"feedback", category},
	})
	if err != nil {
		return "", 0, fmt.Errorf("feedback: marshaling issue payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.apiBase+"/repos/"+s.repo+"/issues", bytes.NewReader(payload))
	if err != nil {
		return "", 0, fmt.Errorf("feedback: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("feedback: posting issue: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return "", 0, fmt.Errorf("feedback: github returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var issue struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return "", 0, fmt.Errorf("feedback: decoding github response: %w", err)
	}

	// The message text is deliberately NOT placed in the audit details.
	emitAudit(s.auditRepo, &actorID, nil, "feedback.submitted", "",
		models.JSONMap{"category": category, "issue_number": issue.Number, "repo": s.repo}, ip)
	return issue.HTMLURL, issue.Number, nil
}

// feedbackTitle builds "[feedback] <category>: <first 60 chars of message>",
// with newlines collapsed to spaces and "…" appended when truncated.
func feedbackTitle(category, message string) string {
	oneLine := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(message)
	if r := []rune(oneLine); len(r) > 60 {
		oneLine = string(r[:60]) + "…"
	}
	return "[feedback] " + category + ": " + oneLine
}

// feedbackFence returns a backtick code fence guaranteed to be longer than the
// longest backtick run inside message (minimum 3), so a message containing
// "```" cannot break out of the fence and inject markdown/@-mentions.
func feedbackFence(message string) string {
	longest, run := 0, 0
	for _, r := range message {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

// feedbackMeta renders an untrusted single-line metadata value (page URL,
// User-Agent) as an inline code span so it cannot inject markdown or
// @-mentions into the issue body. Newlines are first collapsed to spaces so
// the value cannot break out onto a new markdown line (spoofed metadata,
// headings, images), and the backtick delimiter is widened past any backtick
// run inside the value (see feedbackFence) so it cannot close the span early.
func feedbackMeta(v string) string {
	v = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(v)
	fence := feedbackFence(v)
	// Per the CommonMark code-span rule, pad with a space when the value
	// begins or ends with a backtick so the delimiters aren't absorbed.
	if strings.HasPrefix(v, "`") || strings.HasSuffix(v, "`") {
		return fence + " " + v + " " + fence
	}
	return fence + v + fence
}

// feedbackBody renders the issue body: metadata lines, then the raw message
// inside a dynamic code fence (see feedbackFence). Client-influenced metadata
// (page URL, User-Agent) is wrapped in an inline code span (see feedbackMeta)
// so it cannot inject markdown/@-mentions the way the fenced message cannot.
func feedbackBody(app, category, userID, email, pageURL, userAgent, message string) string {
	fence := feedbackFence(message)
	var b strings.Builder
	fmt.Fprintf(&b, "- App: %s\n", app)
	fmt.Fprintf(&b, "- Category: %s\n", category)
	fmt.Fprintf(&b, "- User: %s (%s)\n", userID, email)
	if pageURL != "" {
		fmt.Fprintf(&b, "- Page URL: %s\n", feedbackMeta(pageURL))
	}
	fmt.Fprintf(&b, "- User-Agent: %s\n", feedbackMeta(userAgent))
	fmt.Fprintf(&b, "- Submitted: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "%s\n%s\n%s\n", fence, message, fence)
	return b.String()
}
