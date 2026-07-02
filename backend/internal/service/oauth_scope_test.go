package service

import (
	"strings"
	"testing"
)

// TestGrantedScopes_NoEscalation is the negative-auth proof for the OAuth scope
// escalation fix (CWE-269/863, RFC 6749 §3.3). grantedScopes backs BOTH the
// authorization_code (Authorize) and client_credentials flows, so exercising it
// directly covers the escalation path for both: a client registered with
// ["read"] can never mint "write" or "admin".
func TestGrantedScopes_NoEscalation(t *testing.T) {
	tests := []struct {
		name       string
		requested  []string
		registered []string
		want       []string
		wantErr    bool
	}{
		{
			name:       "read-only client cannot escalate to write",
			requested:  []string{"write"},
			registered: []string{"read"},
			wantErr:    true,
		},
		{
			name:       "read-only client cannot escalate to admin",
			requested:  []string{"admin"},
			registered: []string{"read"},
			wantErr:    true,
		},
		{
			name:       "mixed request is narrowed to the registered subset",
			requested:  []string{"read", "write", "admin"},
			registered: []string{"read"},
			want:       []string{"read"},
		},
		{
			name:       "empty request defaults to the full registered set",
			requested:  nil,
			registered: []string{"read", "write"},
			want:       []string{"read", "write"},
		},
		{
			name:       "exact match is granted verbatim",
			requested:  []string{"read", "write"},
			registered: []string{"read", "write", "delete"},
			want:       []string{"read", "write"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := grantedScopes(tc.requested, tc.registered)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("grantedScopes(%v, %v) = %v, want error", tc.requested, tc.registered, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("grantedScopes(%v, %v) unexpected error: %v", tc.requested, tc.registered, err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("grantedScopes(%v, %v) = %v, want %v", tc.requested, tc.registered, got, tc.want)
			}
			// A granted scope must NEVER exceed the registered set.
			reg := map[string]struct{}{}
			for _, s := range tc.registered {
				reg[s] = struct{}{}
			}
			for _, s := range got {
				if _, ok := reg[s]; !ok {
					t.Errorf("granted scope %q was not registered %v — escalation", s, tc.registered)
				}
			}
		})
	}
}
