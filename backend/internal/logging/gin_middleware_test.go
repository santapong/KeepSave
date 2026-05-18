package logging

import (
	"strings"
	"testing"
)

// TestRedactQuery covers audit S-M2: sensitive parameters MUST be
// replaced with REDACTED before the log line is built. Param NAMES stay
// visible so a reader can still tell "which query had a token".
func TestRedactQuery(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantHas  string // substring that MUST appear
		wantGone string // substring that MUST NOT appear
	}{
		{
			name:     "token redacted",
			raw:      "token=hunter2&page=1",
			wantHas:  "token=REDACTED",
			wantGone: "hunter2",
		},
		{
			name:     "email redacted",
			raw:      "email=victim%40example.com",
			wantHas:  "email=REDACTED",
			wantGone: "victim%40example.com",
		},
		{
			name:     "non-sensitive untouched",
			raw:      "limit=10&offset=20",
			wantHas:  "limit=10",
			wantGone: "REDACTED",
		},
		{
			name:    "empty",
			raw:     "",
			wantHas: "",
		},
		{
			name:     "mixed",
			raw:      "page=2&access_token=abc.def&order=desc",
			wantHas:  "access_token=REDACTED",
			wantGone: "abc.def",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactQuery(tc.raw)
			if tc.wantHas != "" && !strings.Contains(got, tc.wantHas) {
				t.Errorf("redactQuery(%q) = %q, want substring %q", tc.raw, got, tc.wantHas)
			}
			if tc.wantGone != "" && strings.Contains(got, tc.wantGone) {
				t.Errorf("redactQuery(%q) = %q, must not contain %q", tc.raw, got, tc.wantGone)
			}
		})
	}
}
