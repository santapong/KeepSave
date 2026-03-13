package service

import (
	"testing"
)

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "My Organization", "my-organization"},
		{"with special chars", "Test@Org#123", "test-org-123"},
		{"already lowercase", "my-org", "my-org"},
		{"leading/trailing spaces", "  Hello World  ", "hello-world"},
		{"empty string", "", "org"},
		{"unicode chars", "Org!@#$%", "org"},
		{"multiple spaces", "My   Great   Org", "my-great-org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role  string
		valid bool
	}{
		{"viewer", true},
		{"editor", true},
		{"admin", true},
		{"promoter", true},
		{"superadmin", false},
		{"", false},
		{"ADMIN", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := isValidRole(tt.role)
			if result != tt.valid {
				t.Errorf("isValidRole(%q) = %v, want %v", tt.role, result, tt.valid)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name         string
		userRole     string
		requiredRole string
		expected     bool
	}{
		{"admin can do admin", "admin", "admin", true},
		{"admin can do editor", "admin", "editor", true},
		{"admin can do viewer", "admin", "viewer", true},
		{"admin can do promoter", "admin", "promoter", true},
		{"editor can do viewer", "editor", "viewer", true},
		{"editor can do editor", "editor", "editor", true},
		{"editor cannot do admin", "editor", "admin", false},
		{"viewer cannot do editor", "viewer", "editor", false},
		{"viewer can do viewer", "viewer", "viewer", true},
		{"promoter can do editor", "promoter", "editor", true},
		{"promoter cannot do admin", "promoter", "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPermission(tt.userRole, tt.requiredRole)
			if result != tt.expected {
				t.Errorf("hasPermission(%q, %q) = %v, want %v", tt.userRole, tt.requiredRole, result, tt.expected)
			}
		})
	}
}
