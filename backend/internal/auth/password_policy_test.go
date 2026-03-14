package auth

import (
	"testing"
)

func TestPasswordPolicyValidate(t *testing.T) {
	policy := DefaultPasswordPolicy()

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "SecureP@ss1", false},
		{"too short", "Ab1!", false},
		{"no uppercase", "securepass1!", true},
		{"no lowercase", "SECUREPASS1!", true},
		{"no digit", "SecurePass!", true},
		{"no special", "SecurePass1", true},
		{"empty", "", true},
		{"just spaces", "        ", true},
		{"all requirements met", "MyP@ssw0rd!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.Validate(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr = %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestPasswordPolicyTooShort(t *testing.T) {
	policy := DefaultPasswordPolicy()
	err := policy.Validate("Ab1!")
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestPasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		minScore int
	}{
		{"abc", 0},
		{"abcdefgh", 1},
		{"Abcdefgh", 1},
		{"Abcdefgh1!", 3},
		{"MySecureP@ss1!", 4},
	}

	for _, tt := range tests {
		score := PasswordStrength(tt.password)
		if score < tt.minScore {
			t.Errorf("PasswordStrength(%q) = %d, want >= %d", tt.password, score, tt.minScore)
		}
	}
}
