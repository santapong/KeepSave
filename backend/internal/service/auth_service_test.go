package service

import (
	"strings"
	"testing"

	"github.com/santapong/KeepSave/backend/internal/auth"
)

// TestRegisterPasswordPolicy asserts that Register enforces the full
// DefaultPasswordPolicy (upper/lower/digit/special + min length), not just a
// length check. Weak passwords must be rejected before any DB work happens, so
// a service with nil repositories is sufficient to exercise the policy gate.
func TestRegisterPasswordPolicy(t *testing.T) {
	// Repos are nil: a rejected password must return before touching any of
	// them. If the policy check is skipped, these cases would panic instead.
	s := &AuthService{}

	weak := []struct {
		name     string
		password string
	}{
		{"too short", "Aa1!x"},
		{"no uppercase", "lowercase1!"},
		{"no lowercase", "UPPERCASE1!"},
		{"no digit", "NoDigits!!"},
		{"no special", "NoSpecial1"},
	}

	for _, tc := range weak {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := s.Register("user@example.com", tc.password)
			if err == nil {
				t.Fatalf("expected weak password %q to be rejected, got nil error", tc.password)
			}
			if resp != nil {
				t.Fatalf("expected nil response for rejected password, got %+v", resp)
			}
			// Never surface the plaintext password in the error.
			if strings.Contains(err.Error(), tc.password) {
				t.Fatalf("policy error leaked the plaintext password: %v", err)
			}
		})
	}

	// A compliant password must pass the policy gate. We assert against the
	// same validator Register uses so we don't need live DB/JWT deps to prove
	// the compliant case is accepted by the policy.
	if err := auth.DefaultPasswordPolicy().Validate("Str0ng!Pass"); err != nil {
		t.Fatalf("expected compliant password to pass policy, got %v", err)
	}
}
